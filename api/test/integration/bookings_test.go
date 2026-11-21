package integration_test

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// bookingBody is a valid request for the given room and slot; extra overrides or adds fields.
func bookingBody(room, start, end string, extra map[string]any) map[string]any {
	body := map[string]any{"room_id": room, "start_at": start, "end_at": end, "purpose": "Latihan kelompok"}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

func (e *env) postBooking(token, room, start, end string, extra map[string]any, opts ...reqOpt) response {
	e.t.Helper()
	return e.do("POST", "/api/v1/bookings", bookingBody(room, start, end, extra), append([]reqOpt{bearer(token)}, opts...)...)
}

func idOf(res response) string {
	id, _ := res.json()["id"].(string)
	return id
}

func statusOf(res response) string {
	s, _ := res.json()["status"].(string)
	return s
}

func (e *env) bookingStatus(id string) string {
	e.t.Helper()
	var s string
	if err := e.pool.QueryRow(e.t.Context(), "SELECT status FROM bookings WHERE id = $1", id).Scan(&s); err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *env) eventCount(bookingID string) int {
	e.t.Helper()
	return e.countRows("SELECT count(*) FROM booking_events WHERE booking_id = $1", bookingID)
}

func (e *env) notificationCount(userID, typ string) int {
	e.t.Helper()
	return e.countRows("SELECT count(*) FROM notifications WHERE user_id = $1 AND type = $2", userID, typ)
}

func (e *env) action(token, id, action string, body any) response {
	e.t.Helper()
	return e.do("POST", "/api/v1/bookings/"+id+"/"+action, body, bearer(token))
}

var roomSeq int

// freshRoom inserts a room with no schedules, so tests control exactly what occupies it.
func (e *env) freshRoom() string {
	roomSeq++
	return e.insertRoom("BK-"+strconv.Itoa(roomSeq), "KELAS", 40)
}

// pendingBookingOf inserts a MENUNGGU booking for the student in its own room, bypassing the API.
func (e *env) pendingBookingOf(student string) string {
	e.t.Helper()
	return e.insertBooking(e.freshRoom(), student, "MENUNGGU", "Fixture", at(tue, "08:00"), at(tue, "10:00"))
}

// bookingIn creates a booking for owner and walks it to the wanted status through the API.
func (e *env) bookingIn(status, owner string) string {
	e.t.Helper()
	student := e.login(owner).accessToken
	res := e.postBooking(student, e.freshRoom(), at(tue, "08:00"), at(tue, "10:00"), nil)
	mustStatus(e.t, res, http.StatusCreated)
	id := idOf(res)

	switch status {
	case "MENUNGGU":
	case "DISETUJUI":
		mustStatus(e.t, e.action(e.tuToken(), id, "approve", nil), http.StatusOK)
	case "DITOLAK":
		mustStatus(e.t, e.action(e.tuToken(), id, "reject", map[string]string{"reason": "Dipakai ujian"}), http.StatusOK)
	case "DIBATALKAN":
		mustStatus(e.t, e.action(student, id, "cancel", nil), http.StatusOK)
	default:
		e.t.Fatalf("unknown status %s", status)
	}
	return id
}

func TestCreateBooking(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	student := e.login(mahasiswaA)

	res := e.postBooking(student.accessToken, room, at(tue, "13:00"), at(tue, "15:00"),
		map[string]any{"participants": 8, "notes": "  Butuh proyektor  ", "purpose": "  Latihan ManPro  "})
	mustStatus(t, res, http.StatusCreated)

	b := res.json()
	if b["status"] != "MENUNGGU" || b["purpose"] != "Latihan ManPro" || b["notes"] != "Butuh proyektor" || b["participants"] != float64(8) {
		t.Errorf("unexpected booking: %v", b)
	}
	if b["start_at"] != at(tue, "13:00") || b["end_at"] != at(tue, "15:00") {
		t.Errorf("times should come back in WIB: %v - %v", b["start_at"], b["end_at"])
	}
	if b["room_id"] != room || b["requester_id"] != student.userID {
		t.Errorf("room/requester mismatch: %v", b)
	}
	if r, _ := b["room"].(map[string]any); r["code"] == nil {
		t.Errorf("room summary missing: %v", b["room"])
	}
	if u, _ := b["requester"].(map[string]any); u["identifier"] != mahasiswaA || u["email"] != nil {
		t.Errorf("requester should expose identifier and name only: %v", b["requester"])
	}
	if b["decided_by"] != nil || b["decided_at"] != nil {
		t.Errorf("a new booking has no decision yet: %v", b)
	}

	// The creation is the first audit event.
	if n := e.eventCount(idOf(res)); n != 1 {
		t.Errorf("events = %d; want 1", n)
	}
	var from *string
	var to, actor string
	if err := e.pool.QueryRow(t.Context(), "SELECT from_status, to_status, actor_id FROM booking_events WHERE booking_id = $1", idOf(res)).Scan(&from, &to, &actor); err != nil {
		t.Fatal(err)
	}
	if from != nil || to != "MENUNGGU" || actor != student.userID {
		t.Errorf("creation event = %v -> %s by %s", from, to, actor)
	}
}

func TestCreateBookingValidation(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	tok := e.login(mahasiswaA).accessToken
	const today = "2026-10-14" // the test clock reads 10:00 WIB

	cases := []struct {
		name       string
		start, end string
		extra      map[string]any
		field      string
	}{
		{"end before start", at(tue, "10:00"), at(tue, "09:00"), nil, "end_at"},
		{"zero length", at(tue, "10:00"), at(tue, "10:00"), nil, "end_at"},
		{"crosses midnight", at(tue, "20:00"), at(wed, "01:00"), nil, "end_at"},
		{"start off the half hour", at(tue, "08:15"), at(tue, "09:00"), nil, "start_at"},
		{"end off the half hour", at(tue, "08:00"), at(tue, "09:10"), nil, "end_at"},
		{"in the past", at(today, "08:00"), at(today, "09:00"), nil, "start_at"},
		{"longer than 8 hours", at(tue, "07:00"), at(tue, "16:00"), nil, "end_at"},
		{"more than 30 days ahead", at("2026-11-14", "08:00"), at("2026-11-14", "09:00"), nil, "start_at"},
		{"blank purpose", at(tue, "08:00"), at(tue, "09:00"), map[string]any{"purpose": "  "}, "purpose"},
		{"zero participants", at(tue, "08:00"), at(tue, "09:00"), map[string]any{"participants": 0}, "participants"},
		{"huge notes", at(tue, "08:00"), at(tue, "09:00"), map[string]any{"notes": strings.Repeat("x", 1001)}, "notes"},
	}
	for _, tc := range cases {
		res := e.postBooking(tok, room, tc.start, tc.end, tc.extra)
		if res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)[tc.field] {
			t.Errorf("%s: %d %s; want 422 naming %q", tc.name, res.Code, res.Body.String(), tc.field)
		}
	}

	// Outside the room's operating hours has its own code (BR-29): the effective window comes back in details.
	for name, w := range map[string][2]string{
		"before opening": {at(tue, "06:00"), at(tue, "08:00")},
		"after closing":  {at(tue, "20:00"), at(tue, "22:00")},
	} {
		res := e.postBooking(tok, room, w[0], w[1], nil)
		if res.Code != http.StatusUnprocessableEntity || res.errorCode() != "OUTSIDE_OPERATING_HOURS" {
			t.Errorf("%s: %d %s; want 422 OUTSIDE_OPERATING_HOURS", name, res.Code, res.errorCode())
		}
	}

	// Edges that must pass: exactly 8 hours, first and last minute of the day, 30 days ahead, later today.
	for name, w := range map[string][2]string{
		"exactly 8 hours": {at(wed, "07:00"), at(wed, "15:00")},
		"last slot":       {at(wed, "20:30"), at(wed, "21:00")},
		"30 days ahead":   {at("2026-11-13", "08:00"), at("2026-11-13", "09:00")},
		"later today":     {at(today, "13:00"), at(today, "14:00")},
	} {
		if res := e.postBooking(tok, room, w[0], w[1], nil); res.Code != http.StatusCreated {
			t.Errorf("%s: %d %s; want 201", name, res.Code, res.Body.String())
		}
	}

	if res := e.postBooking(tok, "00000000-0000-4000-8000-000000000001", at(tue, "08:00"), at(tue, "09:00"), nil); res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)["room_id"] {
		t.Errorf("unknown room: %d %s; want 422 naming room_id", res.Code, res.Body.String())
	}
	if res := e.do("POST", "/api/v1/bookings", map[string]any{"room_id": room}, bearer(tok)); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing fields: %d; want 422", res.Code)
	}
	for name, body := range map[string]string{
		"timestamp without offset": `{"room_id":"` + room + `","purpose":"x","start_at":"2026-10-20T08:00:00","end_at":"2026-10-20T09:00:00"}`,
		"not json":                 `start=1`,
		"unknown field":            `{"room_id":"` + room + `","bogus":1}`,
	} {
		if res := e.do("POST", "/api/v1/bookings", body, bearer(tok)); res.Code != http.StatusBadRequest {
			t.Errorf("%s: %d; want 400", name, res.Code)
		}
	}
	if n := e.countRows("SELECT count(*) FROM bookings"); n != 4 {
		t.Errorf("%d bookings stored; want only the 4 valid edge cases", n)
	}
}

