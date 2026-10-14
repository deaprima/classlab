package app_test

import (
	"net/http"
	"strings"
	"testing"
)

// at builds an ISO 8601 timestamp in WIB, e.g. at("2026-10-20", "08:00").
func at(date, hm string) string { return date + "T" + hm + ":00+07:00" }

// All timestamps in these tests are in the future of the fixed test clock (2026-10-14 10:00 WIB).
// 2026-10-19 is a Monday.
const (
	mon = "2026-10-19"
	tue = "2026-10-20"
	wed = "2026-10-21"
)

func (e *env) adminToken() string {
	e.t.Helper()
	return e.login(tuKalabID).accessToken
}

// insertRoom creates a room straight in the database, bypassing the API and its rules.
func (e *env) insertRoom(code, typ string, capacity int) string {
	e.t.Helper()
	var id string
	err := e.pool.QueryRow(e.t.Context(),
		`INSERT INTO rooms (org_unit_id, code, name, type, capacity) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		e.orgUnitID(), code, "Ruang "+code, typ, capacity).Scan(&id)
	if err != nil {
		e.t.Fatalf("inserting room %s: %v", code, err)
	}
	return id
}

func (e *env) insertSchedule(roomID, title, start, end string) string {
	e.t.Helper()
	var id string
	err := e.pool.QueryRow(e.t.Context(),
		`INSERT INTO room_schedules (room_id, title, start_at, end_at, created_by)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		roomID, title, start, end, e.userIDOf(tuKalabID)).Scan(&id)
	if err != nil {
		e.t.Fatalf("inserting schedule %q: %v", title, err)
	}
	return id
}

// insertBooking writes a booking directly; the booking API arrives in phase 3.
func (e *env) insertBooking(roomID, requester, status, purpose, start, end string) string {
	e.t.Helper()
	var id string
	err := e.pool.QueryRow(e.t.Context(),
		`INSERT INTO bookings (room_id, requester_id, start_at, end_at, purpose, status)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		roomID, e.userIDOf(requester), start, end, purpose, status).Scan(&id)
	if err != nil {
		e.t.Fatalf("inserting booking %q: %v", purpose, err)
	}
	return id
}

func (e *env) countRows(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(e.t.Context(), query, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func (e *env) postSchedule(token, roomID, title, start, end string, extra map[string]any) response {
	e.t.Helper()
	body := map[string]any{"room_id": roomID, "title": title, "start_at": start, "end_at": end}
	for k, v := range extra {
		body[k] = v
	}
	return e.do("POST", "/api/v1/schedules", body, bearer(token))
}

// conflictKinds returns "KIND:status" for each entry of error.details.
func conflictKinds(res response) []string {
	var out []string
	details, _ := res.json()["error"].(map[string]any)["details"].([]any)
	for _, d := range details {
		m := d.(map[string]any)
		kind, _ := m["kind"].(string)
		status, _ := m["status"].(string)
		out = append(out, strings.TrimSuffix(kind+":"+status, ":"))
	}
	return out
}

func mustStatus(t *testing.T, res response, want int) {
	t.Helper()
	if res.Code != want {
		t.Fatalf("status = %d; want %d. body: %s", res.Code, want, res.Body.String())
	}
}

func dataOf(res response) []any {
	d, _ := res.json()["data"].([]any)
	return d
}

var _ = http.StatusOK
