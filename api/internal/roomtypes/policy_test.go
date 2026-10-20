package roomtypes

import (
	"testing"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/jackc/pgx/v5/pgtype"
)

// The initial policies from brief BR-20, as the migration inserts them.
func seedPolicies() map[string]Policy {
	return map[string]Policy{
		"KELAS":        {Type: "KELAS", ApproverRoles: []httpx.Role{httpx.RoleTU}, RequesterRoles: []httpx.Role{httpx.RoleMahasiswa, httpx.RoleOrganisasi}},
		"LAB":          {Type: "LAB", ApproverRoles: []httpx.Role{httpx.RoleKalab}, RequesterRoles: []httpx.Role{httpx.RoleMahasiswa, httpx.RoleOrganisasi}},
		"AUDITORIUM":   {Type: "AUDITORIUM", ApproverRoles: []httpx.Role{httpx.RoleTU, httpx.RoleAdmin}, RequesterRoles: []httpx.Role{httpx.RoleOrganisasi}},
		"RUANG_UJIAN":  {Type: "RUANG_UJIAN", ApproverRoles: []httpx.Role{httpx.RoleTU, httpx.RoleAdmin}, RequesterRoles: []httpx.Role{httpx.RoleMahasiswa, httpx.RoleOrganisasi}},
		"RUANG_SIDANG": {Type: "RUANG_SIDANG", ApproverRoles: []httpx.Role{httpx.RoleTU, httpx.RoleAdmin}, RequesterRoles: []httpx.Role{httpx.RoleMahasiswa}},
	}
}

func TestApprovalAndManagementFollowThePolicy(t *testing.T) {
	roles := []httpx.Role{httpx.RoleMahasiswa, httpx.RoleOrganisasi, httpx.RoleTU, httpx.RoleKalab, httpx.RoleOB, httpx.RoleAdmin}

	// want[type][role] = {approve, manageRooms}
	type access struct{ approve, manage bool }
	want := map[string]map[httpx.Role]access{
		"KELAS":        {httpx.RoleTU: {true, true}, httpx.RoleAdmin: {false, true}},
		"LAB":          {httpx.RoleKalab: {true, true}, httpx.RoleAdmin: {false, true}},
		"AUDITORIUM":   {httpx.RoleTU: {true, true}, httpx.RoleAdmin: {true, true}},
		"RUANG_UJIAN":  {httpx.RoleTU: {true, true}, httpx.RoleAdmin: {true, true}},
		"RUANG_SIDANG": {httpx.RoleTU: {true, true}, httpx.RoleAdmin: {true, true}},
	}
	for typ, p := range seedPolicies() {
		for _, role := range roles {
			exp := want[typ][role] // zero value = no access
			if got := p.CanApprove(role); got != exp.approve {
				t.Errorf("%s: CanApprove(%s) = %v, want %v", typ, role, got, exp.approve)
			}
			if got := p.CanManageRooms(role); got != exp.manage {
				t.Errorf("%s: CanManageRooms(%s) = %v, want %v", typ, role, got, exp.manage)
			}
		}
	}
}

func TestCanRequestFollowsRequesterRoles(t *testing.T) {
	p := seedPolicies()
	cases := []struct {
		typ  string
		role httpx.Role
		want bool
	}{
		{"KELAS", httpx.RoleMahasiswa, true},
		{"KELAS", httpx.RoleOrganisasi, true},
		{"AUDITORIUM", httpx.RoleMahasiswa, false},
		{"AUDITORIUM", httpx.RoleOrganisasi, true},
		{"RUANG_SIDANG", httpx.RoleOrganisasi, false},
		{"LAB", httpx.RoleTU, false},
	}
	for _, c := range cases {
		if got := p[c.typ].CanRequest(c.role); got != c.want {
			t.Errorf("%s: CanRequest(%s) = %v, want %v", c.typ, c.role, got, c.want)
		}
	}
}

func TestFromRowConvertsTimesAndDurations(t *testing.T) {
	row := policyRow{
		RoomType:       dbgen.RoomTypeAUDITORIUM,
		ApproverRoles:  []string{"TU", "ADMIN"},
		RequesterRoles: []string{"ORGANISASI"},
		ApprovalPolicy: dbgen.ApprovalPolicyALWAYSMANUAL,
		LetterRequired: true,
		OpenTime:       pgtype.Time{Microseconds: int64(7 * time.Hour / time.Microsecond), Valid: true},
		CloseTime:      pgtype.Time{Microseconds: int64(22 * time.Hour / time.Microsecond), Valid: true},
		MinDurationMin: 60,
		MaxDurationMin: 720,
		MaxAdvanceDays: 90,
	}
	p := fromRow(row)
	if p.OpenMinute != 7*60 || p.CloseMinute != 22*60 {
		t.Errorf("hours = %d-%d, want %d-%d", p.OpenMinute, p.CloseMinute, 7*60, 22*60)
	}
	if p.MinDuration != time.Hour || p.MaxDuration != 12*time.Hour || p.MaxAdvanceDays != 90 {
		t.Errorf("limits = %v %v %d", p.MinDuration, p.MaxDuration, p.MaxAdvanceDays)
	}
	if !p.LetterRequired || p.ApprovalPolicy != "ALWAYS_MANUAL" || !p.CanApprove(httpx.RoleAdmin) {
		t.Errorf("policy = %+v", p)
	}
}