// BR-02: intervals are half-open, so back-to-back bookings coexist and overlapping ones do not.
func TestBookingOverlapRules(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	tok := e.login(mahasiswaA).accessToken
	other := e.login(mahasiswaB).accessToken

	mustStatus(t, e.postBooking(tok, room, at(tue, "08:00"), at(tue, "10:00"), nil), http.StatusCreated)
	mustStatus(t, e.postBooking(other, room, at(tue, "10:00"), at(tue, "12:00"), nil), http.StatusCreated)
	mustStatus(t, e.postBooking(other, room, at(tue, "07:00"), at(tue, "08:00"), nil), http.StatusCreated)

	for name, w := range map[string][2]string{
		"inside":        {at(tue, "08:30"), at(tue, "09:30")},
		"straddles end": {at(tue, "09:30"), at(tue, "10:30")},
		"covers both":   {at(tue, "07:30"), at(tue, "12:30")},
		"same slot":     {at(tue, "08:00"), at(tue, "10:00")},
	} {
		res := e.postBooking(other, room, w[0], w[1], nil)
		if res.Code != http.StatusConflict || res.errorCode() != "BOOKING_CONFLICT" {
			t.Errorf("%s: %d %s; want 409 BOOKING_CONFLICT", name, res.Code, res.Body.String())
		}
	}

	// A pending booking holds the slot (BR-03) and the response says what is in the way.
	res := e.postBooking(other, room, at(tue, "09:00"), at(tue, "11:00"), nil)
	if kinds := conflictKinds(res); !slices.Equal(kinds, []string{"BOOKING:MENUNGGU", "BOOKING:MENUNGGU"}) {
		t.Errorf("conflict details = %v; want both pending bookings", kinds)
	}
	d := res.json()["error"].(map[string]any)["details"].([]any)[0].(map[string]any)
	if d["start_at"] != at(tue, "08:00") || d["end_at"] != at(tue, "10:00") {
		t.Errorf("first conflict = %v - %v; want 08:00-10:00 WIB", d["start_at"], d["end_at"])
	}
	if strings.Contains(res.Body.String(), "Latihan") {
		t.Error("conflict details leak another student's purpose")
	}
}

func TestBookingConflictsWithScheduleAndApprovedBooking(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	tok := e.login(mahasiswaA).accessToken
	e.insertSchedule(room, "Basis Data", at(tue, "08:00"), at(tue, "10:00"))
	e.insertBooking(room, mahasiswaB, "DISETUJUI", "Rapat", at(tue, "13:00"), at(tue, "15:00"))

	res := e.postBooking(tok, room, at(tue, "09:00"), at(tue, "11:00"), nil)
	if res.Code != http.StatusConflict || !slices.Equal(conflictKinds(res), []string{"SCHEDULE"}) {
		t.Errorf("against schedule: %d %v", res.Code, conflictKinds(res))
	}
	res = e.postBooking(tok, room, at(tue, "14:00"), at(tue, "16:00"), nil)
	if res.Code != http.StatusConflict || !slices.Equal(conflictKinds(res), []string{"BOOKING:DISETUJUI"}) {
		t.Errorf("against approved booking: %d %v", res.Code, conflictKinds(res))
	}
	mustStatus(t, e.postBooking(tok, room, at(tue, "10:00"), at(tue, "13:00"), nil), http.StatusCreated)
}

