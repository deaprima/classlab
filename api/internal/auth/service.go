package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/platform/password"
	"classlab/api/internal/users"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// errBadCredentials is deliberately identical for "no such user", "wrong password" and "inactive account".
var errBadCredentials = httpx.NewCustomError(http.StatusUnauthorized, "UNAUTHENTICATED",
	"Identifier atau password salah.", nil, httpx.ErrUnauthenticated)

var errSessionExpired = httpx.NewCustomError(http.StatusUnauthorized, "UNAUTHENTICATED",
	"Sesi tidak valid atau telah berakhir. Silakan login kembali.", nil, httpx.ErrUnauthenticated)

type Service struct {
	pool       *db.Pool
	q          *dbgen.Queries
	users      *users.Service
	hasher     *password.Hasher
	tokens     *TokenIssuer
	limiter    *LoginLimiter
	clock      clock.Clock
	refreshTTL time.Duration
	log        *slog.Logger
}

func NewService(pool *db.Pool, usersSvc *users.Service, hasher *password.Hasher, tokens *TokenIssuer,
	limiter *LoginLimiter, clk clock.Clock, refreshTTL time.Duration, log *slog.Logger) *Service {
	return &Service{
		pool: pool, q: dbgen.New(pool), users: usersSvc, hasher: hasher, tokens: tokens,
		limiter: limiter, clock: clk, refreshTTL: refreshTTL, log: log,
	}
}

// Session is what a successful login or refresh hands back to the HTTP layer.
type Session struct {
	AccessToken      string
	ExpiresIn        int
	RefreshToken     string
	RefreshExpiresAt time.Time
	User             *users.User // only set on login
}

type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string { return "login rate limited" }
func (e *RateLimitedError) Unwrap() error { return httpx.ErrRateLimited }

// Authenticate implements httpx.Authenticator. It checks the database on every call so that
// deactivated accounts and role changes apply immediately.
func (s *Service) Authenticate(ctx context.Context, bearer string) (httpx.Principal, error) {
	id, err := s.tokens.Verify(bearer)
	if err != nil {
		return httpx.Principal{}, httpx.ErrUnauthenticated
	}
	u, err := s.q.GetUserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Principal{}, httpx.ErrUnauthenticated
	}
	if err != nil {
		return httpx.Principal{}, fmt.Errorf("loading user for token: %w", err)
	}
	if !u.IsActive {
		return httpx.Principal{}, httpx.ErrUnauthenticated
	}
	return httpx.Principal{UserID: u.ID, Role: httpx.Role(u.Role), MustChangePassword: u.MustChangePassword}, nil
}

// Login verifies credentials. clientIP only feeds the rate-limit key.
func (s *Service) Login(ctx context.Context, identifier, plainPassword, clientIP string) (*Session, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || plainPassword == "" {
		var problems []httpx.ErrorDetail
		if identifier == "" {
			problems = append(problems, httpx.ErrorDetail{Field: "identifier", Message: "Identifier wajib diisi."})
		}
		if plainPassword == "" {
			problems = append(problems, httpx.ErrorDetail{Field: "password", Message: "Password wajib diisi."})
		}
		return nil, httpx.ValidationError(problems...)
	}

	limitKey := strings.ToLower(identifier) + "|" + clientIP
	if retryAfter, ok := s.limiter.Allow(limitKey); !ok {
		s.log.WarnContext(ctx, "login rate limited", slog.String("identifier_ref", identifierRef(identifier)))
		return nil, &RateLimitedError{RetryAfter: retryAfter}
	}

	u, err := s.q.GetUserForLogin(ctx, identifier)
	if errors.Is(err, pgx.ErrNoRows) {
		s.hasher.BurnVerify(plainPassword)
		s.log.InfoContext(ctx, "login failed", slog.String("identifier_ref", identifierRef(identifier)), slog.String("reason", "unknown_user"))
		return nil, errBadCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("loading user for login: %w", err)
	}

	match, err := s.hasher.Verify(plainPassword, u.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("verifying password for user %s: %w", u.ID, err)
	}
	if !match || !u.IsActive {
		reason := "bad_password"
		if match {
			reason = "inactive"
		}
		s.log.InfoContext(ctx, "login failed", slog.String("identifier_ref", identifierRef(identifier)), slog.String("reason", reason))
		return nil, errBadCredentials
	}

	s.limiter.Reset(limitKey)

	raw, hash, err := newRefreshToken()
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	expiresAt := now.Add(s.refreshTTL)
	if err := s.q.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
		UserID: u.ID, FamilyID: uuid.New(), TokenHash: hash, ExpiresAt: expiresAt, CreatedAt: now,
	}); err != nil {
		return nil, fmt.Errorf("storing refresh token: %w", err)
	}

	access, err := s.tokens.Issue(u.ID)
	if err != nil {
		return nil, err
	}

	user := users.FromLoginRow(u)
	s.log.InfoContext(ctx, "login succeeded", slog.String("user_id", u.ID.String()))
	return &Session{
		AccessToken:      access,
		ExpiresIn:        int(s.tokens.TTL().Seconds()),
		RefreshToken:     raw,
		RefreshExpiresAt: expiresAt,
		User:             &user,
	}, nil
}

