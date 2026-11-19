// Package bootstrap creates what a fresh production database needs before anyone can sign in: the
// organisation unit and the first administrator. The dev seed refuses to run in production, so this
// is the only way in; everything else is done from the web app by that administrator.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/platform/password"
	"classlab/api/internal/users"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// DefaultOrgUnit is the name the dev seed uses too, so a database seeded once and one bootstrapped
// share the same unit.
const DefaultOrgUnit = "Rumpun TI"

// ErrExists is returned when the identifier already belongs to an account; nothing is changed.
var ErrExists = errors.New("an account with this identifier already exists")

// Same rule the user API applies to NIM/NIP.
var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`)

// Admin describes the first administrator.
type Admin struct {
	Identifier string
	Name       string
	// Email is optional.
	Email string
	// OrgUnit is created when missing; DefaultOrgUnit when empty.
	OrgUnit string
}

// ValidationError says which input is wrong, in words for the person at the terminal.
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func (a *Admin) validate() error {
	a.Identifier = strings.TrimSpace(a.Identifier)
	a.Name = strings.TrimSpace(a.Name)
	a.Email = strings.TrimSpace(a.Email)
	a.OrgUnit = strings.TrimSpace(a.OrgUnit)
	if a.OrgUnit == "" {
		a.OrgUnit = DefaultOrgUnit
	}
	switch {
	case !identifierPattern.MatchString(a.Identifier):
		return &ValidationError{"identifier must be 3-64 characters: letters, digits, dot, underscore or dash"}
	case a.Name == "" || utf8.RuneCountInString(a.Name) > 120:
		return &ValidationError{"name is required and at most 120 characters"}
	case utf8.RuneCountInString(a.OrgUnit) > 120:
		return &ValidationError{"org unit name is at most 120 characters"}
	}
	if a.Email != "" {
		if _, err := mail.ParseAddress(a.Email); err != nil {
			return &ValidationError{"email is not a valid address"}
		}
	}
	return nil
}

// CreateAdmin makes the organisation unit when it is missing and an ADMIN account that must change
// its password at first sign-in. It returns the temporary password, which exists nowhere else: only
// its hash is stored, and the caller is expected to show it once.
func CreateAdmin(
	ctx context.Context,
	pool *db.Pool,
	hasher *password.Hasher,
	clk clock.Clock,
	in Admin,
	tempPasswordLength int,
) (string, error) {
	if err := in.validate(); err != nil {
		return "", err
	}
	q := dbgen.New(pool)
	now := clk.Now()

	if _, err := q.GetUserForLogin(ctx, in.Identifier); err == nil {
		return "", ErrExists
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("checking identifier: %w", err)
	}

	unit, err := q.GetOrgUnitByNameAndType(ctx, dbgen.GetOrgUnitByNameAndTypeParams{
		Name: in.OrgUnit, Type: dbgen.OrgUnitTypeRUMPUN,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		unit, err = q.CreateOrgUnit(ctx, dbgen.CreateOrgUnitParams{
			Type: dbgen.OrgUnitTypeRUMPUN, Name: in.OrgUnit, CreatedAt: now,
		})
	}
	if err != nil {
		return "", fmt.Errorf("ensuring org unit: %w", err)
	}

	temp, err := users.NewTemporaryPassword(tempPasswordLength)
	if err != nil {
		return "", err
	}
	hash, err := hasher.Hash(temp)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}

	_, err = q.CreateUser(ctx, dbgen.CreateUserParams{
		OrgUnitID:          unit.ID,
		Identifier:         in.Identifier,
		Name:               in.Name,
		Email:              pgtype.Text{String: in.Email, Valid: in.Email != ""},
		PasswordHash:       hash,
		Role:               dbgen.UserRoleADMIN,
		MustChangePassword: true,
		CreatedAt:          now,
	})
	if err != nil {
		return "", fmt.Errorf("creating administrator: %w", err)
	}
	return temp, nil
}