// BR-03: the slot is held while pending and released again by rejection or cancellation.
func TestSlotIsReleasedByRejectionAndCancellation(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	student := e.login(mahasiswaA).accessToken
	other := e.login(mahasiswaB).accessToken

	first := e.postBooking(student, room, at(tue, "08:00"), at(tue, "10:00"), nil)
	mustStatus(t, first, http.StatusCreated)
	if res := e.postBooking(other, room, at(tue, "08:00"), at(tue, "10:00"), nil); res.Code != http.StatusConflict {
		t.Fatalf("pending booking should hold the slot, got %d", res.Code)
	}

	mustStatus(t, e.action(e.tuToken(), idOf(first), "reject", map[string]string{"reason": "Dipakai ujian"}), http.StatusOK)
	second := e.postBooking(other, room, at(tue, "08:00"), at(tue, "10:00"), nil)
	mustStatus(t, second, http.StatusCreated)

	mustStatus(t, e.action(other, idOf(second), "cancel", nil), http.StatusOK)
	mustStatus(t, e.postBooking(student, room, at(tue, "08:00"), at(tue, "10:00"), nil), http.StatusCreated)
}

func TestCreateBookingRoomRules(t *testing.T) {
	e := newEnv(t)
	tok := e.login(mahasiswaA).accessToken

	inactive := e.freshRoom()
	if _, err := e.pool.Exec(t.Context(), "UPDATE rooms SET is_active = false WHERE id = $1", inactive); err != nil {
		t.Fatal(err)
	}
	if res := e.postBooking(tok, inactive, at(tue, "08:00"), at(tue, "09:00"), nil); res.Code != http.StatusConflict || res.errorCode() != "ROOM_INACTIVE" {
		t.Errorf("inactive room: %d %s; want 409 ROOM_INACTIVE", res.Code, res.errorCode())
	}

	deleted := e.freshRoom()
	if _, err := e.pool.Exec(t.Context(), "UPDATE rooms SET deleted_at = now(), is_active = false WHERE id = $1", deleted); err != nil {
		t.Fatal(err)
	}
	if res := e.postBooking(tok, deleted, at(tue, "08:00"), at(tue, "09:00"), nil); res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)["room_id"] {
		t.Errorf("deleted room: %d %s; want 422 naming room_id", res.Code, res.Body.String())
	}
}

func TestBookingIdempotency(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	tok := e.login(mahasiswaA).accessToken
	key := func(k string) reqOpt { return func(r *http.Request) { r.Header.Set("Idempotency-Key", k) } }

	first := e.postBooking(tok, room, at(tue, "08:00"), at(tue, "10:00"), nil, key("abc-1"))
	mustStatus(t, first, http.StatusCreated)
	if first.Header().Get("Idempotency-Replayed") != "" {
		t.Error("the first request is not a replay")
	}

	// A retry after the slot is taken (by itself) must return the original, not a conflict.
	again := e.postBooking(tok, room, at(tue, "08:00"), at(tue, "10:00"), nil, key("abc-1"))
	mustStatus(t, again, http.StatusCreated)
	if idOf(again) != idOf(first) || again.Header().Get("Idempotency-Replayed") != "true" {
		t.Errorf("retry returned %s (replayed=%q); want original %s", idOf(again), again.Header().Get("Idempotency-Replayed"), idOf(first))
	}
	if n := e.countRows("SELECT count(*) FROM bookings"); n != 1 {
		t.Errorf("bookings = %d; want 1", n)
	}
	if n := e.eventCount(idOf(first)); n != 1 {
		t.Errorf("events = %d; want 1", n)
	}
	if n := e.countRows("SELECT count(*) FROM notifications WHERE type = 'BOOKING_CREATED'"); n != 1 {
		t.Errorf("notifications = %d; want 1 (a replay must not notify again)", n)
	}

	// The same key with a different request is a client bug.
	if res := e.postBooking(tok, room, at(tue, "11:00"), at(tue, "12:00"), nil, key("abc-1")); res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)["Idempotency-Key"] {
		t.Errorf("reused key: %d %s; want 422 naming Idempotency-Key", res.Code, res.Body.String())
	}
	// Keys are scoped to the requester.
	other := e.login(mahasiswaB).accessToken
	if res := e.postBooking(other, room, at(tue, "11:00"), at(tue, "12:00"), nil, key("abc-1")); res.Code != http.StatusCreated {
		t.Errorf("another student's identical key: %d; want 201", res.Code)
	}
	if res := e.postBooking(tok, room, at(tue, "13:00"), at(tue, "14:00"), nil, key(strings.Repeat("k", 129))); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("oversized key: %d; want 422", res.Code)
	}

	// Concurrent retries of one request all get the same booking.
	room2 := e.freshRoom()
	results := make([]response, 6)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = e.postBooking(tok, room2, at(tue, "08:00"), at(tue, "10:00"), nil, key("race-1"))
		}()
	}
	wg.Wait()
	for i, res := range results {
		if res.Code != http.StatusCreated || idOf(res) != idOf(results[0]) {
			t.Errorf("concurrent retry %d: %d %s", i, res.Code, res.Body.String())
		}
	}
	if n := e.countRows("SELECT count(*) FROM bookings WHERE room_id = $1", room2); n != 1 {
		t.Errorf("concurrent retries created %d bookings; want 1", n)
	}
}

// The mandatory concurrency test (agent.md section 7.1): 20 requests for one slot, exactly one wins.
func TestConcurrentBookingsForSameSlot(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	students := []string{mahasiswaA, mahasiswaB, "2021101003", "2021101004", "2021101005"}
	tokens := make([]string, len(students))
	for i, s := range students {
		tokens[i] = e.login(s).accessToken
	}

	const n = 20
	results := make([]response, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = e.postBooking(tokens[i%len(tokens)], room, at(tue, "08:00"), at(tue, "10:00"), nil)
		}()
	}
	close(start)
	wg.Wait()

	created, conflicts := 0, 0
	for _, res := range results {
		switch {
		case res.Code == http.StatusCreated:
			created++
		case res.Code == http.StatusConflict && res.errorCode() == "BOOKING_CONFLICT":
			conflicts++
		default:
			t.Errorf("unexpected response %d: %s", res.Code, res.Body.String())
		}
	}
	if created != 1 || conflicts != n-1 {
		t.Errorf("%d created, %d conflicts; want 1 and %d", created, conflicts, n-1)
	}
	if rows := e.countRows("SELECT count(*) FROM bookings WHERE room_id = $1", room); rows != 1 {
		t.Errorf("bookings stored = %d; want 1", rows)
	}
	if events := e.countRows("SELECT count(*) FROM booking_events"); events != 1 {
		t.Errorf("events stored = %d; want 1", events)
	}
	if notes := e.countRows("SELECT count(*) FROM notifications WHERE type = 'BOOKING_CREATED'"); notes != 1 {
		t.Errorf("notifications = %d; want 1 (losers must not notify)", notes)
	}
}

