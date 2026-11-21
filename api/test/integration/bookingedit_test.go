package integration_test

import (
	"net/http"
	"testing"
	"time"
)

func (e *env) patchBooking(token, id string, body any) response {
	e.t.Helper()
	return e.do("PATCH", "/api/v1/bookings/"+id, body, bearer(token))
}

// eventKinds lists "KIND to_status" of the booking's events as the API shows them.
func (e *env) eventKinds(token, id string) []map[string]any {
	e.t.Helper()
	res := e.do("GET", "/api/v1/bookings/"+id+"/events", nil, bearer(token))
	mustStatus(e.t, res, http.StatusOK)
	var out []map[string]any
	for _, v := range dataOf(res) {
		out = append(out, v.(map[string]any))
	}
	return out
}

func TestEditAWaitingRequestKeepsItWaitingAndNeverExtendsTheExpiry(t *testing.T) {
	e := newEnv(t)
	lab := e.insertRoom("ED-WAIT", "LAB", 30)
	student := e.login(mahasiswaA).accessToken
	created := e.postBooking(student, lab, at(wed, "08:00"), at(wed, "10:00"), nil)
	mustStatus(t, created, http.StatusCreated)
	id := idOf(created)
	expires := created.json()["expires_at"]

	e.clock.Add(2 * time.Hour)
	student = e.login(mahasiswaA).accessToken
	res := e.patchBooking(student, id, map[string]any{"purpose": "Praktikum tambahan", "start_at": at(wed, "09:00"), "end_at": at(wed, "11:00")})
	mustStatus(t, res, http.StatusOK)
	b := res.json()
	if b["status"] != "MENUNGGU" || b["purpose"] != "Praktikum tambahan" || b["expires_at"] != expires {
		t.Errorf("status %v purpose %v expires %v (was %v)", b["status"], b["purpose"], b["expires_at"], expires)
	}
	if b["edited_at"] == nil {
		t.Error("edited_at not set")
	}
	events := e.eventKinds(student, id)
	last := events[len(events)-1]
	if last["kind"] != "EDITED" {
		t.Fatalf("events = %v; want an EDITED event last", events)
	}
	changes := last["changes"].(map[string]any)
	if _, ok := changes["purpose"]; !ok {
		t.Errorf("changes = %v; want purpose", changes)
	}
	startChange := changes["start_at"].(map[string]any)
	if startChange["from"] != at(wed, "08:00") || startChange["to"] != at(wed, "09:00") {
		t.Errorf("start_at change = %v", startChange)
	}
	if _, ok := changes["notes"]; ok {
		t.Error("an unchanged field appears in changes")
	}
}

func TestEditIsOpenUntilSixtyMinutesBeforeTheStart(t *testing.T) {
	e := newEnv(t)
	lab := e.insertRoom("ED-CUT", "LAB", 30)
	student := e.login(mahasiswaA).accessToken
	id := idOf(e.postBooking(student, lab, at(tue, "08:00"), at(tue, "10:00"), nil))

	// Exactly 60 minutes before 08:00 WIB (01:00 UTC): still allowed.
	e.clock.Set(time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC))
	student = e.login(mahasiswaA).accessToken
	mustStatus(t, e.patchBooking(student, id, map[string]any{"notes": "bawa laptop"}), http.StatusOK)

	e.clock.Add(time.Second)
	student = e.login(mahasiswaA).accessToken
	res := e.patchBooking(student, id, map[string]any{"notes": "bawa kabel"})
	if res.Code != http.StatusConflict || res.errorCode() != "EDIT_WINDOW_CLOSED" {
		t.Errorf("one second later: %d %s; want 409 EDIT_WINDOW_CLOSED", res.Code, res.Body.String())
	}
}

func TestEditingAnApprovedBookingByAPersonSendsItBackUnlessOnlyTheNotesChange(t *testing.T) {
	e := newEnv(t)
	lab := e.insertRoom("ED-APP", "LAB", 30)
	student := e.login(mahasiswaA).accessToken
	id := idOf(e.postBooking(student, lab, at(wed, "08:00"), at(wed, "10:00"), nil))
	kalab := e.kalabToken()
	mustStatus(t, e.action(kalab, id, "approve", nil), http.StatusOK)
	before := e.notificationCount(e.userIDOf(kalabID), "BOOKING_EDITED")

	// Notes alone change nothing about the decision.
	res := e.patchBooking(student, id, map[string]any{"notes": "Mohon siapkan proyektor"})
	mustStatus(t, res, http.StatusOK)
	if res.json()["status"] != "DISETUJUI" {
		t.Errorf("status after a notes-only edit = %v; want DISETUJUI", res.json()["status"])
	}
	if n := e.notificationCount(e.userIDOf(kalabID), "BOOKING_EDITED"); n != before {
		t.Error("approvers were notified of a change they need not act on")
	}

	// A change to the time needs a fresh decision.
	e.clock.Add(time.Hour)
	student = e.login(mahasiswaA).accessToken
	res = e.patchBooking(student, id, map[string]any{"end_at": at(wed, "11:00")})
	mustStatus(t, res, http.StatusOK)
	b := res.json()
	if b["status"] != "MENUNGGU" || b["approval_mode"] != "MANUAL" || b["expires_at"] == nil || b["decided_by"] != nil {
		t.Errorf("after a material edit: status %v mode %v expires %v decided_by %v", b["status"], b["approval_mode"], b["expires_at"], b["decided_by"])
	}
	if n := e.notificationCount(e.userIDOf(kalabID), "BOOKING_EDITED"); n != before+1 {
		t.Errorf("BOOKING_EDITED notifications = %d; want %d", n, before+1)
	}
	kinds := e.eventKinds(student, id)
	if len(kinds) < 3 {
		t.Fatalf("events = %v", kinds)
	}
	tail := kinds[len(kinds)-2:]
	if tail[0]["kind"] != "EDITED" || tail[1]["kind"] != "STATUS" || tail[1]["from_status"] != "DISETUJUI" || tail[1]["to_status"] != "MENUNGGU" {
		t.Errorf("last events = %v; want EDITED then STATUS DISETUJUI->MENUNGGU", tail)
	}
}

