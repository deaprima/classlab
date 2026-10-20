package app_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

func (e *env) patchPolicy(token, typ string, body any) response {
	e.t.Helper()
	return e.do("PATCH", "/api/v1/room-types/"+typ, body, bearer(token))
}

func (e *env) policyOf(typ string) map[string]any {
	e.t.Helper()
	res := e.do("GET", "/api/v1/room-types", nil, bearer(e.adminToken()))
	mustStatus(e.t, res, http.StatusOK)
	for _, item := range dataOf(res) {
		if m := item.(map[string]any); m["type"] == typ {
			return m
		}
	}
	e.t.Fatalf("room type %s not in the list", typ)
	return nil
}

func rolesOf(v any) []string {
	var out []string
	for _, r := range v.([]any) {
		out = append(out, r.(string))
	}
	return out
}

func TestRoomTypesListShowsTheFivePoliciesToEveryRole(t *testing.T) {
	e := newEnv(t)
	for name, token := range map[string]string{
		"admin": e.adminToken(), "tu": e.tuToken(), "kalab": e.kalabToken(), "ob": e.obToken(),
		"student": e.login(mahasiswaA).accessToken, "organisation": e.orgToken(),
	} {
		res := e.do("GET", "/api/v1/room-types", nil, bearer(token))
		if res.Code != http.StatusOK || len(dataOf(res)) != 5 {
			t.Errorf("%s: %d with %d items, want 200 with 5", name, res.Code, len(dataOf(res)))
		}
	}
	if res := e.do("GET", "/api/v1/room-types", nil); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d, want 401", res.Code)
	}

	lab := e.policyOf("LAB")
	if !slices.Equal(rolesOf(lab["approver_roles"]), []string{"KALAB"}) || lab["approval_policy"] != "ALWAYS_MANUAL" ||
		lab["open_time"] != "07:00" || lab["close_time"] != "21:00" || lab["max_duration_min"] != float64(480) || lab["max_advance_days"] != float64(30) {
		t.Errorf("LAB policy = %v", lab)
	}
	aud := e.policyOf("AUDITORIUM")
	if aud["letter_required"] != true || !slices.Equal(rolesOf(aud["requester_roles"]), []string{"ORGANISASI"}) || !slices.Equal(rolesOf(aud["approver_roles"]), []string{"TU", "ADMIN"}) {
		t.Errorf("AUDITORIUM policy = %v", aud)
	}
}

func TestOnlyAnAdministratorChangesAPolicy(t *testing.T) {
	e := newEnv(t)
	body := map[string]any{"max_duration_min": 240}

	for name, token := range map[string]string{
		"tu": e.tuToken(), "kalab": e.kalabToken(), "ob": e.obToken(), "student": e.login(mahasiswaA).accessToken, "organisation": e.orgToken(),
	} {
		if res := e.patchPolicy(token, "KELAS", body); res.Code != http.StatusForbidden {
			t.Errorf("%s: %d, want 403", name, res.Code)
		}
	}
	if res := e.do("PATCH", "/api/v1/room-types/KELAS", body); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d, want 401", res.Code)
	}
	if n := e.countRows(`SELECT count(*) FROM audit_logs WHERE action = 'ROOM_TYPE_POLICY_UPDATED'`); n != 0 {
		t.Fatalf("refused changes left %d audit lines", n)
	}

	res := e.patchPolicy(e.adminToken(), "KELAS", map[string]any{
		"max_duration_min": 240, "open_time": "06:00", "approval_policy": "ALWAYS_MANUAL",
		"approver_roles": []string{"TU", "ADMIN", "TU"}, // a repeat is dropped, not refused
	})
	mustStatus(t, res, http.StatusOK)
	got := res.json()
	if got["max_duration_min"] != float64(240) || got["open_time"] != "06:00" || got["close_time"] != "21:00" || got["approval_policy"] != "ALWAYS_MANUAL" ||
		!slices.Equal(rolesOf(got["approver_roles"]), []string{"TU", "ADMIN"}) {
		t.Errorf("updated policy = %v", got)
	}
	// Untouched fields keep their values, and the change is what a later read returns.
	if again := e.policyOf("KELAS"); again["min_duration_min"] != float64(30) || again["max_advance_days"] != float64(30) || again["open_time"] != "06:00" {
		t.Errorf("policy after the change = %v", again)
	}

	var action, actorRole, before, after string
	err := e.pool.QueryRow(t.Context(),
		`SELECT action, actor_role::text, before->>'max_duration_min', after->>'max_duration_min' FROM audit_logs WHERE action = 'ROOM_TYPE_POLICY_UPDATED'`).
		Scan(&action, &actorRole, &before, &after)
	if err != nil || actorRole != "ADMIN" || before != "480" || after != "240" {
		t.Errorf("audit line = %s %s %s -> %s (%v)", action, actorRole, before, after, err)
	}

	if res := e.patchPolicy(e.adminToken(), "GUDANG", body); res.Code != http.StatusNotFound {
		t.Errorf("unknown type: %d, want 404", res.Code)
	}
	if res := e.patchPolicy(e.adminToken(), "KELAS", map[string]any{}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("empty change: %d, want 422", res.Code)
	}
}

