package bookings

import (
	"errors"
	"strings"
	"testing"

	"classlab/api/internal/httpx"
	"classlab/api/internal/roomtypes"
)

// initialPolicies mirrors the values migration 000006 inserts (brief BR-20).
func initialPolicies() map[string]roomtypes.Policy {
	both := []httpx.Role{httpx.RoleMahasiswa, httpx.RoleOrganisasi}
	return map[string]roomtypes.Policy{
		"KELAS":        {Type: "KELAS", ApprovalPolicy: "AUTO_IF_ELIGIBLE", RequesterRoles: both},
		"LAB":          {Type: "LAB", ApprovalPolicy: "ALWAYS_MANUAL", RequesterRoles: both},
		"AUDITORIUM":   {Type: "AUDITORIUM", ApprovalPolicy: "ALWAYS_MANUAL", RequesterRoles: []httpx.Role{httpx.RoleOrganisasi}},
		"RUANG_UJIAN":  {Type: "RUANG_UJIAN", ApprovalPolicy: "ALWAYS_MANUAL", RequesterRoles: both},
		"RUANG_SIDANG": {Type: "RUANG_SIDANG", ApprovalPolicy: "ALWAYS_MANUAL", RequesterRoles: []httpx.Role{httpx.RoleMahasiswa}},
	}
}

// TestDecideApprovalFollowsTheBriefTable walks every requester role against every room type and several
// point totals. The expected outcome is written out per case from the brief's table, not computed by the
// code under test.
func TestDecideApprovalFollowsTheBriefTable(t *testing.T) {
	roles := []httpx.Role{httpx.RoleMahasiswa, httpx.RoleOrganisasi, httpx.RoleTU, httpx.RoleKalab, httpx.RoleOB, httpx.RoleAdmin}
	types := []string{"KELAS", "LAB", "AUDITORIUM", "RUANG_UJIAN", "RUANG_SIDANG"}
	points := []int{0, 1, 5, 6, 9}
	policies := initialPolicies()

	type outcome struct {
		err    error
		mode   ApprovalMode
		reason ManualReason
	}
	auto := outcome{mode: ModeAuto}
	manual := func(r ManualReason) outcome { return outcome{mode: ModeManual, reason: r} }
	denied := outcome{err: ErrRoleNotAllowed}

	// want[role][type] gives the outcome for a requester with `pts` points.
	want := func(role httpx.Role, typ string, pts int) outcome {
		switch role {
		case httpx.RoleMahasiswa:
			switch typ {
			case "KELAS":
				if pts == 0 {
					return auto
				}
				return manual(ReasonViolationHistory)
			case "AUDITORIUM":
				return denied
			default: // LAB, RUANG_UJIAN, RUANG_SIDANG
				return manual(ReasonRoomPolicy)
			}
		case httpx.RoleOrganisasi:
			if typ == "RUANG_SIDANG" {
				return denied
			}
			return manual(ReasonOrganization)
		default: // staff do not book
			return denied
		}
	}

	for _, role := range roles {
		for _, typ := range types {
			for _, pts := range points {
				got, err := DecideApproval(ApprovalInput{Requester: role, Policy: policies[typ], ActivePoints: pts, AutoApproveMaxPoints: 1})
				exp := want(role, typ, pts)
				if !errors.Is(err, exp.err) || got.Mode != exp.mode || got.Reason != exp.reason {
					t.Errorf("%s booking %s with %d points: got (%q, %q, %v), want (%q, %q, %v)",
						role, typ, pts, got.Mode, got.Reason, err, exp.mode, exp.reason, exp.err)
				}
			}
		}
	}
}

func TestDecideApprovalHonoursTheConfiguredPointLimit(t *testing.T) {
	kelas := initialPolicies()["KELAS"]
	cases := []struct {
		limit, points int
		wantAuto      bool
		reason        ManualReason
	}{
		{1, 0, true, ""},
		{1, 1, false, ReasonViolationHistory},
		{3, 2, true, ""},                      // fewer than the limit
		{3, 3, false, ReasonViolationHistory}, // exactly the limit is not fewer
		{0, 0, false, ReasonRoomPolicy},       // zero switches auto-approval off for everyone
		{-1, 0, false, ReasonRoomPolicy},
	}
	for _, c := range cases {
		got, err := DecideApproval(ApprovalInput{Requester: httpx.RoleMahasiswa, Policy: kelas, ActivePoints: c.points, AutoApproveMaxPoints: c.limit})
		if err != nil {
			t.Fatal(err)
		}
		if (got.Mode == ModeAuto) != c.wantAuto || got.Reason != c.reason {
			t.Errorf("limit %d, points %d: %+v, want auto=%v reason %q", c.limit, c.points, got, c.wantAuto, c.reason)
		}
	}
}

