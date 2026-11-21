package app_test

import (
	"net/http"
	"strings"
	"testing"
)

func (e *env) roomJSON(token, id string) map[string]any {
	e.t.Helper()
	res := e.do("GET", "/api/v1/rooms/"+id, nil, bearer(token))
	mustStatus(e.t, res, http.StatusOK)
	return res.json()
}

func TestRoomsCanHaveTheirOwnOperatingHours(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	student := e.login(mahasiswaA).accessToken

	created := e.do("POST", "/api/v1/rooms", map[string]any{
		"org_unit_id": e.orgUnitID(), "code": "HR-1", "name": "Ruang Jam", "type": "KELAS", "capacity": 30,
		"open_time": "06:00", "close_time": "22:00",
	}, bearer(tu))
	mustStatus(t, created, http.StatusCreated)
	if created.json()["open_time"] != "06:00" || created.json()["close_time"] != "22:00" {
		t.Fatalf("created room hours = %v - %v", created.json()["open_time"], created.json()["close_time"])
	}
	id := created.json()["id"].(string)

	// A room may open longer than its type's default 07:00-21:00 (BR-29).
	mustStatus(t, e.postBooking(student, id, at(mon, "06:00"), at(mon, "08:00"), nil), http.StatusCreated)
	mustStatus(t, e.postBooking(student, id, at(mon, "20:00"), at(mon, "22:00"), nil), http.StatusCreated)

	// A room with no hours of its own follows the type; the API shows null.
	plain := e.insertRoom("HR-2", "KELAS", 30)
	if r := e.roomJSON(tu, plain); r["open_time"] != nil || r["close_time"] != nil {
		t.Errorf("room without own hours = %v - %v, want null", r["open_time"], r["close_time"])
	}
	res := e.postBooking(student, plain, at(mon, "06:00"), at(mon, "08:00"), nil)
	if res.Code != http.StatusUnprocessableEntity || res.errorCode() != "OUTSIDE_OPERATING_HOURS" {
		t.Fatalf("06:00 in a default-hours room: %d %s, want OUTSIDE_OPERATING_HOURS", res.Code, res.errorCode())
	}
	// The error names the window that applied, so the client can show it.
	if body := res.Body.String(); !strings.Contains(body, `"07:00"`) || !strings.Contains(body, `"21:00"`) {
		t.Errorf("details do not carry the 07:00-21:00 window: %s", body)
	}

	// Tighter hours beat the type's wider ones, and reach the details too.
	mustStatus(t, e.do("PATCH", "/api/v1/rooms/"+plain, map[string]any{"open_time": "09:00", "close_time": "17:00"}, bearer(tu)), http.StatusOK)
	res = e.postBooking(student, plain, at(mon, "08:00"), at(mon, "10:00"), nil)
	if res.Code != http.StatusUnprocessableEntity || res.errorCode() != "OUTSIDE_OPERATING_HOURS" || !strings.Contains(res.Body.String(), `"09:00"`) {
		t.Errorf("08:00 in a room that opens at 09:00: %d %s", res.Code, res.Body.String())
	}
	mustStatus(t, e.postBooking(student, plain, at(mon, "09:00"), at(mon, "17:00"), nil), http.StatusCreated)
}

func TestChangingRoomHoursNeverTouchesExistingBookings(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	room := e.insertRoom("HR-3", "KELAS", 30)
	booking := e.postBooking(e.login(mahasiswaA).accessToken, room, at(mon, "08:00"), at(mon, "10:00"), nil)
	mustStatus(t, booking, http.StatusCreated)

	// Closing the room for the morning does not cancel what was already asked for (BR-29).
	mustStatus(t, e.do("PATCH", "/api/v1/rooms/"+room, map[string]any{"open_time": "13:00", "close_time": "17:00"}, bearer(tu)), http.StatusOK)
	if got := e.bookingStatus(idOf(booking)); got != "MENUNGGU" {
		t.Errorf("booking became %s after the room's hours changed", got)
	}
}