func TestPolicyChangesAreValidated(t *testing.T) {
	e := newEnv(t)
	admin := e.adminToken()
	before := e.policyOf("LAB")

	cases := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"no approvers", map[string]any{"approver_roles": []string{}}, "approver_roles"},
		{"a student as approver", map[string]any{"approver_roles": []string{"MAHASISWA"}}, "approver_roles"},
		{"unknown role", map[string]any{"approver_roles": []string{"DEKAN"}}, "approver_roles"},
		{"no requesters", map[string]any{"requester_roles": []string{}}, "requester_roles"},
		{"staff as requester", map[string]any{"requester_roles": []string{"TU"}}, "requester_roles"},
		{"unknown approval policy", map[string]any{"approval_policy": "SOMETIMES"}, "approval_policy"},
		{"time without a colon", map[string]any{"open_time": "0700"}, "open_time"},
		{"hour 25", map[string]any{"close_time": "25:00"}, "close_time"},
		{"open after the current close", map[string]any{"open_time": "21:30"}, "open_time"},
		{"close before the current open", map[string]any{"close_time": "06:30"}, "open_time"},
		{"open equals close", map[string]any{"open_time": "10:00", "close_time": "10:00"}, "open_time"},
		{"off the 30 minute grid", map[string]any{"open_time": "07:15"}, "open_time"},
		{"minimum over the current maximum", map[string]any{"min_duration_min": 600}, "min_duration_min"},
		{"maximum under the current minimum", map[string]any{"max_duration_min": 15}, "min_duration_min"},
		{"minimum off the grid", map[string]any{"min_duration_min": 45}, "min_duration_min"},
		{"no days ahead", map[string]any{"max_advance_days": 0}, "max_advance_days"},
		{"more than a year ahead", map[string]any{"max_advance_days": 366}, "max_advance_days"},
	}
	for _, c := range cases {
		res := e.patchPolicy(admin, "LAB", c.body)
		if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), `"field":"`+c.field+`"`) {
			t.Errorf("%s: %d %s, want 422 naming %s", c.name, res.Code, res.Body.String(), c.field)
		}
	}

	// A refused request changes nothing and logs nothing.
	after := e.policyOf("LAB")
	if after["updated_at"] != before["updated_at"] || after["open_time"] != before["open_time"] || after["max_duration_min"] != before["max_duration_min"] {
		t.Errorf("policy moved by refused requests: %v -> %v", before, after)
	}
	if n := e.countRows(`SELECT count(*) FROM audit_logs WHERE action = 'ROOM_TYPE_POLICY_UPDATED'`); n != 0 {
		t.Errorf("refused changes left %d audit lines", n)
	}

	// Moving both ends at once is judged as a pair, so a window that is fine as a whole is accepted.
	mustStatus(t, e.patchPolicy(admin, "LAB", map[string]any{"open_time": "22:00", "close_time": "24:00"}), http.StatusOK)
	if p := e.policyOf("LAB"); p["open_time"] != "22:00" || p["close_time"] != "24:00" {
		t.Errorf("window = %v - %v", p["open_time"], p["close_time"])
	}
}

