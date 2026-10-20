package roomtypes_test

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/testdb"
	"classlab/api/internal/roomtypes"
)

func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }

// Reading the policy table needs a real database: an array of an enum type is not something pgx decodes
// by itself, and a fake would have hidden exactly that.
func TestStoreReadsTheInitialPoliciesFromTheDatabase(t *testing.T) {
	store := roomtypes.NewStore(testdb.New(t))
	ctx := context.Background()

	all, err := store.All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("policies = %d, want 5", len(all))
	}

	lab, err := store.Get(ctx, "LAB")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(lab.ApproverRoles, []httpx.Role{httpx.RoleKalab}) || lab.ApprovalPolicy != "ALWAYS_MANUAL" || lab.LetterRequired {
		t.Errorf("LAB policy = %+v", lab)
	}
	if lab.OpenMinute != 7*60 || lab.CloseMinute != 21*60 || lab.MaxAdvanceDays != 30 {
		t.Errorf("LAB hours/limits = %d-%d, +%d days", lab.OpenMinute, lab.CloseMinute, lab.MaxAdvanceDays)
	}

	aud, err := store.Get(ctx, "AUDITORIUM")
	if err != nil {
		t.Fatal(err)
	}
	if !aud.LetterRequired || !aud.CanApprove(httpx.RoleAdmin) || aud.CanRequest(httpx.RoleMahasiswa) || !aud.CanRequest(httpx.RoleOrganisasi) {
		t.Errorf("AUDITORIUM policy = %+v", aud)
	}

	if _, err := store.Get(ctx, "GUDANG"); !errors.Is(err, roomtypes.ErrUnknownType) {
		t.Errorf("unknown type: err = %v, want ErrUnknownType", err)
	}
}

func TestStoreAuthorisationFollowsTheTableNotTheCode(t *testing.T) {
	pool := testdb.New(t)
	store := roomtypes.NewStore(pool)
	ctx := context.Background()
	tu := httpx.Principal{Role: httpx.RoleTU}
	kalab := httpx.Principal{Role: httpx.RoleKalab}

	if err := store.RequireApprover(ctx, tu, "KELAS"); err != nil {
		t.Errorf("TU on KELAS: %v", err)
	}
	if err := store.RequireApprover(ctx, tu, "LAB"); !errors.Is(err, httpx.ErrForbidden) {
		t.Errorf("TU on LAB: %v, want forbidden", err)
	}
	if err := store.RequireApprover(ctx, kalab, "KELAS"); !errors.Is(err, httpx.ErrForbidden) {
		t.Errorf("KALAB on KELAS: %v, want forbidden", err)
	}
	if err := store.RequireApprover(ctx, tu, "NOT_A_TYPE"); !errors.Is(err, httpx.ErrForbidden) {
		t.Errorf("unknown type: %v, want forbidden", err)
	}

	// An administrator's change to the policy applies to the very next call: nothing is cached.
	if _, err := pool.Exec(ctx, `UPDATE room_type_policies SET approver_roles = '{TU,KALAB}' WHERE room_type = 'LAB'`); err != nil {
		t.Fatal(err)
	}
	if err := store.RequireApprover(ctx, tu, "LAB"); err != nil {
		t.Errorf("TU on LAB after the policy change: %v, want allowed", err)
	}

	types, err := store.ApprovableTypes(ctx, tu.Role)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(types)
	if want := []string{"AUDITORIUM", "KELAS", "LAB", "RUANG_SIDANG", "RUANG_UJIAN"}; !slices.Equal(types, want) {
		t.Errorf("TU approves %v, want %v", types, want)
	}
	managed, err := store.ManageableTypes(ctx, httpx.RoleAdmin)
	if err != nil || len(managed) != 5 {
		t.Errorf("ADMIN manages %v (%v), want all five types", managed, err)
	}
	none, err := store.ApprovableTypes(ctx, httpx.RoleOB)
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("OB approves %v (%v), want an empty, non-nil list", none, err)
	}
}