// Two pending requests can never both be approved: the loser is already refused at creation, and
// approving after something else took the slot is refused too.
func TestConcurrentApprovalsOfCompetingRequests(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	tu := e.tuToken()

	first := e.postBooking(e.login(mahasiswaA).accessToken, room, at(tue, "08:00"), at(tue, "10:00"), nil)
	mustStatus(t, first, http.StatusCreated)

	// Free the slot, let a second request in, then try to revive the first by approving it.
	mustStatus(t, e.action(e.login(mahasiswaA).accessToken, idOf(first), "cancel", nil), http.StatusOK)
	second := e.postBooking(e.login(mahasiswaB).accessToken, room, at(tue, "09:00"), at(tue, "11:00"), nil)
	mustStatus(t, second, http.StatusCreated)

	if res := e.action(tu, idOf(first), "approve", nil); res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
		t.Errorf("approving a cancelled booking: %d %s; want 409 INVALID_STATE", res.Code, res.errorCode())
	}

	// Many parallel approvals of the same booking: one wins, the rest see it already decided.
	results := make([]response, 8)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = e.action(tu, idOf(second), "approve", nil)
		}()
	}
	wg.Wait()
	ok := 0
	for _, res := range results {
		switch {
		case res.Code == http.StatusOK:
			ok++
		case res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE":
			t.Errorf("parallel approve: %d %s", res.Code, res.Body.String())
		}
	}
	if ok != 1 {
		t.Errorf("%d approvals succeeded; want 1", ok)
	}
	if n := e.eventCount(idOf(second)); n != 2 {
		t.Errorf("events = %d; want 2 (created, approved)", n)
	}
	if n := e.notificationCount(e.userIDOf(mahasiswaB), "BOOKING_APPROVED"); n != 1 {
		t.Errorf("approval notifications = %d; want 1", n)
	}
}

// Every transition of brief 4.1, valid and invalid, for the owner and for TU/Kalab.
func TestBookingStateMachine(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	owner := e.login(mahasiswaA).accessToken

	type step struct {
		name    string
		token   string
		action  string
		body    any
		allowed []string // the statuses this action may start from
		result  string
	}
	steps := []step{
		{"approve", tu, "approve", nil, []string{"MENUNGGU"}, "DISETUJUI"},
		{"reject", tu, "reject", map[string]string{"reason": "Dipakai ujian"}, []string{"MENUNGGU"}, "DITOLAK"},
		{"owner cancels (before the start)", owner, "cancel", nil, []string{"MENUNGGU", "DISETUJUI"}, "DIBATALKAN"},
		{"TU cancels", tu, "cancel", map[string]string{"reason": "Ruang diperbaiki"}, []string{"DISETUJUI"}, "DIBATALKAN"},
	}

	for _, s := range steps {
		for _, from := range []string{"MENUNGGU", "DISETUJUI", "DITOLAK", "DIBATALKAN"} {
			id := e.bookingIn(from, mahasiswaA)
			events := e.eventCount(id)
			res := e.action(s.token, id, s.action, s.body)

			if slices.Contains(s.allowed, from) {
				mustStatus(t, res, http.StatusOK)
				if statusOf(res) != s.result || e.bookingStatus(id) != s.result {
					t.Errorf("%s from %s: status = %s/%s; want %s", s.name, from, statusOf(res), e.bookingStatus(id), s.result)
				}
				if n := e.eventCount(id); n != events+1 {
					t.Errorf("%s from %s: events %d -> %d; want exactly one more", s.name, from, events, n)
				}
				continue
			}
			if res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
				t.Errorf("%s from %s: %d %s; want 409 INVALID_STATE", s.name, from, res.Code, res.Body.String())
			}
			if e.bookingStatus(id) != from || e.eventCount(id) != events {
				t.Errorf("%s from %s: a refused transition changed state (status %s, events %d->%d)", s.name, from, e.bookingStatus(id), events, e.eventCount(id))
			}
		}
	}
}

func TestDecisionFieldsAndReasons(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	tuUserID := e.userIDOf(tuID)

	approved := e.bookingIn("MENUNGGU", mahasiswaA)
	res := e.action(tu, approved, "approve", nil)
	mustStatus(t, res, http.StatusOK)
	if res.json()["decided_by"] != tuUserID || res.json()["decided_at"] != "2026-10-14T10:00:00+07:00" || res.json()["decision_reason"] != nil {
		t.Errorf("approval fields: %v", res.json())
	}

	rejected := e.bookingIn("MENUNGGU", mahasiswaA)
	for _, bad := range []any{nil, map[string]string{}, map[string]string{"reason": "    "}, map[string]string{"reason": "abcd"}, map[string]string{"reason": strings.Repeat("x", 501)}} {
		if res := e.action(tu, rejected, "reject", bad); res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)["reason"] {
			t.Errorf("reject with %v: %d %s; want 422 naming reason", bad, res.Code, res.Body.String())
		}
	}
	if e.bookingStatus(rejected) != "MENUNGGU" {
		t.Fatal("a rejected request for a bad reason changed the booking")
	}
	res = e.action(tu, rejected, "reject", map[string]string{"reason": "  Dipakai ujian  "})
	mustStatus(t, res, http.StatusOK)
	if res.json()["decision_reason"] != "Dipakai ujian" {
		t.Errorf("reason = %v; want trimmed", res.json()["decision_reason"])
	}
	if res := e.action(tu, e.bookingIn("MENUNGGU", mahasiswaA), "reject", map[string]string{"reason": "abcde"}); res.Code != http.StatusOK {
		t.Errorf("a 5-character reason is the minimum and must pass: %d", res.Code)
	}

	// Cancelling an approved booking needs a reason; an owner cancelling their own pending one does not.
	app2 := e.bookingIn("DISETUJUI", mahasiswaA)
	for _, bad := range []any{nil, map[string]string{"reason": "oops"}} {
		if res := e.action(tu, app2, "cancel", bad); res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)["reason"] {
			t.Errorf("TU cancel with %v: %d; want 422 naming reason", bad, res.Code)
		}
	}
	mustStatus(t, e.action(tu, app2, "cancel", map[string]string{"reason": "Ruang diperbaiki"}), http.StatusOK)

	owner := e.login(mahasiswaA).accessToken
	pending := e.bookingIn("MENUNGGU", mahasiswaA)
	res = e.action(owner, pending, "cancel", map[string]string{"reason": "Tidak jadi"})
	mustStatus(t, res, http.StatusOK)
	if res.json()["decision_reason"] != "Tidak jadi" || res.json()["decided_by"] != e.userIDOf(mahasiswaA) {
		t.Errorf("owner cancel fields: %v", res.json())
	}
}