// Refresh rotates the refresh token. Presenting a token that was already rotated or revoked means
// it may have been stolen, so the whole family (every token descended from one login) is revoked.
func (s *Service) Refresh(ctx context.Context, rawToken string) (*Session, error) {
	if rawToken == "" {
		return nil, errSessionExpired
	}

	newRaw, newHash, err := newRefreshToken()
	if err != nil {
		return nil, err
	}

	var (
		userID    uuid.UUID
		expiresAt time.Time
		rejected  string // non-empty when the token must be refused but the revocation must still commit
	)
	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()
		revokedAt := pgtype.Timestamptz{Time: now, Valid: true}

		tok, err := q.GetRefreshTokenForUpdate(ctx, hashRefreshToken(rawToken))
		if errors.Is(err, pgx.ErrNoRows) {
			rejected = "unknown_token"
			return nil
		}
		if err != nil {
			return fmt.Errorf("loading refresh token: %w", err)
		}
		userID = tok.UserID

		// Returning nil (not an error) keeps the revocation: an error would roll the transaction back.
		revokeFamily := func(reason string) error {
			rejected = reason
			return q.RevokeRefreshTokenFamily(ctx, dbgen.RevokeRefreshTokenFamilyParams{FamilyID: tok.FamilyID, RevokedAt: revokedAt})
		}

		if tok.RevokedAt.Valid {
			return revokeFamily("reuse_detected")
		}
		if !now.Before(tok.ExpiresAt) {
			return revokeFamily("expired")
		}
		u, err := q.GetUserByID(ctx, tok.UserID)
		if err != nil {
			return fmt.Errorf("loading refresh token owner: %w", err)
		}
		if !u.IsActive {
			return revokeFamily("inactive_user")
		}

		if err := q.RevokeRefreshToken(ctx, dbgen.RevokeRefreshTokenParams{ID: tok.ID, RevokedAt: revokedAt}); err != nil {
			return fmt.Errorf("revoking rotated token: %w", err)
		}
		expiresAt = now.Add(s.refreshTTL)
		if err := q.CreateRefreshToken(ctx, dbgen.CreateRefreshTokenParams{
			UserID: tok.UserID, FamilyID: tok.FamilyID, TokenHash: newHash, ExpiresAt: expiresAt, CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("storing rotated token: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("rotating refresh token: %w", err)
	}
	if rejected != "" {
		level := slog.LevelInfo
		if rejected == "reuse_detected" {
			level = slog.LevelWarn
		}
		s.log.Log(ctx, level, "refresh rejected", slog.String("reason", rejected), slog.String("user_id", userID.String()))
		return nil, errSessionExpired
	}

	access, err := s.tokens.Issue(userID)
	if err != nil {
		return nil, err
	}
	return &Session{
		AccessToken:      access,
		ExpiresIn:        int(s.tokens.TTL().Seconds()),
		RefreshToken:     newRaw,
		RefreshExpiresAt: expiresAt,
	}, nil
}

// Logout revokes the whole family of the presented refresh token, if it belongs to the caller.
func (s *Service) Logout(ctx context.Context, actor httpx.Principal, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	err := s.q.RevokeFamilyOfTokenForUser(ctx, dbgen.RevokeFamilyOfTokenForUserParams{
		RevokedAt: pgtype.Timestamptz{Time: s.clock.Now(), Valid: true},
		TokenHash: hashRefreshToken(rawToken),
		UserID:    actor.UserID,
	})
	if err != nil {
		return fmt.Errorf("revoking refresh token family: %w", err)
	}
	return nil
}

func (s *Service) Me(ctx context.Context, actor httpx.Principal) (users.User, error) {
	return s.users.Profile(ctx, actor.UserID)
}

// identifierRef is a short, stable stand-in for an identifier in logs. The raw value is not logged:
// people mistype their password into the identifier field, so it can hold a credential. The
// reference still lets an operator see that several failures came from the same account.
func identifierRef(identifier string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(identifier)))
	return hex.EncodeToString(sum[:4])
}