func TestPolicyChangesApplyToTheNextBookingOnly(t *testing.T) {
	e := newEnv(t)
	admin := e.adminToken()
	student := e.login(mahasiswaA).accessToken
	room := e.freshRoomOfType("KELAS")

	// A booking made under the old policy stays as it was.
	existing := e.postBooking(student, room, at(mon, "08:00"), at(mon, "12:00"), nil)
	mustStatus(t, existing, http.StatusCreated)

	mustStatus(t, e.patchPolicy(admin, "KELAS", map[string]any{"max_duration_min": 60}), http.StatusOK)
	res := e.postBooking(student, room, at(tue, "08:00"), at(tue, "10:00"), nil)
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), `"field":"end_at"`) {
		t.Errorf("2h under a 1h maximum: %d %s, want 422 naming end_at", res.Code, res.Body.String())
	}
	mustStatus(t, e.postBooking(student, room, at(tue, "08:00"), at(tue, "09:00"), nil), http.StatusCreated)
	if got := e.bookingStatus(idOf(existing)); got != "MENUNGGU" {
		t.Errorf("the earlier booking became %s", got)
	}

	// The booking horizon is policy too: monday is five days after the test clock.
	mustStatus(t, e.patchPolicy(admin, "KELAS", map[string]any{"max_duration_min": 480, "max_advance_days": 3}), http.StatusOK)
	res = e.postBooking(student, room, at(mon, "14:00"), at(mon, "15:00"), nil)
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), `"field":"start_at"`) {
		t.Errorf("five days ahead with a three day horizon: %d %s", res.Code, res.Body.String())
	}

	// So are the default hours.
	mustStatus(t, e.patchPolicy(admin, "KELAS", map[string]any{"max_advance_days": 30, "open_time": "09:00"}), http.StatusOK)
	res = e.postBooking(student, room, at(wed, "08:00"), at(wed, "10:00"), nil)
	if res.Code != http.StatusUnprocessableEntity || res.errorCode() != "OUTSIDE_OPERATING_HOURS" {
		t.Errorf("08:00 after moving the opening to 09:00: %d %s, want 422 OUTSIDE_OPERATING_HOURS", res.Code, res.errorCode())
	}
	mustStatus(t, e.postBooking(student, room, at(wed, "09:00"), at(wed, "10:00"), nil), http.StatusCreated)
}

func TestChangingApproversMovesTheAuthorityAndTheNotifications(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoomOfType("LAB")
	student := e.login(mahasiswaA).accessToken
	tuUser, kalabUser := e.userIDOf(tuID), e.userIDOf(kalabID)

	first := e.postBooking(student, room, at(mon, "08:00"), at(mon, "10:00"), nil)
	mustStatus(t, first, http.StatusCreated)
	if e.notificationCount(kalabUser, "BOOKING_CREATED") != 1 || e.notificationCount(tuUser, "BOOKING_CREATED") != 0 {
		t.Fatal("a LAB request should notify KALAB and not TU")
	}
	if res := e.action(e.tuToken(), idOf(first), "approve", nil); res.Code != http.StatusForbidden {
		t.Fatalf("TU approving a LAB booking: %d, want 403", res.Code)
	}

	// The administrator hands LABs to TU: authority and notifications follow at once.
	mustStatus(t, e.patchPolicy(e.adminToken(), "LAB", map[string]any{"approver_roles": []string{"TU"}}), http.StatusOK)
	second := e.postBooking(student, room, at(tue, "08:00"), at(tue, "10:00"), nil)
	mustStatus(t, second, http.StatusCreated)
	if e.notificationCount(tuUser, "BOOKING_CREATED") != 1 || e.notificationCount(kalabUser, "BOOKING_CREATED") != 1 {
		t.Errorf("after the change TU has %d, KALAB %d notifications; want 1 and 1 (the old one stays)",
			e.notificationCount(tuUser, "BOOKING_CREATED"), e.notificationCount(kalabUser, "BOOKING_CREATED"))
	}
	if res := e.action(e.kalabToken(), idOf(second), "approve", nil); res.Code != http.StatusForbidden {
		t.Errorf("KALAB approving after losing LAB: %d, want 403", res.Code)
	}
	mustStatus(t, e.action(e.tuToken(), idOf(second), "approve", nil), http.StatusOK)
	// Reading follows authority too: KALAB no longer sees LAB bookings, TU does.
	if res := e.do("GET", "/api/v1/bookings/"+idOf(second), nil, bearer(e.kalabToken())); res.Code != http.StatusNotFound {
		t.Errorf("KALAB reading a LAB booking after the change: %d, want 404", res.Code)
	}
	mustStatus(t, e.do("GET", "/api/v1/bookings/"+idOf(second), nil, bearer(e.tuToken())), http.StatusOK)
}

func TestRequesterRolesDecideWhoMayBook(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoomOfType("RUANG_SIDANG") // MAHASISWA only
	org := e.orgToken()
	if res := e.postBooking(org, room, at(mon, "08:00"), at(mon, "10:00"), nil); res.Code != http.StatusForbidden || res.errorCode() != "ROOM_TYPE_NOT_ALLOWED" {
		t.Fatalf("organisation booking a sidang room: %d %s, want 403 ROOM_TYPE_NOT_ALLOWED", res.Code, res.errorCode())
	}
	mustStatus(t, e.patchPolicy(e.adminToken(), "RUANG_SIDANG", map[string]any{"requester_roles": []string{"MAHASISWA", "ORGANISASI"}}), http.StatusOK)
	// An organisation always attaches a letter (BR-18).
	mustStatus(t, e.postBooking(org, room, at(mon, "08:00"), at(mon, "10:00"), map[string]any{"letter_attachment_id": e.letterFor(org)}), http.StatusCreated)
}