func TestDecideApprovalReadsThePolicyNotTheTypeName(t *testing.T) {
	// A LAB switched to AUTO_IF_ELIGIBLE by an administrator is approved like a classroom; a classroom
	// switched to ALWAYS_MANUAL is not. Nothing in the decision looks at the type's name.
	lab := initialPolicies()["LAB"]
	lab.ApprovalPolicy = "AUTO_IF_ELIGIBLE"
	got, _ := DecideApproval(ApprovalInput{Requester: httpx.RoleMahasiswa, Policy: lab, AutoApproveMaxPoints: 1})
	if got.Mode != ModeAuto {
		t.Errorf("LAB set to AUTO_IF_ELIGIBLE: %+v, want AUTO", got)
	}
	kelas := initialPolicies()["KELAS"]
	kelas.ApprovalPolicy = "ALWAYS_MANUAL"
	got, _ = DecideApproval(ApprovalInput{Requester: httpx.RoleMahasiswa, Policy: kelas, AutoApproveMaxPoints: 1})
	if got.Mode != ModeManual || got.Reason != ReasonRoomPolicy {
		t.Errorf("KELAS set to ALWAYS_MANUAL: %+v, want MANUAL/ROOM_POLICY", got)
	}
	// Letting organisations book a sidang room is a policy change too.
	sidang := initialPolicies()["RUANG_SIDANG"]
	sidang.RequesterRoles = append(sidang.RequesterRoles, httpx.RoleOrganisasi)
	got, err := DecideApproval(ApprovalInput{Requester: httpx.RoleOrganisasi, Policy: sidang, AutoApproveMaxPoints: 1})
	if err != nil || got.Reason != ReasonOrganization {
		t.Errorf("organisation on a widened sidang policy: %+v, %v", got, err)
	}
}

func TestAutoDecisionsCarryNoReasonAndManualOnesAlwaysDo(t *testing.T) {
	for _, policy := range initialPolicies() {
		for _, role := range []httpx.Role{httpx.RoleMahasiswa, httpx.RoleOrganisasi} {
			for _, pts := range []int{0, 1, 4} {
				got, err := DecideApproval(ApprovalInput{Requester: role, Policy: policy, ActivePoints: pts, AutoApproveMaxPoints: 1})
				if err != nil {
					continue
				}
				if (got.Mode == ModeAuto) != (got.Reason == "") {
					t.Errorf("%s on %s with %d points: mode %q with reason %q; the database requires a reason exactly for MANUAL", role, policy.Type, pts, got.Mode, got.Reason)
				}
			}
		}
	}
}

func TestNormalizeDetailsAcceptsOnlyWhatTheTypeDefines(t *testing.T) {
	for name, raw := range map[string]string{"missing": "", "null": "null", "empty object": "{}", "spaced": " {  } "} {
		out, problems := normalizeDetails("KELAS", []byte(raw))
		if len(problems) != 0 || string(out) != "{}" {
			t.Errorf("%s: %q %v, want {} and no problems", name, out, problems)
		}
	}
	for name, raw := range map[string]string{"array": "[]", "string": `"x"`, "number": "3", "broken": "{"} {
		if _, problems := normalizeDetails("KELAS", []byte(raw)); len(problems) != 1 || problems[0].Field != "details" {
			t.Errorf("%s: problems = %v, want one on details", name, problems)
		}
	}
	// The MVP defines no extra fields for any type, so any key is refused and named.
	_, problems := normalizeDetails("LAB", []byte(`{"dosen_pendamping":"Dr. X","penguji":1}`))
	if len(problems) != 1 || problems[0].Message == "" {
		t.Fatalf("problems = %v", problems)
	}
	for _, key := range []string{"dosen_pendamping", "penguji"} {
		if !strings.Contains(problems[0].Message, key) {
			t.Errorf("message %q does not name %s", problems[0].Message, key)
		}
	}
}

func TestOnlyTheAuditoriumRequiresAHeadCount(t *testing.T) {
	for typ := range initialPolicies() {
		if got := specFor(typ).participantsRequired; got != (typ == "AUDITORIUM") {
			t.Errorf("%s participantsRequired = %v", typ, got)
		}
	}
}
