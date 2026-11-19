package app_test

import (
	"net/http"
	"testing"
	"time"
)

// recordViolation records a violation of the student on a booking made for it.
func (e *env) recordViolation(student, kind string, points int, at time.Time) string {
	e.t.Helper()
	booking := e.insertBooking(e.freshRoom(), student, "TIDAK_HADIR", "Fixture", "2026-10-13T08:00:00+07:00", "2026-10-13T10:00:00+07:00")
	var id string
	if err := e.pool.QueryRow(e.t.Context(),
		`INSERT INTO violations (user_id, booking_id, kind, points, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		e.userIDOf(student), booking, kind, points, at).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func TestListViolations(t *testing.T) {
	e := newEnv(t)
	now := e.clock.Now()
	recent := e.recordViolation(mahasiswaA, "TIDAK_HADIR", 2, now.Add(-24*time.Hour))
	old := e.recordViolation(mahasiswaA, "TANPA_CHECKOUT", 1, now.Add(-45*24*time.Hour)) // outside the 30-day window
	e.recordViolation(mahasiswaB, "TIDAK_HADIR", 2, now)                                 // someone else's

	annulled := e.recordViolation(mahasiswaA, "BATAL_TERLAMBAT", 1, now.Add(-48*time.Hour))
	mustStatus(t, e.do("POST", "/api/v1/violations/"+annulled+"/annul", map[string]string{"reason": "Dibatalkan pengelola"}, bearer(e.adminToken())), http.StatusOK)

	path := "/api/v1/users/" + e.userIDOf(mahasiswaA) + "/violations"
	for name, tok := range map[string]string{"TU": e.tuToken(), "KALAB": e.kalabToken(), "ADMIN": e.adminToken()} {
		res := e.do("GET", path, nil, bearer(tok))
		mustStatus(t, res, http.StatusOK)
		body := res.json()
		if body["active_points"] != float64(2) {
			t.Errorf("%s: active_points = %v; want 2 (the recent one only)", name, body["active_points"])
		}
		items := dataOf(res)
		if len(items) != 3 || body["meta"].(map[string]any)["total"] != float64(3) {
			t.Fatalf("%s: %d items; want the student's 3", name, len(items))
		}
		byID := map[string]map[string]any{}
		for _, it := range items {
			m := it.(map[string]any)
			byID[m["id"].(string)] = m
		}
		if byID[recent]["active"] != true || byID[old]["active"] != false || byID[annulled]["active"] != false {
			t.Errorf("%s: active flags = %v %v %v; want true false false", name, byID[recent]["active"], byID[old]["active"], byID[annulled]["active"])
		}
		if byID[annulled]["annul_reason"] != "Dibatalkan pengelola" || byID[annulled]["annulled_by"] != e.userIDOf(adminID) || byID[annulled]["annulled_at"] == nil {
			t.Errorf("%s: annulled entry = %v", name, byID[annulled])
		}
		if byID[recent]["room_code"] == nil || byID[recent]["booking_start_at"] == nil {
			t.Errorf("%s: entry lacks the booking summary: %v", name, byID[recent])
		}
		if items[0].(map[string]any)["id"] != recent {
			t.Errorf("%s: newest first expected, got %v", name, items[0])
		}
	}
	// Paging.
	res := e.do("GET", path+"?per_page=2&page=2", nil, bearer(e.adminToken()))
	if got := dataOf(res); len(got) != 1 || res.json()["meta"].(map[string]any)["total"] != float64(3) {
		t.Errorf("page 2 = %d items (meta %v); want 1 of 3", len(got), res.json()["meta"])
	}

	// A requester reads only their own record, and sees 404 for anyone else's.
	own := e.do("GET", path, nil, bearer(e.login(mahasiswaA).accessToken))
	mustStatus(t, own, http.StatusOK)
	if own.json()["active_points"] != float64(2) || len(dataOf(own)) != 3 {
		t.Errorf("own record: active_points = %v with %d items; want 2 with 3", own.json()["active_points"], len(dataOf(own)))
	}
	for name, tok := range map[string]string{"another student": e.login(mahasiswaB).accessToken, "an organisation": e.orgToken()} {
		if res := e.do("GET", path, nil, bearer(tok)); res.Code != http.StatusNotFound {
			t.Errorf("%s: %d; want 404", name, res.Code)
		}
	}
	// OB has no record to read, and an account that is not a requester has none.
	if res := e.do("GET", path, nil, bearer(e.obToken())); res.Code != http.StatusForbidden {
		t.Errorf("OB: %d; want 403", res.Code)
	}
	for name, p := range map[string]string{
		"a TU account": "/api/v1/users/" + e.userIDOf(tuID) + "/violations",
		"unknown user": "/api/v1/users/00000000-0000-4000-8000-000000000000/violations",
		"bad id":       "/api/v1/users/xyz/violations",
	} {
		if res := e.do("GET", p, nil, bearer(e.adminToken())); res.Code != http.StatusNotFound {
			t.Errorf("%s: %d; want 404", name, res.Code)
		}
	}
	empty := e.do("GET", "/api/v1/users/"+e.userIDOf(mahasiswaB)+"/violations", nil, bearer(e.tuToken()))
	if empty.json()["active_points"] != float64(2) {
		t.Errorf("B's active points = %v; want 2", empty.json()["active_points"])
	}
}

func TestListViolationsShowsTheBlock(t *testing.T) {
	e := newEnv(t)
	until := e.clock.Now().Add(72 * time.Hour)
	if _, err := e.pool.Exec(t.Context(), "UPDATE users SET booking_blocked_until = $1 WHERE id = $2", until, e.userIDOf(mahasiswaA)); err != nil {
		t.Fatal(err)
	}
	res := e.do("GET", "/api/v1/users/"+e.userIDOf(mahasiswaA)+"/violations", nil, bearer(e.tuToken()))
	mustStatus(t, res, http.StatusOK)
	if res.json()["booking_blocked_until"] != "2026-10-17T10:00:00+07:00" {
		t.Errorf("booking_blocked_until = %v; want the end of the block", res.json()["booking_blocked_until"])
	}
	res = e.do("GET", "/api/v1/users/"+e.userIDOf(mahasiswaB)+"/violations", nil, bearer(e.tuToken()))
	if res.json()["booking_blocked_until"] != nil {
		t.Errorf("an unblocked student has booking_blocked_until = %v", res.json()["booking_blocked_until"])
	}
}

// Annulling takes the points away and is audited; it needs a reason and works once (BR-14, BR-25).
func TestAnnulViolation(t *testing.T) {
	e := newEnv(t)
	v := e.recordViolation(mahasiswaA, "TIDAK_HADIR", 2, e.clock.Now())
	admin := e.adminToken()
	annul := func(id string, body any, tok string) response {
		return e.do("POST", "/api/v1/violations/"+id+"/annul", body, bearer(tok))
	}

	for name, body := range map[string]any{
		"no reason": map[string]string{}, "short reason": map[string]string{"reason": "ok"},
		"blank reason": map[string]string{"reason": "      "},
	} {
		if res := annul(v, body, admin); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d; want 422", name, res.Code)
		}
	}
	for name, tok := range map[string]string{"TU": e.tuToken(), "KALAB": e.kalabToken(), "MAHASISWA": e.login(mahasiswaA).accessToken, "OB": e.obToken()} {
		if res := annul(v, map[string]string{"reason": "Mohon dibatalkan"}, tok); res.Code != http.StatusForbidden {
			t.Errorf("%s: %d; want 403", name, res.Code)
		}
	}
	if res := annul("00000000-0000-4000-8000-000000000000", map[string]string{"reason": "Mohon dibatalkan"}, admin); res.Code != http.StatusNotFound {
		t.Errorf("unknown violation: %d; want 404", res.Code)
	}
	if n := e.countRows("SELECT count(*) FROM violations WHERE annulled_at IS NOT NULL"); n != 0 {
		t.Fatalf("a refused annulment changed %d violations", n)
	}

	res := annul(v, map[string]string{"reason": "  Sistem salah catat  "}, admin)
	mustStatus(t, res, http.StatusOK)
	if res.json()["active"] != false || res.json()["annul_reason"] != "Sistem salah catat" {
		t.Errorf("annulled violation = %v", res.json())
	}
	if res := annul(v, map[string]string{"reason": "Sekali lagi"}, admin); res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
		t.Errorf("second annulment: %d %s; want 409 INVALID_STATE", res.Code, res.errorCode())
	}
	if n := e.countRows("SELECT count(*) FROM audit_logs WHERE action = 'VIOLATION_ANNULLED' AND entity_id = $1 AND actor_id = $2", v, e.userIDOf(adminID)); n != 1 {
		t.Errorf("%d audit lines; want 1", n)
	}

	// The points no longer count towards the student's standing.
	me := e.do("GET", "/api/v1/auth/me", nil, bearer(e.login(mahasiswaA).accessToken))
	if me.json()["active_points"] != float64(0) {
		t.Errorf("active_points after the annulment = %v; want 0", me.json()["active_points"])
	}
}