func TestBookingAuthorization(t *testing.T) {
	e := newEnv(t)
	id := e.bookingIn("MENUNGGU", mahasiswaA)
	owner := e.login(mahasiswaA).accessToken
	stranger := e.login(mahasiswaB).accessToken
	ob := e.login(obID).accessToken
	admin := e.adminToken()
	tu := e.tuToken()
	kalab := e.kalabToken() // approves LAB, not the KELAS room used here

	// Another student's booking is indistinguishable from a missing one.
	missing := "00000000-0000-4000-8000-0000000000aa"
	for _, p := range []string{"", "/events"} {
		if res := e.do("GET", "/api/v1/bookings/"+id+p, nil, bearer(stranger)); res.Code != http.StatusNotFound {
			t.Errorf("stranger GET %s: %d; want 404", p, res.Code)
		}
		if res := e.do("GET", "/api/v1/bookings/"+missing+p, nil, bearer(owner)); res.Code != http.StatusNotFound {
			t.Errorf("missing GET %s: %d; want 404", p, res.Code)
		}
		if res := e.do("GET", "/api/v1/bookings/"+id+p, nil, bearer(ob)); res.Code != http.StatusForbidden {
			t.Errorf("OB GET %s: %d; want 403", p, res.Code)
		}
		for name, tok := range map[string]string{"owner": owner, "TU": tu, "ADMIN": admin} {
			if res := e.do("GET", "/api/v1/bookings/"+id+p, nil, bearer(tok)); res.Code != http.StatusOK {
				t.Errorf("%s GET %s: %d; want 200", name, p, res.Code)
			}
		}
		// A KALAB does not approve KELAS rooms, so this booking is outside their reach (BR-10): 404, not 403.
		if res := e.do("GET", "/api/v1/bookings/"+id+p, nil, bearer(kalab)); res.Code != http.StatusNotFound {
			t.Errorf("KALAB GET %s of a KELAS booking: %d; want 404", p, res.Code)
		}
	}
	if res := e.do("GET", "/api/v1/bookings/not-a-uuid", nil, bearer(owner)); res.Code != http.StatusNotFound {
		t.Errorf("malformed id: %d; want 404", res.Code)
	}

	if res := e.action(stranger, id, "cancel", nil); res.Code != http.StatusNotFound {
		t.Errorf("stranger cancel: %d; want 404", res.Code)
	}
	if e.bookingStatus(id) != "MENUNGGU" {
		t.Fatal("a stranger changed the booking")
	}
	for name, tok := range map[string]string{"student": owner, "ADMIN": admin, "KALAB": kalab, "OB": ob} {
		for _, a := range []string{"approve", "reject"} {
			if res := e.action(tok, id, a, map[string]string{"reason": "alasan"}); res.Code != http.StatusForbidden {
				t.Errorf("%s %s: %d; want 403", name, a, res.Code)
			}
		}
	}
	// Managers must give a reason, so one is sent: the 403 comes from the room type, not from validation.
	for name, tok := range map[string]string{"ADMIN": admin, "KALAB": kalab, "OB": ob} {
		if res := e.action(tok, id, "cancel", map[string]string{"reason": "Ruang diperbaiki"}); res.Code != http.StatusForbidden {
			t.Errorf("%s cancel: %d; want 403", name, res.Code)
		}
	}
	for name, tok := range map[string]string{"TU": tu, "KALAB": kalab, "ADMIN": admin, "OB": ob} {
		if res := e.postBooking(tok, e.freshRoom(), at(tue, "08:00"), at(tue, "09:00"), nil); res.Code != http.StatusForbidden {
			t.Errorf("%s create: %d; want 403", name, res.Code)
		}
	}
	if e.bookingStatus(id) != "MENUNGGU" {
		t.Error("a forbidden action changed the booking")
	}
}

