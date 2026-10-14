package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"classlab/api/internal/platform/clock"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const jwtIssuer = "classlab"

var ErrInvalidToken = errors.New("invalid access token")

// TokenIssuer signs and verifies short-lived access tokens (HS256).
// The token carries only the subject: role and active state are read from the database per request,
// so a role change or deactivation takes effect immediately instead of after the token expires.
type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
	clock  clock.Clock
}

func NewTokenIssuer(secret string, ttl time.Duration, clk clock.Clock) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), ttl: ttl, clock: clk}
}

func (t *TokenIssuer) TTL() time.Duration { return t.ttl }

func (t *TokenIssuer) Issue(userID uuid.UUID) (string, error) {
	now := t.clock.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    jwtIssuer,
		Subject:   userID.String(),
		ID:        uuid.NewString(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", fmt.Errorf("signing access token: %w", err)
	}
	return signed, nil
}

// Verify returns the user id of a valid, unexpired token.
func (t *TokenIssuer) Verify(token string) (uuid.UUID, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(jwtIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithTimeFunc(t.clock.Now),
	)

	claims := &jwt.RegisteredClaims{}
	parsed, err := parser.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return t.secret, nil })
	if err != nil || !parsed.Valid {
		return uuid.Nil, ErrInvalidToken
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return id, nil
}

// newRefreshToken returns a random opaque token and the hash that is stored in its place.
func newRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generating refresh token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashRefreshToken(raw), nil
}

// A plain SHA-256 is enough here: the input is 256 random bits, so there is nothing to brute-force.
func hashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
