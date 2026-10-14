package users

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/platform/password"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	minPasswordRunes = 8
	maxPasswordRunes = 128

	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`)

type Service struct {
	pool   *db.Pool
	q      *dbgen.Queries
	hasher *password.Hasher
	clock  clock.Clock
}

func NewService(pool *db.Pool, hasher *password.Hasher, clk clock.Clock) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), hasher: hasher, clock: clk}
}

type ListFilter struct {
	Role     *httpx.Role
	IsActive *bool
	Query    string
	Page     int
	PerPage  int
}

type CreateInput struct {
	OrgUnitID  uuid.UUID
	Identifier string
	Name       string
	Email      string
	Password   string
	Role       httpx.Role
}

// UpdateInput holds only the fields the caller sent; nil means "leave unchanged".
type UpdateInput struct {
	Name     *string
	Email    *string
	Role     *httpx.Role
	IsActive *bool
	Password *string
}

func (in UpdateInput) empty() bool {
	return in.Name == nil && in.Email == nil && in.Role == nil && in.IsActive == nil && in.Password == nil
}

// Get returns the account for id. Used for /auth/me, so it is not restricted to Kaprodi.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := s.q.GetUserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, httpx.ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("getting user: %w", err)
	}
	return userRow(row).toUser(), nil
}

func (s *Service) List(ctx context.Context, actor httpx.Principal, f ListFilter) ([]User, int64, error) {
	if !actor.HasRole(httpx.RoleKaprodi) {
		return nil, 0, httpx.ErrForbidden
	}

	var role dbgen.NullUserRole
	if f.Role != nil {
		role = dbgen.NullUserRole{UserRole: dbgen.UserRole(*f.Role), Valid: true}
	}
	var active pgtype.Bool
	if f.IsActive != nil {
		active = pgtype.Bool{Bool: *f.IsActive, Valid: true}
	}
	var q pgtype.Text
	if term := strings.TrimSpace(f.Query); term != "" {
		q = pgtype.Text{String: escapeLike(term), Valid: true}
	}

	total, err := s.q.CountUsers(ctx, dbgen.CountUsersParams{Role: role, IsActive: active, Q: q})
	if err != nil {
		return nil, 0, fmt.Errorf("counting users: %w", err)
	}
	rows, err := s.q.ListUsers(ctx, dbgen.ListUsersParams{
		Role: role, IsActive: active, Q: q,
		PageLimit:  int32(f.PerPage),
		PageOffset: int32((f.Page - 1) * f.PerPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing users: %w", err)
	}

	out := make([]User, 0, len(rows))
	for _, r := range rows {
		out = append(out, userRow(r).toUser())
	}
	return out, total, nil
}

func (s *Service) Create(ctx context.Context, actor httpx.Principal, in CreateInput) (User, error) {
	if !actor.HasRole(httpx.RoleKaprodi) {
		return User{}, httpx.ErrForbidden
	}

	in.Identifier = strings.TrimSpace(in.Identifier)
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(in.Email)

	var problems []httpx.ErrorDetail
	if in.OrgUnitID == uuid.Nil {
		problems = append(problems, httpx.ErrorDetail{Field: "org_unit_id", Message: "org_unit_id wajib diisi."})
	}
	problems = append(problems, validateIdentifier(in.Identifier)...)
	problems = append(problems, validateName(in.Name)...)
	problems = append(problems, validateEmail(in.Email)...)
	problems = append(problems, validatePassword(in.Password)...)
	if !in.Role.Valid() {
		problems = append(problems, httpx.ErrorDetail{Field: "role", Message: "Peran tidak dikenal."})
	}
	if len(problems) > 0 {
		return User{}, httpx.ValidationError(problems...)
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return User{}, fmt.Errorf("hashing password: %w", err)
	}

	row, err := s.q.CreateUser(ctx, dbgen.CreateUserParams{
		OrgUnitID:    in.OrgUnitID,
		Identifier:   in.Identifier,
		Name:         in.Name,
		Email:        in.Email,
		PasswordHash: hash,
		Role:         dbgen.UserRole(in.Role),
		CreatedAt:    s.clock.Now(),
	})
	if err != nil {
		if d, ok := constraintDetail(err); ok {
			return User{}, httpx.ValidationError(d)
		}
		return User{}, fmt.Errorf("creating user: %w", err)
	}
	return userRow(row).toUser(), nil
}

func (s *Service) Update(ctx context.Context, actor httpx.Principal, id uuid.UUID, in UpdateInput) (User, error) {
	if !actor.HasRole(httpx.RoleKaprodi) {
		return User{}, httpx.ErrForbidden
	}
	if in.empty() {
		return User{}, httpx.ValidationError(httpx.ErrorDetail{Message: "Tidak ada field yang diubah."})
	}

	var problems []httpx.ErrorDetail
	params := dbgen.UpdateUserParams{ID: id, UpdatedAt: s.clock.Now()}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		problems = append(problems, validateName(name)...)
		params.Name = pgtype.Text{String: name, Valid: true}
	}
	if in.Email != nil {
		email := strings.TrimSpace(*in.Email)
		problems = append(problems, validateEmail(email)...)
		params.Email = pgtype.Text{String: email, Valid: true}
	}
	if in.Role != nil {
		if !in.Role.Valid() {
			problems = append(problems, httpx.ErrorDetail{Field: "role", Message: "Peran tidak dikenal."})
		}
		params.Role = dbgen.NullUserRole{UserRole: dbgen.UserRole(*in.Role), Valid: true}
	}
	if in.IsActive != nil {
		params.IsActive = pgtype.Bool{Bool: *in.IsActive, Valid: true}
	}
	if in.Password != nil {
		problems = append(problems, validatePassword(*in.Password)...)
	}

	// A Kaprodi who deactivates or demotes themselves could leave nobody able to manage accounts.
	if id == actor.UserID {
		if in.IsActive != nil && !*in.IsActive {
			problems = append(problems, httpx.ErrorDetail{Field: "is_active", Message: "Anda tidak dapat menonaktifkan akun sendiri."})
		}
		if in.Role != nil && *in.Role != httpx.RoleKaprodi {
			problems = append(problems, httpx.ErrorDetail{Field: "role", Message: "Anda tidak dapat mengubah peran akun sendiri."})
		}
	}
	if len(problems) > 0 {
		return User{}, httpx.ValidationError(problems...)
	}

	if in.Password != nil {
		hash, err := s.hasher.Hash(*in.Password)
		if err != nil {
			return User{}, fmt.Errorf("hashing password: %w", err)
		}
		params.PasswordHash = pgtype.Text{String: hash, Valid: true}
	}

	var updated User
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		row, err := q.UpdateUser(ctx, params)
		if err != nil {
			return err
		}
		updated = userRow(row).toUser()

		// Deactivation and password reset must end existing sessions, not just block new logins.
		deactivated := in.IsActive != nil && !*in.IsActive
		if deactivated || in.Password != nil {
			if err := q.RevokeUserRefreshTokens(ctx, dbgen.RevokeUserRefreshTokensParams{
				UserID: id, RevokedAt: pgtype.Timestamptz{Time: params.UpdatedAt, Valid: true},
			}); err != nil {
				return fmt.Errorf("revoking refresh tokens: %w", err)
			}
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, httpx.ErrNotFound
	}
	if err != nil {
		if d, ok := constraintDetail(err); ok {
			return User{}, httpx.ValidationError(d)
		}
		return User{}, fmt.Errorf("updating user: %w", err)
	}
	return updated, nil
}

func validateIdentifier(v string) []httpx.ErrorDetail {
	if !identifierPattern.MatchString(v) {
		return []httpx.ErrorDetail{{Field: "identifier", Message: "Identifier harus 3-64 karakter (huruf, angka, titik, garis)."}}
	}
	return nil
}

func validateName(v string) []httpx.ErrorDetail {
	if n := utf8.RuneCountInString(v); n < 1 || n > 120 {
		return []httpx.ErrorDetail{{Field: "name", Message: "Nama wajib diisi (maksimal 120 karakter)."}}
	}
	return nil
}

func validateEmail(v string) []httpx.ErrorDetail {
	addr, err := mail.ParseAddress(v)
	// ParseAddress also accepts "Name <a@b>"; only the bare address is allowed here.
	if err != nil || addr.Address != v || len(v) > 254 {
		return []httpx.ErrorDetail{{Field: "email", Message: "Format email tidak valid."}}
	}
	return nil
}

func validatePassword(v string) []httpx.ErrorDetail {
	if n := utf8.RuneCountInString(v); n < minPasswordRunes || n > maxPasswordRunes {
		return []httpx.ErrorDetail{{Field: "password", Message: fmt.Sprintf("Password harus %d-%d karakter.", minPasswordRunes, maxPasswordRunes)}}
	}
	return nil
}

// constraintDetail maps unique/foreign-key violations to a field error instead of a 500.
func constraintDetail(err error) (httpx.ErrorDetail, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return httpx.ErrorDetail{}, false
	}
	switch {
	case pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "users_identifier_key":
		return httpx.ErrorDetail{Field: "identifier", Message: "Identifier sudah digunakan."}, true
	case pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "users_email_key":
		return httpx.ErrorDetail{Field: "email", Message: "Email sudah digunakan."}, true
	case pgErr.Code == pgForeignKeyViolation && pgErr.ConstraintName == "users_org_unit_id_fkey":
		return httpx.ErrorDetail{Field: "org_unit_id", Message: "Unit organisasi tidak ditemukan."}, true
	}
	return httpx.ErrorDetail{}, false
}

// escapeLike neutralises LIKE wildcards so user input is matched literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