func TestListBookings(t *testing.T) {
	e := newEnv(t)
	roomA, roomB := e.freshRoom(), e.freshRoom()
	a, b := e.login(mahasiswaA).accessToken, e.login(mahasiswaB).accessToken
	tu, admin := e.tuToken(), e.login(adminID).accessToken

	a1 := idOf(e.postBooking(a, roomA, at(tue, "08:00"), at(tue, "09:00"), nil))
	// The clock is frozen, so step it to give each booking a distinct created_at to sort by.
	e.clock.Add(time.Minute)
	a2 := idOf(e.postBooking(a, roomB, at(wed, "08:00"), at(wed, "09:00"), nil))
	e.clock.Add(time.Minute)
	b1 := idOf(e.postBooking(b, roomA, at(tue, "10:00"), at(tue, "11:00"), nil))
	mustStatus(t, e.action(tu, b1, "approve", nil), http.StatusOK)

	list := func(tok, query string) ([]string, float64, response) {
		res := e.do("GET", "/api/v1/bookings"+query, nil, bearer(tok))
		mustStatus(t, res, http.StatusOK)
		var ids []string
		for _, it := range dataOf(res) {
			ids = append(ids, it.(map[string]any)["id"].(string))
		}
		total, _ := res.json()["meta"].(map[string]any)["total"].(float64)
		return ids, total, res
	}
	sameSet := func(name string, got []string, want ...string) {
		t.Helper()
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s = %v; want %v", name, got, want)
		}
	}

	got, _, _ := list(a, "")
	sameSet("student A", got, a1, a2)
	got, _, _ = list(a, "?mine=false")
	sameSet("student A mine=false is still only their own", got, a1, a2)
	got, total, _ := list(tu, "")
	sameSet("TU", got, a1, a2, b1)
	if total != 3 {
		t.Errorf("total = %v; want 3", total)
	}
	got, _, _ = list(admin, "")
	sameSet("ADMIN", got, a1, a2, b1)
	got, _, _ = list(tu, "?status=DISETUJUI")
	sameSet("status filter", got, b1)
	got, _, _ = list(tu, "?room_id="+roomB)
	sameSet("room filter", got, a2)
	got, _, _ = list(tu, "?mine=true")
	sameSet("TU mine (has none)", got)
	got, _, _ = list(a, "?status=DISETUJUI")
	sameSet("student filter cannot reach others", got)

	// from/to select bookings overlapping the window; the unescaped "+" of the offset is accepted.
	got, _, _ = list(tu, "?from="+at(wed, "00:00")+"&to="+at("2026-10-22", "00:00"))
	sameSet("wednesday window", got, a2)
	got, _, _ = list(tu, "?from="+at(tue, "09:00")+"&to="+at(tue, "10:00"))
	sameSet("window between back-to-back bookings", got)
	got, _, _ = list(tu, "?from="+at(tue, "08:30"))
	sameSet("from only", got, a1, a2, b1)

	// Newest first, and pagination reports the full total.
	got, total, res := list(tu, "?per_page=2&page=1")
	if len(got) != 2 || total != 3 || got[0] != b1 {
		t.Errorf("page 1 = %v total %v; want 2 items, newest (%s) first, total 3", got, total, b1)
	}
	if m := res.json()["meta"].(map[string]any); m["page"] != float64(1) || m["per_page"] != float64(2) {
		t.Errorf("meta = %v", m)
	}
	got, _, _ = list(tu, "?per_page=2&page=2")
	if len(got) != 1 {
		t.Errorf("page 2 = %v; want 1 item", got)
	}

	for name, q := range map[string]string{
		"bad status": "?status=DONE", "bad room": "?room_id=x", "bad mine": "?mine=maybe",
		"bad from": "?from=kemarin", "reversed window": "?from=" + at(wed, "00:00") + "&to=" + at(tue, "00:00"),
		"bad page": "?page=0", "huge page": "?per_page=101",
	} {
		if res := e.do("GET", "/api/v1/bookings"+q, nil, bearer(tu)); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d; want 422", name, res.Code)
		}
	}
	if res := e.do("GET", "/api/v1/bookings", nil, bearer(e.login(obID).accessToken)); res.Code != http.StatusForbidden {
		t.Errorf("OB list: %d; want 403", res.Code)
	}
}

func TestBookingEventsTrail(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	owner := e.login(mahasiswaA)
	tuUserID := e.userIDOf(tuID)
	tu := e.tuToken()

	id := idOf(e.postBooking(owner.accessToken, room, at(tue, "08:00"), at(tue, "10:00"), nil))
	mustStatus(t, e.action(tu, id, "approve", nil), http.StatusOK)
	e.clock.Add(time.Minute)
	mustStatus(t, e.action(tu, id, "cancel", map[string]string{"reason": "Ruang diperbaiki"}), http.StatusOK)

	res := e.do("GET", "/api/v1/bookings/"+id+"/events", nil, bearer(owner.accessToken))
	mustStatus(t, res, http.StatusOK)
	events := dataOf(res)
	type ev struct{ from, to, actor, reason string }
	want := []ev{
		{"", "MENUNGGU", owner.userID, ""},
		{"MENUNGGU", "DISETUJUI", tuUserID, ""},
		{"DISETUJUI", "DIBATALKAN", tuUserID, "Ruang diperbaiki"},
	}
	if len(events) != len(want) {
		t.Fatalf("events = %d; want %d: %s", len(events), len(want), res.Body.String())
	}
	for i, w := range want {
		m := events[i].(map[string]any)
		got := ev{}
		got.from, _ = m["from_status"].(string)
		got.to, _ = m["to_status"].(string)
		got.actor, _ = m["actor_id"].(string)
		got.reason, _ = m["reason"].(string)
		if got != w || m["booking_id"] != id {
			t.Errorf("event %d = %+v; want %+v", i, got, w)
		}
	}
}

// BR-06: the audit trail cannot be rewritten, even by someone with database access through the app role.
func TestBookingEventsAreAppendOnly(t *testing.T) {
	e := newEnv(t)
	id := e.bookingIn("DISETUJUI", mahasiswaA)
	before := e.eventCount(id)

	for name, stmt := range map[string]string{
		"update reason":   "UPDATE booking_events SET reason = 'diubah'",
		"update status":   "UPDATE booking_events SET to_status = 'DITOLAK'",
		"delete one":      "DELETE FROM booking_events WHERE booking_id = '" + id + "'",
		"delete all":      "DELETE FROM booking_events",
		"update by actor": "UPDATE booking_events SET actor_id = actor_id",
	} {
		_, err := e.pool.Exec(t.Context(), stmt)
		if err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s: err = %v; want the append-only trigger to refuse", name, err)
		}
	}
	if n := e.eventCount(id); n != before {
		t.Errorf("events %d -> %d after tampering attempts", before, n)
	}
}