func TestEditingASystemApprovedBookingRunsTheDecisionAgain(t *testing.T) {
	e := newEnv(t, withAutoApproval)
	room := e.insertRoom("ED-AUTO", "KELAS", 40)
	student := e.login(mahasiswaA).accessToken
	id := idOf(e.postBooking(student, room, at(wed, "08:00"), at(wed, "10:00"), nil))
	if e.bookingStatus(id) != "DISETUJUI" {
		t.Fatal("fixture should be auto-approved")
	}

	// Still eligible: it stays approved by the system.
	res := e.patchBooking(student, id, map[string]any{"purpose": "Diskusi tugas akhir"})
	mustStatus(t, res, http.StatusOK)
	if res.json()["status"] != "DISETUJUI" || res.json()["approval_mode"] != "AUTO" {
		t.Errorf("still eligible: status %v mode %v", res.json()["status"], res.json()["approval_mode"])
	}

	// A violation earned in the meantime makes it a decision for a person.
	e.addViolation(mahasiswaA)
	res = e.patchBooking(student, id, map[string]any{"purpose": "Diskusi tugas akhir lanjutan"})
	mustStatus(t, res, http.StatusOK)
	b := res.json()
	if b["status"] != "MENUNGGU" || b["approval_mode"] != "MANUAL" || b["manual_reason"] != "VIOLATION_HISTORY" {
		t.Errorf("no longer eligible: status %v mode %v reason %v", b["status"], b["approval_mode"], b["manual_reason"])
	}
}

func TestEditIsChecked(t *testing.T) {
	e := newEnv(t)
	lab := e.insertRoom("ED-CHK", "LAB", 20)
	e.insertSchedule(lab, "Kuliah", at(wed, "13:00"), at(wed, "15:00"))
	student := e.login(mahasiswaA).accessToken
	other := e.login(mahasiswaB).accessToken
	id := idOf(e.postBooking(student, lab, at(wed, "08:00"), at(wed, "10:00"), nil))

	cases := []struct {
		name  string
		tok   string
		body  any
		code  int
		error string
	}{
		{"empty body", student, map[string]any{}, 422, "VALIDATION_ERROR"},
		{"room cannot change", student, map[string]any{"room_id": e.insertRoom("ED-OTHER", "LAB", 20)}, 400, ""},
		{"collision", student, map[string]any{"start_at": at(wed, "12:00"), "end_at": at(wed, "14:00")}, 409, "BOOKING_CONFLICT"},
		{"outside the hours", student, map[string]any{"start_at": at(wed, "05:00"), "end_at": at(wed, "06:00")}, 422, "OUTSIDE_OPERATING_HOURS"},
		{"over the capacity", student, map[string]any{"participants": 21}, 422, "VALIDATION_ERROR"},
		{"end before start", student, map[string]any{"start_at": at(wed, "10:00"), "end_at": at(wed, "09:00")}, 422, "VALIDATION_ERROR"},
		{"someone else's booking", other, map[string]any{"purpose": "x"}, 404, "NOT_FOUND"},
		{"an approver", e.kalabToken(), map[string]any{"purpose": "x"}, 403, "FORBIDDEN"},
		{"no token", "", map[string]any{"purpose": "x"}, 401, ""},
	}
	for _, c := range cases {
		var res response
		if c.tok == "" {
			res = e.do("PATCH", "/api/v1/bookings/"+id, c.body)
		} else {
			res = e.patchBooking(c.tok, id, c.body)
		}
		if res.Code != c.code || (c.error != "" && res.errorCode() != c.error) {
			t.Errorf("%s: %d %s; want %d %s", c.name, res.Code, res.errorCode(), c.code, c.error)
		}
	}
	if e.eventCount(id) != 1 {
		t.Errorf("refused edits left %d events", e.eventCount(id))
	}

	// A booking can be moved over its own old slot.
	mustStatus(t, e.patchBooking(student, id, map[string]any{"start_at": at(wed, "09:00"), "end_at": at(wed, "11:00")}), http.StatusOK)

	// Only waiting and approved bookings can be edited.
	mustStatus(t, e.action(student, id, "cancel", nil), http.StatusOK)
	if res := e.patchBooking(student, id, map[string]any{"purpose": "x"}); res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
		t.Errorf("edit after cancel: %d %s; want 409 INVALID_STATE", res.Code, res.Body.String())
	}
}

func TestEditingOneDateOfAGroupLeavesTheOthersAlone(t *testing.T) {
	e := newEnv(t)
	lab := e.insertRoom("ED-GRP", "LAB", 30)
	student := e.login(mahasiswaA).accessToken
	res := e.postGroup(student, lab, []map[string]string{occ(tue, "08:00", "09:00"), occ(wed, "08:00", "09:00")}, nil)
	_, ids := groupIDs(t, res)

	mustStatus(t, e.patchBooking(student, ids[0], map[string]any{"end_at": at(tue, "10:00")}), http.StatusOK)
	got := e.do("GET", "/api/v1/bookings/"+ids[1], nil, bearer(student))
	if got.json()["end_at"] != at(wed, "09:00") || got.json()["edited_at"] != nil {
		t.Errorf("the other date changed: end %v edited_at %v", got.json()["end_at"], got.json()["edited_at"])
	}
}
