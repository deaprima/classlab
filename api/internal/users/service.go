package users

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"classlab/api/internal/audit"
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
	maxPasswordRunes = 128

	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgCheckViolation      = "23514"
)

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`)
	phonePattern      = regexp.MustCompile(`^[0-9+()\- ]{6,20}$`)
)

// Options are the settings the service reads from configuration.
type Options struct {
	MinPasswordLength  int
	TempPasswordLength int
	ViolationWindow    time.Duration // how far back violation points still count (BR-14)
}

type Service struct {
	pool   *db.Pool
	q      *dbgen.Queries
	hasher *password.Hasher
	clock  clock.Clock
	opts   Options
}

func NewService(pool *db.Pool, hasher *password.Hasher, clk clock.Clock, opts Options) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), hasher: hasher, clock: clk, opts: opts}
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
	Email      string // optional
	Password   string
	Role       httpx.Role
	PicName    string // required for ORGANISASI, refused for other roles
	PicPhone   string
}

// UpdateInput holds only the fields the caller sent; nil means "leave unchanged".
type UpdateInput struct {
	Name              *string
	Email             *string
	Role              *httpx.Role
	IsActive          *bool
	PicName           *string
	PicPhone          *string
	ClearBookingBlock bool
}

func (in UpdateInput) empty() bool {
	return in.Name == nil && in.Email == nil && in.Role == nil && in.IsActive == nil &&
		in.PicName == nil && in.PicPhone == nil && !in.ClearBookingBlock
}

// onlyAdminFields reports whether the input touches anything TU may not change (BR-24).
func (in UpdateInput) onlyAdminFields() bool {
	return in.Role != nil || in.PicName != nil || in.PicPhone != nil || in.ClearBookingBlock
}

// mayManage reports whether actor may manage an account holding targetRole: administrators any
// account, TU only students (BR-24).
func mayManage(actor httpx.Principal, targetRole httpx.Role) bool {
	switch actor.Role {
	case httpx.RoleAdmin:
		return true
	case httpx.RoleTU:
		return targetRole == httpx.RoleMahasiswa
	}
	return false
}

// Get returns the account for id. Used for /auth/me, so it is not restricted by role.
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

// Profile is Get plus the name of the account's org unit and, for a user who can book, the active
// violation points (BR-14).
func (s *Service) Profile(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := s.Get(ctx, id)
	if err != nil {
		return User{}, err
	}
	unit, err := s.q.GetOrgUnitNameByID(ctx, u.OrgUnitID)
	if err != nil {
		return User{}, fmt.Errorf("getting org unit: %w", err)
	}
	u.OrgUnitName = &unit
	if httpx.Role(u.Role) == httpx.RoleMahasiswa || httpx.Role(u.Role) == httpx.RoleOrganisasi {
		points, err := s.ActivePoints(ctx, id)
		if err != nil {
			return User{}, err
		}
		u.ActivePoints = &points
	}
	return u, nil
}

// ActivePoints sums the user's violation points that are not annulled and still inside the window.
func (s *Service) ActivePoints(ctx context.Context, id uuid.UUID) (int, error) {
	since := s.clock.Now().Add(-s.opts.ViolationWindow)
	n, err := s.q.SumActiveViolationPoints(ctx, dbgen.SumActiveViolationPointsParams{UserID: id, CreatedAt: since})
	if err != nil {
		return 0, fmt.Errorf("summing violation points: %w", err)
	}
	return int(n), nil
}

func (s *Service) List(ctx context.Context, actor httpx.Principal, f ListFilter) ([]User, int64, error) {
	switch actor.Role {
	case httpx.RoleAdmin:
	case httpx.RoleTU:
		// TU sees students only; asking for another role is an error, not an empty page.
		if f.Role != nil && *f.Role != httpx.RoleMahasiswa {
			return nil, 0, httpx.ErrForbidden
		}
		student := httpx.RoleMahasiswa
		f.Role = &student
	default:
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

// Create makes an account. Only administrators do; the password they choose is temporary, so the
// owner must change it at first login (BR-24).
func (s *Service) Create(ctx context.Context, actor httpx.Principal, in CreateInput) (User, error) {
	if actor.Role != httpx.RoleAdmin {
		return User{}, httpx.ErrForbidden
	}

	in.Identifier = strings.TrimSpace(in.Identifier)
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(in.Email)
	in.PicName = strings.TrimSpace(in.PicName)
	in.PicPhone = strings.TrimSpace(in.PicPhone)

	var problems []httpx.ErrorDetail
	if in.OrgUnitID == uuid.Nil {
		problems = append(problems, httpx.ErrorDetail{Field: "org_unit_id", Message: "org_unit_id wajib diisi."})
	}
	problems = append(problems, validateIdentifier(in.Identifier)...)
	problems = append(problems, validateName(in.Name)...)
	if in.Email != "" {
		problems = append(problems, validateEmail(in.Email)...)
	}
	problems = append(problems, s.validatePassword(in.Password)...)
	if !in.Role.Valid() {
		problems = append(problems, httpx.ErrorDetail{Field: "role", Message: "Peran tidak dikenal."})
	}
	problems = append(problems, validatePIC(in.Role, in.PicName, in.PicPhone)...)
	if len(problems) > 0 {
		return User{}, httpx.ValidationError(problems...)
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return User{}, fmt.Errorf("hashing password: %w", err)
	}

	var created User
	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		now := s.clock.Now()
		row, err := s.q.WithTx(tx).CreateUser(ctx, dbgen.CreateUserParams{
			OrgUnitID:          in.OrgUnitID,
			Identifier:         in.Identifier,
			Name:               in.Name,
			Email:              optionalText(in.Email),
			PasswordHash:       hash,
			Role:               dbgen.UserRole(in.Role),
			PicName:            optionalText(in.PicName),
			PicPhone:           optionalText(in.PicPhone),
			MustChangePassword: true,
			CreatedAt:          now,
		})
		if err != nil {
			return err
		}
		created = userRow(row).toUser()
		return audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionUserCreated, EntityType: audit.EntityUser,
			EntityID: created.ID, After: created, At: now,
		})
	})
	if err != nil {
		if d, ok := constraintDetail(err); ok {
			return User{}, httpx.ValidationError(d)
		}
		return User{}, fmt.Errorf("creating user: %w", err)
	}
	return created, nil
}

func (s *Service) Update(ctx context.Context, actor httpx.Principal, id uuid.UUID, in UpdateInput) (User, error) {
	if actor.Role != httpx.RoleAdmin && actor.Role != httpx.RoleTU {
		return User{}, httpx.ErrForbidden
	}
	// TU may change name, email and activation of students only; the role, contact person and block
	// date stay with administrators.
	if actor.Role == httpx.RoleTU && in.onlyAdminFields() {
		return User{}, httpx.ErrForbidden
	}
	if in.empty() {
		return User{}, httpx.ValidationError(httpx.ErrorDetail{Message: "Tidak ada field yang diubah."})
	}

	var problems []httpx.ErrorDetail
	params := dbgen.UpdateUserParams{ID: id, ClearBookingBlock: in.ClearBookingBlock}

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
	if in.PicName != nil {
		name := strings.TrimSpace(*in.PicName)
		if n := utf8.RuneCountInString(name); n < 1 || n > 120 {
			problems = append(problems, httpx.ErrorDetail{Field: "pic_name", Message: "Nama penanggung jawab wajib diisi (maksimal 120 karakter)."})
		}
		params.PicName = pgtype.Text{String: name, Valid: true}
	}
	if in.PicPhone != nil {
		phone := strings.TrimSpace(*in.PicPhone)
		if !phonePattern.MatchString(phone) {
			problems = append(problems, httpx.ErrorDetail{Field: "pic_phone", Message: "Nomor telepon penanggung jawab tidak valid."})
		}
		params.PicPhone = pgtype.Text{String: phone, Valid: true}
	}

	// An administrator who deactivates or demotes themselves could leave nobody able to manage accounts.
	if id == actor.UserID {
		if in.IsActive != nil && !*in.IsActive {
			problems = append(problems, httpx.ErrorDetail{Field: "is_active", Message: "Anda tidak dapat menonaktifkan akun sendiri."})
		}
		if in.Role != nil && *in.Role != actor.Role {
			problems = append(problems, httpx.ErrorDetail{Field: "role", Message: "Anda tidak dapat mengubah peran akun sendiri."})
		}
	}
	if len(problems) > 0 {
		return User{}, httpx.ValidationError(problems...)
	}

	var updated User
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()
		params.UpdatedAt = now

		// Changes that can remove an administrator are serialised and checked afterwards. The
		// self-protection rule above is not enough: two administrators deactivating each other at the
		// same moment would each pass it.
		mayRemoveAdmin := (in.IsActive != nil && !*in.IsActive) || (in.Role != nil && *in.Role != httpx.RoleAdmin)
		if mayRemoveAdmin {
			if err := q.LockAdminGuard(ctx); err != nil {
				return fmt.Errorf("locking admin guard: %w", err)
			}
		}

		// The row lock also makes the before-state in the audit log the state the update really started from.
		beforeRow, err := q.LockUser(ctx, id)
		if err != nil {
			return err
		}
		before := userRow(beforeRow).toUser()
		if !mayManage(actor, httpx.Role(before.Role)) {
			return httpx.ErrForbidden
		}

		row, err := q.UpdateUser(ctx, params)
		if err != nil {
			return err
		}
		updated = userRow(row).toUser()

		if mayRemoveAdmin {
			left, err := q.CountActiveAdmins(ctx)
			if err != nil {
				return fmt.Errorf("counting active admins: %w", err)
			}
			if left == 0 {
				return httpx.ValidationError(httpx.ErrorDetail{Message: "Harus tersisa minimal satu Admin aktif."})
			}
		}

		// Deactivation must end existing sessions, not just block new logins.
		if in.IsActive != nil && !*in.IsActive {
			if err := q.RevokeUserRefreshTokens(ctx, dbgen.RevokeUserRefreshTokensParams{
				UserID: id, RevokedAt: pgtype.Timestamptz{Time: now, Valid: true},
			}); err != nil {
				return fmt.Errorf("revoking refresh tokens: %w", err)
			}
		}

		for _, action := range changeActions(before, updated) {
			if err := audit.Record(ctx, tx, audit.Entry{
				Actor: &actor, Action: action, EntityType: audit.EntityUser,
				EntityID: id, Before: before, After: updated, At: now,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, httpx.ErrNotFound
	}
	if err != nil {
		var domain *httpx.CustomDomainError
		if errors.As(err, &domain) || errors.Is(err, httpx.ErrForbidden) {
			return User{}, err
		}
		if d, ok := constraintDetail(err); ok {
			return User{}, httpx.ValidationError(d)
		}
		return User{}, fmt.Errorf("updating user: %w", err)
	}
	return updated, nil
}

// ResetPassword replaces an account's password with a random temporary one and returns it. This is the
// only time the value exists in clear: it is not stored, logged or written to the audit log (BR-24).
func (s *Service) ResetPassword(ctx context.Context, actor httpx.Principal, id uuid.UUID) (string, error) {
	if actor.Role != httpx.RoleAdmin && actor.Role != httpx.RoleTU {
		return "", httpx.ErrForbidden
	}

	temp, err := NewTemporaryPassword(s.opts.TempPasswordLength)
	if err != nil {
		return "", err
	}
	hash, err := s.hasher.Hash(temp)
	if err != nil {
		return "", fmt.Errorf("hashing temporary password: %w", err)
	}

	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		targetRow, err := q.LockUser(ctx, id)
		if err != nil {
			return err
		}
		target := userRow(targetRow).toUser()
		if !mayManage(actor, httpx.Role(target.Role)) {
			return httpx.ErrForbidden
		}

		if err := q.SetUserPassword(ctx, dbgen.SetUserPasswordParams{
			ID: id, PasswordHash: hash, MustChangePassword: true, UpdatedAt: now,
		}); err != nil {
			return fmt.Errorf("storing temporary password: %w", err)
		}
		if err := q.RevokeUserRefreshTokens(ctx, dbgen.RevokeUserRefreshTokensParams{
			UserID: id, RevokedAt: pgtype.Timestamptz{Time: now, Valid: true},
		}); err != nil {
			return fmt.Errorf("revoking refresh tokens: %w", err)
		}
		// The entry says a reset happened, never what the new password is.
		after := target
		after.MustChangePassword = true
		return audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionUserPasswordReset, EntityType: audit.EntityUser,
			EntityID: id, Before: target, After: after, At: now,
		})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", httpx.ErrNotFound
	}
	if err != nil {
		if errors.Is(err, httpx.ErrForbidden) {
			return "", err
		}
		return "", fmt.Errorf("resetting password: %w", err)
	}
	return temp, nil
}

// changeActions names what an update did, one audit action per kind of change. A request that changes
// the role and the active flag together produces both lines, so either can be found by its action.
func changeActions(before, after User) []string {
	var actions []string
	if before.Role != after.Role {
		actions = append(actions, audit.ActionUserRoleChanged)
	}
	if before.IsActive != after.IsActive {
		if after.IsActive {
			actions = append(actions, audit.ActionUserActivated)
		} else {
			actions = append(actions, audit.ActionUserDeactivated)
		}
	}
	if before.BookingBlockedUntil != nil && after.BookingBlockedUntil == nil {
		actions = append(actions, audit.ActionUserBlockCleared)
	}
	if before.Name != after.Name || !samePtr(before.Email, after.Email) ||
		!samePtr(before.PicName, after.PicName) || !samePtr(before.PicPhone, after.PicPhone) {
		actions = append(actions, audit.ActionUserUpdated)
	}
	return actions
}

func samePtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func optionalText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
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

func (s *Service) validatePassword(v string) []httpx.ErrorDetail {
	if n := utf8.RuneCountInString(v); n < s.opts.MinPasswordLength || n > maxPasswordRunes {
		return []httpx.ErrorDetail{{Field: "password", Message: fmt.Sprintf("Password harus %d-%d karakter.", s.opts.MinPasswordLength, maxPasswordRunes)}}
	}
	return nil
}

// ValidatePassword checks a new password against the configured minimum.
func (s *Service) ValidatePassword(v string) []httpx.ErrorDetail { return s.validatePassword(v) }

// validatePIC requires the contact person on organisation accounts (BR-18) and refuses it elsewhere,
// so the data is never silently dropped.
func validatePIC(role httpx.Role, name, phone string) []httpx.ErrorDetail {
	var problems []httpx.ErrorDetail
	if role == httpx.RoleOrganisasi {
		if n := utf8.RuneCountInString(name); n < 1 || n > 120 {
			problems = append(problems, httpx.ErrorDetail{Field: "pic_name", Message: "Nama penanggung jawab wajib diisi untuk akun organisasi (maksimal 120 karakter)."})
		}
		if !phonePattern.MatchString(phone) {
			problems = append(problems, httpx.ErrorDetail{Field: "pic_phone", Message: "Nomor telepon penanggung jawab wajib diisi untuk akun organisasi."})
		}
		return problems
	}
	if name != "" {
		problems = append(problems, httpx.ErrorDetail{Field: "pic_name", Message: "Penanggung jawab hanya untuk akun organisasi."})
	}
	if phone != "" {
		problems = append(problems, httpx.ErrorDetail{Field: "pic_phone", Message: "Penanggung jawab hanya untuk akun organisasi."})
	}
	return problems
}

// constraintDetail maps unique, foreign-key and check violations to a field error instead of a 500.
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
	case pgErr.Code == pgCheckViolation && pgErr.ConstraintName == "users_org_pic_check":
		return httpx.ErrorDetail{Field: "pic_name", Message: "Akun organisasi wajib memiliki nama dan telepon penanggung jawab."}, true
	}
	return httpx.ErrorDetail{}, false
}

// escapeLike neutralises LIKE wildcards so user input is matched literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// ValidateIdentifier, ValidateName and ValidateEmail expose the account field rules to the CSV import,
// so a student created by import obeys the same rules as one created by an administrator.
func ValidateIdentifier(v string) []httpx.ErrorDetail { return validateIdentifier(v) }
func ValidateName(v string) []httpx.ErrorDetail       { return validateName(v) }
func ValidateEmail(v string) []httpx.ErrorDetail      { return validateEmail(v) }