// The exclusion constraint is the last line of defence when the application check is bypassed.
func TestBookingsNoOverlapConstraint(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	insert := func(status, start, end string) error {
		_, err := e.pool.Exec(t.Context(),
			`INSERT INTO bookings (room_id, requester_id, start_at, end_at, purpose, status, approval_mode, manual_reason)
			 VALUES ($1, $2, $3, $4, 'x', $5, 'MANUAL', 'ROOM_POLICY')`,
			room, e.userIDOf(mahasiswaA), start, end, status)
		return err
	}
	violation := func(err error) bool { return err != nil && strings.Contains(err.Error(), "bookings_no_overlap") }

	if err := insert("MENUNGGU", at(tue, "08:00"), at(tue, "10:00")); err != nil {
		t.Fatal(err)
	}
	if err := insert("DISETUJUI", at(tue, "09:00"), at(tue, "11:00")); !violation(err) {
		t.Errorf("overlapping pending/approved: err = %v; want bookings_no_overlap", err)
	}
	if err := insert("MENUNGGU", at(tue, "10:00"), at(tue, "12:00")); err != nil {
		t.Errorf("back-to-back booking refused: %v", err)
	}
	for _, status := range []string{"DITOLAK", "DIBATALKAN"} {
		if err := insert(status, at(tue, "08:00"), at(tue, "10:00")); err != nil {
			t.Errorf("a %s booking must not hold the slot: %v", status, err)
		}
	}
	// Reviving a released booking into a taken slot is refused by the same constraint.
	if _, err := e.pool.Exec(t.Context(), `UPDATE bookings SET status = 'MENUNGGU' WHERE status = 'DITOLAK' AND room_id = $1`, room); !violation(err) {
		t.Errorf("reviving into a taken slot: err = %v; want bookings_no_overlap", err)
	}
}

func TestApproveRechecksTheSlot(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	id := idOf(e.postBooking(e.login(mahasiswaA).accessToken, room, at(tue, "08:00"), at(tue, "10:00"), nil))

	// A schedule that sits on the request anyway, as data from before the guard trigger of migration 000009 can.
	e.insertScheduleUnguarded(room, "Kuliah Pengganti", at(tue, "09:00"), at(tue, "11:00"))

	res := e.action(e.tuToken(), id, "approve", nil)
	if res.Code != http.StatusConflict || res.errorCode() != "BOOKING_CONFLICT" || !slices.Equal(conflictKinds(res), []string{"SCHEDULE"}) {
		t.Errorf("approve: %d %s; want 409 BOOKING_CONFLICT naming the schedule", res.Code, res.Body.String())
	}
	if e.bookingStatus(id) != "MENUNGGU" || e.eventCount(id) != 1 {
		t.Error("a refused approval changed the booking")
	}
	// The request can still be rejected.
	mustStatus(t, e.action(e.tuToken(), id, "reject", map[string]string{"reason": "Bentrok kuliah"}), http.StatusOK)
}

func TestApproveRefusalsOnRoomAndTime(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()

	// Room deactivated while the request was pending.
	room := e.freshRoom()
	id := idOf(e.postBooking(e.login(mahasiswaA).accessToken, room, at(tue, "08:00"), at(tue, "10:00"), nil))
	if _, err := e.pool.Exec(t.Context(), "UPDATE rooms SET is_active = false WHERE id = $1", room); err != nil {
		t.Fatal(err)
	}
	if res := e.action(tu, id, "approve", nil); res.Code != http.StatusConflict || res.errorCode() != "ROOM_INACTIVE" {
		t.Errorf("inactive room: %d %s; want 409 ROOM_INACTIVE", res.Code, res.errorCode())
	}

	// A request whose time has already passed.
	late := e.bookingIn("MENUNGGU", mahasiswaB)
	e.clock.Set(time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)) // 19:00 WIB on the day, after the 10:00 end
	tu = e.tuToken()                                            // the earlier token expired with the clock jump
	if res := e.action(tu, late, "approve", nil); res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
		t.Errorf("past booking: %d %s; want 409 INVALID_STATE", res.Code, res.errorCode())
	}
	// Past expires_at (BR-22) a decision of either kind is refused, and so is the owner cancelling a booking that has started.
	if res := e.action(tu, late, "reject", map[string]string{"reason": "Sudah lewat"}); res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
		t.Errorf("rejecting an expired booking: %d %s; want 409 INVALID_STATE", res.Code, res.errorCode())
	}
	if res := e.action(e.login(mahasiswaB).accessToken, late, "cancel", nil); res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
		t.Errorf("owner cancelling after the start: %d %s; want 409 INVALID_STATE", res.Code, res.errorCode())
	}
}

func TestBookingNotifications(t *testing.T) {
	e := newEnv(t)
	admin := e.login(adminID)

	// A second approver, plus one that has been deactivated.
	second := e.newUserBody("198505052010012004")
	second["role"] = "TU"
	res := e.do("POST", "/api/v1/users", second, bearer(admin.accessToken))
	mustStatus(t, res, http.StatusCreated)
	secondID := res.json()["id"].(string)
	gone := e.newUserBody("198505052010012005")
	gone["role"] = "TU"
	res = e.do("POST", "/api/v1/users", gone, bearer(admin.accessToken))
	mustStatus(t, res, http.StatusCreated)
	goneID := res.json()["id"].(string)
	mustStatus(t, e.do("PATCH", "/api/v1/users/"+goneID, map[string]any{"is_active": false}, bearer(admin.accessToken)), http.StatusOK)

	tuUserID, studentID := e.userIDOf(tuID), e.userIDOf(mahasiswaA)
	student := e.login(mahasiswaA).accessToken
	tu := e.tuToken()

	id := idOf(e.postBooking(student, e.freshRoom(), at(tue, "08:00"), at(tue, "10:00"), nil))
	for _, who := range []string{tuUserID, secondID} {
		if n := e.notificationCount(who, "BOOKING_CREATED"); n != 1 {
			t.Errorf("approver %s got %d BOOKING_CREATED; want 1", who, n)
		}
	}
	for name, who := range map[string]string{"deactivated TU": goneID, "ADMIN": admin.userID, "KALAB": e.userIDOf(kalabID), "requester": studentID, "OB": e.userIDOf(obID)} {
		if n := e.notificationCount(who, "BOOKING_CREATED"); n != 0 {
			t.Errorf("%s got %d BOOKING_CREATED; want 0", name, n)
		}
	}
	var refType, refID, title, body string
	if err := e.pool.QueryRow(t.Context(), "SELECT ref_type, ref_id::text, title, body FROM notifications WHERE user_id = $1", tuUserID).Scan(&refType, &refID, &title, &body); err != nil {
		t.Fatal(err)
	}
	if refType != "BOOKING" || refID != id || !strings.Contains(body, "20/10/2026 08:00-10:00 WIB") {
		t.Errorf("notification = %s %s %q %q", refType, refID, title, body)
	}

	// A LAB booking goes to the KALAB (the approver of that type) and to nobody else.
	lab := e.insertRoom("BKN-LAB", "LAB", 30)
	labBooking := idOf(e.postBooking(student, lab, at(tue, "08:00"), at(tue, "10:00"), nil))
	kalabUserID := e.userIDOf(kalabID)
	for name, who := range map[string]string{"TU": tuUserID, "second TU": secondID, "ADMIN": admin.userID, "OB": e.userIDOf(obID)} {
		if n := e.countRows("SELECT count(*) FROM notifications WHERE user_id = $1 AND ref_id = $2", who, labBooking); n != 0 {
			t.Errorf("%s was notified of a LAB booking they do not approve (%d)", name, n)
		}
	}
	if n := e.countRows("SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'BOOKING_CREATED' AND ref_id = $2", kalabUserID, labBooking); n != 1 {
		t.Errorf("KALAB got %d BOOKING_CREATED for the LAB booking; want 1", n)
	}

	mustStatus(t, e.action(tu, id, "approve", nil), http.StatusOK)
	if n := e.notificationCount(studentID, "BOOKING_APPROVED"); n != 1 {
		t.Errorf("BOOKING_APPROVED = %d; want 1", n)
	}
	mustStatus(t, e.action(tu, id, "cancel", map[string]string{"reason": "Ruang diperbaiki"}), http.StatusOK)
	if n := e.notificationCount(studentID, "BOOKING_CANCELLED"); n != 1 {
		t.Errorf("BOOKING_CANCELLED = %d; want 1", n)
	}

	rejected := idOf(e.postBooking(student, e.freshRoom(), at(tue, "08:00"), at(tue, "10:00"), nil))
	mustStatus(t, e.action(tu, rejected, "reject", map[string]string{"reason": "Dipakai ujian"}), http.StatusOK)
	if err := e.pool.QueryRow(t.Context(), "SELECT body FROM notifications WHERE user_id = $1 AND type = 'BOOKING_REJECTED'", studentID).Scan(&body); err != nil || !strings.Contains(body, "Dipakai ujian") {
		t.Errorf("rejection notification = %q (%v); want it to carry the reason", body, err)
	}

	// Cancelling your own request tells nobody; the approvers already know about it.
	own := idOf(e.postBooking(student, e.freshRoom(), at(tue, "08:00"), at(tue, "10:00"), nil))
	before := e.countRows("SELECT count(*) FROM notifications")
	mustStatus(t, e.action(student, own, "cancel", nil), http.StatusOK)
	if after := e.countRows("SELECT count(*) FROM notifications"); after != before {
		t.Errorf("self-cancel created %d notifications; want none", after-before)
	}
}

