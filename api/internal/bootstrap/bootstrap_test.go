package bootstrap_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"classlab/api/internal/bootstrap"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/password"
	"classlab/api/internal/platform/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

var (
	hasher = password.NewHasher(8, 1, 1)
	clk    = clock.NewMockClock(time.Date(2026, 10, 22, 3, 0, 0, 0, time.UTC))
)

func TestCreateAdminMakesUnitAndAccountThatMustChangeThePassword(t *testing.T) {
	pool := testdb.New(t)

	temp, err := bootstrap.CreateAdmin(t.Context(), pool, hasher, clk,
		bootstrap.Admin{Identifier: "ADM-001", Name: "Admin Kampus", Email: "admin@kampus.test"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(temp) != 10 {
		t.Errorf("temporary password has %d characters; want 10", len(temp))
	}

	var role, unit, hash string
	var mustChange, active bool
	err = pool.QueryRow(t.Context(),
		`SELECT u.role::text, o.name, u.password_hash, u.must_change_password, u.is_active
		   FROM users u JOIN org_units o ON o.id = u.org_unit_id WHERE u.identifier = 'ADM-001'`,
	).Scan(&role, &unit, &hash, &mustChange, &active)
	if err != nil {
		t.Fatal(err)
	}
	if role != "ADMIN" || unit != bootstrap.DefaultOrgUnit || !mustChange || !active {
		t.Errorf("got role=%s unit=%q mustChange=%v active=%v", role, unit, mustChange, active)
	}
	// Only the hash is stored, and it verifies the password that was shown.
	if hash == temp {
		t.Error("the password is stored in plain text")
	}
	if ok, err := hasher.Verify(temp, hash); err != nil || !ok {
		t.Errorf("stored hash does not verify the shown password: ok=%v err=%v", ok, err)
	}
}

func TestCreateAdminRefusesAnExistingIdentifierAndChangesNothing(t *testing.T) {
	pool := testdb.New(t)
	in := bootstrap.Admin{Identifier: "ADM-001", Name: "Admin Kampus"}
	if _, err := bootstrap.CreateAdmin(t.Context(), pool, hasher, clk, in, 10); err != nil {
		t.Fatal(err)
	}

	_, err := bootstrap.CreateAdmin(t.Context(), pool, hasher, clk, in, 10)
	if !errors.Is(err, bootstrap.ErrExists) {
		t.Fatalf("second run: %v; want ErrExists", err)
	}
	var users, units int
	if err := pool.QueryRow(t.Context(),
		`SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM org_units)`).Scan(&users, &units); err != nil {
		t.Fatal(err)
	}
	if users != 1 || units != 1 {
		t.Errorf("users=%d units=%d; want 1 and 1", users, units)
	}
}

func TestCreateAdminReusesTheNamedOrgUnit(t *testing.T) {
	pool := testdb.New(t)
	for _, id := range []string{"ADM-001", "ADM-002"} {
		_, err := bootstrap.CreateAdmin(t.Context(), pool, hasher, clk,
			bootstrap.Admin{Identifier: id, Name: "Admin " + id, OrgUnit: "Fakultas Teknik"}, 10)
		if err != nil {
			t.Fatal(err)
		}
	}
	var units int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM org_units`).Scan(&units); err != nil {
		t.Fatal(err)
	}
	if units != 1 {
		t.Errorf("%d org units; want the named one reused", units)
	}
}

func TestCreateAdminRejectsBadInputBeforeTouchingTheDatabase(t *testing.T) {
	pool := testdb.New(t)
	for name, in := range map[string]bootstrap.Admin{
		"short identifier": {Identifier: "ab", Name: "X"},
		"bad characters":   {Identifier: "adm 001", Name: "X"},
		"no name":          {Identifier: "ADM-001", Name: "  "},
		"bad email":        {Identifier: "ADM-001", Name: "X", Email: "not-an-email"},
	} {
		_, err := bootstrap.CreateAdmin(t.Context(), pool, hasher, clk, in, 10)
		var invalid *bootstrap.ValidationError
		if !errors.As(err, &invalid) {
			t.Errorf("%s: %v; want a ValidationError", name, err)
		}
	}
	var units int
	_ = pool.QueryRow(t.Context(), `SELECT count(*) FROM org_units`).Scan(&units)
	if units != 0 {
		t.Errorf("bad input created %d org unit(s)", units)
	}
}