// ChangePassword replaces the caller's password after checking the current one (BR-24). Every other
// session ends; the one that made the change, identified by its refresh token, stays. A temporary
// password stops being temporary here: must_change_password is cleared.
func (s *Service) ChangePassword(ctx context.Context, actor httpx.Principal, rawRefreshToken, current, next string) error {
	var problems []httpx.ErrorDetail
	if current == "" {
		problems = append(problems, httpx.ErrorDetail{Field: "current_password", Message: "Password saat ini wajib diisi."})
	}
	problems = append(problems, s.users.ValidatePassword(next)...)
	if current != "" && current == next {
		problems = append(problems, httpx.ErrorDetail{Field: "new_password", Message: "Password baru harus berbeda dari password saat ini."})
	}
	// Field names in the details follow the request body, not the service's internal names.
	for i := range problems {
		if problems[i].Field == "password" {
			problems[i].Field = "new_password"
		}
	}
	if len(problems) > 0 {
		return httpx.ValidationError(problems...)
	}

	// Someone holding a stolen access token could otherwise guess the current password at full speed.
	limitKey := "change-password|" + actor.UserID.String()
	if retryAfter, ok := s.limiter.Allow(limitKey); !ok {
		return &RateLimitedError{RetryAfter: retryAfter}
	}

	stored, err := s.q.GetPasswordHash(ctx, actor.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.ErrUnauthenticated
	}
	if err != nil {
		return fmt.Errorf("loading password hash: %w", err)
	}
	match, err := s.hasher.Verify(current, stored)
	if err != nil {
		return fmt.Errorf("verifying current password for user %s: %w", actor.UserID, err)
	}
	if !match {
		// 422, not 401: the session is fine, and the client treats 401 as "log in again".
		return httpx.ValidationError(httpx.ErrorDetail{Field: "current_password", Message: "Password saat ini salah."})
	}
	s.limiter.Reset(limitKey)

	hash, err := s.hasher.Hash(next)
	if err != nil {
		return fmt.Errorf("hashing new password: %w", err)
	}
	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()
		if err := q.SetUserPassword(ctx, dbgen.SetUserPasswordParams{
			ID: actor.UserID, PasswordHash: hash, MustChangePassword: false, UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("storing new password: %w", err)
		}
		return q.RevokeUserTokensExceptFamilyOf(ctx, dbgen.RevokeUserTokensExceptFamilyOfParams{
			RevokedAt: pgtype.Timestamptz{Time: now, Valid: true},
			UserID:    actor.UserID,
			TokenHash: hashRefreshToken(rawRefreshToken),
		})
	})
	if err != nil {
		return fmt.Errorf("changing password: %w", err)
	}
	s.log.InfoContext(ctx, "password changed", slog.String("user_id", actor.UserID.String()))
	return nil
}