// A status change, its audit event and its notification commit together or not at all.
func TestBookingWritesAreAtomic(t *testing.T) {
	e := newEnv(t)
	student := e.login(mahasiswaA).accessToken
	tu := e.tuToken()

	breakTable := func(table string) {
		t.Helper()
		sql := `CREATE FUNCTION fail_` + table + `() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$ LANGUAGE plpgsql;
			CREATE TRIGGER fail_` + table + ` BEFORE INSERT ON ` + table + ` FOR EACH ROW EXECUTE FUNCTION fail_` + table + `();`
		if _, err := e.pool.Exec(t.Context(), sql); err != nil {
			t.Fatal(err)
		}
	}
	repair := func(table string) {
		t.Helper()
		if _, err := e.pool.Exec(t.Context(), `DROP TRIGGER fail_`+table+` ON `+table+`; DROP FUNCTION fail_`+table+`()`); err != nil {
			t.Fatal(err)
		}
	}

	// Creating: a failing notification insert must leave no booking and no event behind.
	breakTable("notifications")
	res := e.doUnchecked("POST", "/api/v1/bookings", bookingBody(e.freshRoom(), at(tue, "08:00"), at(tue, "10:00"), nil), bearer(student))
	if res.Code != http.StatusInternalServerError || res.errorCode() != "INTERNAL_SERVER_ERROR" {
		t.Fatalf("create with failing notification: %d %s; want 500", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "injected") {
		t.Error("the 500 response leaks the database error")
	}
	if b, ev := e.countRows("SELECT count(*) FROM bookings"), e.countRows("SELECT count(*) FROM booking_events"); b != 0 || ev != 0 {
		t.Errorf("after a failed create: %d bookings, %d events; want 0, 0", b, ev)
	}
	repair("notifications")

	// Deciding: a failing notification insert must leave the status and the trail untouched.
	id := e.bookingIn("MENUNGGU", mahasiswaA)
	events, notes := e.eventCount(id), e.countRows("SELECT count(*) FROM notifications")
	breakTable("notifications")
	res = e.doUnchecked("POST", "/api/v1/bookings/"+id+"/approve", nil, bearer(tu))
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("approve with failing notification: %d; want 500", res.Code)
	}
	if e.bookingStatus(id) != "MENUNGGU" || e.eventCount(id) != events || e.countRows("SELECT count(*) FROM notifications") != notes {
		t.Errorf("a failed approval left traces: status %s, events %d->%d", e.bookingStatus(id), events, e.eventCount(id))
	}
	repair("notifications")

	// And the other way round: a failing event insert must leave no notification.
	breakTable("booking_events")
	res = e.doUnchecked("POST", "/api/v1/bookings/"+id+"/approve", nil, bearer(tu))
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("approve with failing event: %d; want 500", res.Code)
	}
	if e.bookingStatus(id) != "MENUNGGU" || e.countRows("SELECT count(*) FROM notifications") != notes {
		t.Error("a failed event insert left a status change or notification behind")
	}
	repair("booking_events")

	mustStatus(t, e.action(tu, id, "approve", nil), http.StatusOK)
}

func TestBookingEndpointsRejectUnauthenticated(t *testing.T) {
	e := newEnv(t)
	id := e.bookingIn("MENUNGGU", mahasiswaA)
	for _, c := range []struct{ method, path string }{
		{"POST", "/api/v1/bookings"}, {"GET", "/api/v1/bookings"}, {"GET", "/api/v1/bookings/" + id},
		{"GET", "/api/v1/bookings/" + id + "/events"}, {"POST", "/api/v1/bookings/" + id + "/approve"},
		{"POST", "/api/v1/bookings/" + id + "/reject"}, {"POST", "/api/v1/bookings/" + id + "/cancel"},
	} {
		if res := e.do(c.method, c.path, nil); res.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without a token: %d; want 401", c.method, c.path, res.Code)
		}
	}
}