func TestRoomHoursAreValidatedAndCanBeCleared(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	room := e.insertRoom("HR-4", "KELAS", 30)
	patch := func(body map[string]any) response {
		return e.do("PATCH", "/api/v1/rooms/"+room, body, bearer(tu))
	}

	cases := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"only an opening time on a room without hours", map[string]any{"open_time": "08:00"}, "open_time"},
		{"only a closing time on a room without hours", map[string]any{"close_time": "17:00"}, "open_time"},
		{"not HH:MM", map[string]any{"open_time": "8", "close_time": "17:00"}, "open_time"},
		{"close before open", map[string]any{"open_time": "17:00", "close_time": "08:00"}, "open_time"},
		{"off the 30 minute grid", map[string]any{"open_time": "08:10", "close_time": "17:00"}, "open_time"},
		{"clearing together with new times", map[string]any{"clear_operating_hours": true, "open_time": "08:00"}, "open_time"},
	}
	for _, c := range cases {
		res := patch(c.body)
		if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), `"field":"`+c.field+`"`) {
			t.Errorf("%s: %d %s, want 422 naming %s", c.name, res.Code, res.Body.String(), c.field)
		}
	}
	if r := e.roomJSON(tu, room); r["open_time"] != nil {
		t.Errorf("refused requests changed the hours: %v", r)
	}

	// Setting both works; then one end can be moved on its own because the other already exists.
	mustStatus(t, patch(map[string]any{"open_time": "08:00", "close_time": "17:00"}), http.StatusOK)
	mustStatus(t, patch(map[string]any{"close_time": "19:30"}), http.StatusOK)
	if r := e.roomJSON(tu, room); r["open_time"] != "08:00" || r["close_time"] != "19:30" {
		t.Errorf("hours after moving one end = %v - %v", r["open_time"], r["close_time"])
	}
	if res := patch(map[string]any{"open_time": "20:00"}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("opening after the existing close: %d, want 422", res.Code)
	}

	// Clearing puts the room back on its type's hours, and the change is audited with before and after.
	mustStatus(t, patch(map[string]any{"clear_operating_hours": true}), http.StatusOK)
	if r := e.roomJSON(tu, room); r["open_time"] != nil || r["close_time"] != nil {
		t.Errorf("hours after clearing = %v - %v", r["open_time"], r["close_time"])
	}
	// The frozen test clock gives every audit line the same time, so the line is found by what it says, not
	// by being the latest: before 08:00, after nothing.
	if n := e.countRows(`SELECT count(*) FROM audit_logs WHERE action = 'ROOM_UPDATED' AND entity_id = $1
		AND before->>'open_time' = '08:00' AND after->>'open_time' IS NULL`, room); n != 1 {
		t.Errorf("audit lines recording the hours being cleared = %d, want 1", n)
	}

	// Other staff may not touch the hours of a room outside their types.
	if res := e.do("PATCH", "/api/v1/rooms/"+room, map[string]any{"open_time": "08:00", "close_time": "17:00"}, bearer(e.kalabToken())); res.Code != http.StatusForbidden {
		t.Errorf("KALAB setting hours of a KELAS room: %d, want 403", res.Code)
	}
	mustStatus(t, e.do("PATCH", "/api/v1/rooms/"+room, map[string]any{"open_time": "08:00", "close_time": "17:00"}, bearer(e.adminToken())), http.StatusOK)
}

func TestAvailabilityAndTimelineShowTheEffectiveHoursAndRunningBookings(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	room := e.insertRoom("HR-5", "KELAS", 30)
	custom := e.insertRoom("HR-6", "KELAS", 30)
	mustStatus(t, e.do("PATCH", "/api/v1/rooms/"+custom, map[string]any{"open_time": "06:00", "close_time": "22:00"}, bearer(tu)), http.StatusOK)

	avail := e.do("GET", "/api/v1/availability?start_at="+at(mon, "08:00")+"&end_at="+at(mon, "10:00")+"&type=KELAS", nil, bearer(tu))
	mustStatus(t, avail, http.StatusOK)
	hours := map[string]map[string]any{}
	for _, item := range dataOf(avail) {
		m := item.(map[string]any)
		hours[m["room"].(map[string]any)["code"].(string)] = m["operating_hours"].(map[string]any)
	}
	if h := hours["HR-5"]; h["open_time"] != "07:00" || h["close_time"] != "21:00" || h["custom"] != false {
		t.Errorf("default room hours = %v", h)
	}
	if h := hours["HR-6"]; h["open_time"] != "06:00" || h["close_time"] != "22:00" || h["custom"] != true {
		t.Errorf("custom room hours = %v", h)
	}

	// A booking that is under way holds the room: DIGUNAKAN, listed as a BERLANGSUNG conflict, shown on
	// the timeline, and in the way of a new request.
	e.insertBooking(room, mahasiswaA, "BERLANGSUNG", "Sedang dipakai", at(mon, "08:00"), at(mon, "10:00"))
	avail = e.do("GET", "/api/v1/availability?start_at="+at(mon, "08:00")+"&end_at="+at(mon, "10:00")+"&type=KELAS", nil, bearer(tu))
	for _, item := range dataOf(avail) {
		m := item.(map[string]any)
		if m["room"].(map[string]any)["code"] != "HR-5" {
			continue
		}
		conflicts := m["conflicts"].([]any)
		if m["status"] != "DIGUNAKAN" || len(conflicts) != 1 || conflicts[0].(map[string]any)["status"] != "BERLANGSUNG" {
			t.Errorf("room with a running booking = %v", m)
		}
	}
	tl := e.do("GET", "/api/v1/rooms/"+room+"/schedule?from="+at(mon, "00:00")+"&to="+at(tue, "00:00"), nil, bearer(tu))
	mustStatus(t, tl, http.StatusOK)
	items := dataOf(tl)
	if len(items) != 1 || items[0].(map[string]any)["kind"] != "BOOKING" {
		t.Errorf("timeline = %v, want the running booking", items)
	}
	if h := tl.json()["operating_hours"].(map[string]any); h["open_time"] != "07:00" || h["custom"] != false {
		t.Errorf("timeline hours = %v", h)
	}
	if res := e.postBooking(e.login(mahasiswaB).accessToken, room, at(mon, "09:00"), at(mon, "11:00"), nil); res.Code != http.StatusConflict || res.errorCode() != "BOOKING_CONFLICT" {
		t.Errorf("booking over a running one: %d %s, want 409 BOOKING_CONFLICT", res.Code, res.errorCode())
	}
}
