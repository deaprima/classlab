// Package seed loads development data. The accounts it creates share a public password,
// so nothing here may run, or leave rows behind, in production.
package seed

import (
	"context"
	"errors"
	"fmt"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/platform/password"

	"github.com/jackc/pgx/v5"
)

// DevPassword is printed by `make seed` and is intentionally not a secret.
const DevPassword = "classlab-dev-2026"

const orgUnitName = "Rumpun TI"

type account struct {
	Identifier string
	Name       string
	Email      string
	Role       dbgen.UserRole
}

var accounts = []account{
	{"197501012000011001", "Dr. Budi Santoso", "kaprodi@classlab.test", dbgen.UserRoleKAPRODI},
	{"198203152008012002", "Siti Rahma (TU/Kalab)", "tukalab@classlab.test", dbgen.UserRoleTUKALAB},
	{"199001102015011003", "Agus Prasetyo (OB)", "ob@classlab.test", dbgen.UserRoleOB},
	{"2021101001", "Andi Pratama", "andi@classlab.test", dbgen.UserRoleMAHASISWA},
	{"2021101002", "Bunga Lestari", "bunga@classlab.test", dbgen.UserRoleMAHASISWA},
	{"2021101003", "Citra Dewi", "citra@classlab.test", dbgen.UserRoleMAHASISWA},
	{"2021101004", "Dimas Saputra", "dimas@classlab.test", dbgen.UserRoleMAHASISWA},
	{"2021101005", "Eka Wijaya", "eka@classlab.test", dbgen.UserRoleMAHASISWA},
}

func identifiers() []string {
	ids := make([]string, len(accounts))
	for i, a := range accounts {
		ids[i] = a.Identifier
	}
	return ids
}

// Run is idempotent: existing org units and accounts are left untouched.
func Run(ctx context.Context, pool *db.Pool, hasher *password.Hasher, clk clock.Clock) (created int, err error) {
	q := dbgen.New(pool)
	now := clk.Now()

	unit, err := q.GetOrgUnitByNameAndType(ctx, dbgen.GetOrgUnitByNameAndTypeParams{Name: orgUnitName, Type: dbgen.OrgUnitTypeRUMPUN})
	if errors.Is(err, pgx.ErrNoRows) {
		unit, err = q.CreateOrgUnit(ctx, dbgen.CreateOrgUnitParams{Type: dbgen.OrgUnitTypeRUMPUN, Name: orgUnitName, CreatedAt: now})
	}
	if err != nil {
		return 0, fmt.Errorf("ensuring org unit: %w", err)
	}

	// One hash for every account: each login still verifies against its own stored copy.
	hash, err := hasher.Hash(DevPassword)
	if err != nil {
		return 0, fmt.Errorf("hashing seed password: %w", err)
	}

	for _, a := range accounts {
		n, err := q.SeedUser(ctx, dbgen.SeedUserParams{
			OrgUnitID: unit.ID, Identifier: a.Identifier, Name: a.Name, Email: a.Email,
			PasswordHash: hash, Role: a.Role, CreatedAt: now,
		})
		if err != nil {
			return created, fmt.Errorf("seeding %s: %w", a.Identifier, err)
		}
		created += int(n)
	}
	return created, nil
}

// AssertNoDevAccounts fails when any active seed account exists. Production must deactivate or delete them.
func AssertNoDevAccounts(ctx context.Context, pool *db.Pool) error {
	n, err := dbgen.New(pool).CountActiveUsersByIdentifiers(ctx, identifiers())
	if err != nil {
		return fmt.Errorf("checking for dev seed accounts: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("%d active dev seed account(s) found; deactivate them before running in production", n)
	}
	return nil
}

// Accounts lists what Run creates, for the CLI summary.
func Accounts() []string {
	out := make([]string, len(accounts))
	for i, a := range accounts {
		out[i] = fmt.Sprintf("%-9s %s", a.Role, a.Identifier)
	}
	return out
}
