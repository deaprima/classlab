package app_test

import (
	"net/http"
	"strings"
	"testing"

	"classlab/api/internal/platform/password"
	"classlab/api/internal/seed"
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

// Tokens by role. Authority depends on the room type (BR-10): TU approves KELAS, KALAB approves LAB, and ADMIN
// manages rooms and schedules of every type but approves only the special rooms. approverToken picks the right one.
func (e *env) adminToken() string { e.t.Helper(); return e.login(adminID).accessToken }
func (e *env) tuToken() string    { e.t.Helper(); return e.login(tuID).accessToken }
func (e *env) kalabToken() string { e.t.Helper(); return e.login(kalabID).accessToken }
func (e *env) obToken() string    { e.t.Helper(); return e.login(obID).accessToken }
func (e *env) orgToken() string   { e.t.Helper(); return e.login(orgID).accessToken }

// approverToken returns the token of the role that approves the type of the room: TU for KELAS, KALAB for LAB.
func (e *env) approverToken(roomID string) string {
	e.t.Helper()
	switch typ := e.roomTypeOf(roomID); typ {
	case "KELAS":
		return e.tuToken()
	case "LAB":
		return e.kalabToken()
	default:
		return e.tuToken() // special rooms: TU and ADMIN both approve
	}
}

func (e *env) roomTypeOf(roomID string) string {
	e.t.Helper()
	var typ string
	if err := e.pool.QueryRow(e.t.Context(), "SELECT type::text FROM rooms WHERE id = $1", roomID).Scan(&typ); err != nil {
		e.t.Fatal(err)
	}
	return typ
}

// insertRoom creates a room straight in the database, bypassing the API and its rules.
func (e *env) insertRoom(code, typ string, capacity int) string {
	e.t.Helper()
	var id string
	err := e.pool.QueryRow(e.t.Context(),
		`INSERT INTO rooms (org_unit_id, code, name, type, capacity, qr_token) VALUES ($1, $2, $3, $4, $5, replace(gen_random_uuid()::text, '-', '')) RETURNING id`,
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
		roomID, title, start, end, e.userIDOf(tuID)).Scan(&id)
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
		`INSERT INTO bookings (room_id, requester_id, start_at, end_at, purpose, status, approval_mode, manual_reason)
		 VALUES ($1, $2, $3, $4, $5, $6, 'MANUAL', 'ROOM_POLICY') RETURNING id`,
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

// insertUser creates an account straight in the database with the dev password and
// must_change_password = false, so a test can log in as it without going through the first-login flow.
// Accounts made through POST /users start with a temporary password and are blocked until it is changed.
func (e *env) insertUser(identifier, name, role string) string {
	e.t.Helper()
	hasher := password.NewHasher(e.cfg.Argon2MemoryKiB, e.cfg.Argon2Iterations, e.cfg.Argon2Parallelism)
	hash, err := hasher.Hash(seed.DevPassword)
	if err != nil {
		e.t.Fatal(err)
	}
	var picName, picPhone *string
	if role == "ORGANISASI" { // users_org_pic_check
		n, p := "Penanggung Jawab", "081200000000"
		picName, picPhone = &n, &p
	}
	var id string
	err = e.pool.QueryRow(e.t.Context(),
		`INSERT INTO users (org_unit_id, identifier, name, email, password_hash, role, pic_name, pic_phone, must_change_password)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, false) RETURNING id`,
		e.orgUnitID(), identifier, name, strings.ToLower(identifier)+"@kampus.test", hash, role, picName, picPhone).Scan(&id)
	if err != nil {
		e.t.Fatalf("inserting user %s: %v", identifier, err)
	}
	return id
}
