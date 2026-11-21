package app_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func (e *env) getQR(token, roomID string) response {
	e.t.Helper()
	return e.do("GET", "/api/v1/rooms/"+roomID+"/qr", nil, bearer(token))
}

func (e *env) rotateQR(token, roomID string) response {
	e.t.Helper()
	return e.do("POST", "/api/v1/rooms/"+roomID+"/qr/rotate", nil, bearer(token))
}

func TestRoomQR(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	code, token := e.roomQR(room)

	res := e.getQR(e.tuToken(), room)
	mustStatus(t, res, http.StatusOK)
	body := res.json()
	if body["qr_token"] != token || len(token) < 32 {
		t.Errorf("qr_token = %v; want the stored token of %d characters", body["qr_token"], len(token))
	}
	// The QR encodes <frontend>/check-in?room=<code>&t=<token>; the frontend is the first CORS origin.
	u, err := url.Parse(body["url"].(string))
	if err != nil || u.Scheme+"://"+u.Host != "http://localhost:5173" || u.Path != "/check-in" ||
		u.Query().Get("room") != code || u.Query().Get("t") != token {
		t.Errorf("url = %v; want http://localhost:5173/check-in?room=%s&t=<token>", body["url"], code)
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q; want no-store", res.Header().Get("Cache-Control"))
	}

	// A missing room is 404 for a manager; the token never shows in the ordinary room payloads.
	if res := e.getQR(e.tuToken(), "00000000-0000-4000-8000-000000000000"); res.Code != http.StatusNotFound {
		t.Errorf("missing room: %d; want 404", res.Code)
	}
	if res := e.getQR(e.tuToken(), "bukan-uuid"); res.Code != http.StatusNotFound {
		t.Errorf("malformed id: %d; want 404", res.Code)
	}
	for _, path := range []string{"/api/v1/rooms/" + room, "/api/v1/rooms", "/api/v1/availability?start_at=" + at(tue, "08:00") + "&end_at=" + at(tue, "10:00")} {
		if got := e.do("GET", path, nil, bearer(e.adminToken())); strings.Contains(got.Body.String(), token) {
			t.Errorf("%s leaks the QR token", path)
		}
	}
}

func TestRotateQR(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	_, oldToken := e.roomQR(room)
	e.approvedIn(room, mahasiswaA, at(tue, "08:00"), at(tue, "10:00"))
	e.setNow(tue, "08:00:00")

	res := e.rotateQR(e.tuToken(), room)
	mustStatus(t, res, http.StatusOK)
	newToken := res.json()["qr_token"].(string)
	if newToken == oldToken || len(newToken) < 32 {
		t.Fatalf("new token = %q, old = %q; want a different long token", newToken, oldToken)
	}
	if res.json()["rotated_at"] != "2026-10-20T08:00:00+07:00" {
		t.Errorf("rotated_at = %v; want the server time", res.json()["rotated_at"])
	}
	if _, stored := e.roomQR(room); stored != newToken {
		t.Error("the new token was not stored")
	}
	mustStatus(t, e.getQR(e.adminToken(), room), http.StatusOK)

	// The old token stops working at once; the new one works.
	code, _ := e.roomQR(room)
	student := e.login(mahasiswaA).accessToken
	if res := e.checkIn(student, code, oldToken); res.Code != http.StatusUnprocessableEntity || res.errorCode() != "QR_INVALID" {
		t.Errorf("old token: %d %s; want 422 QR_INVALID", res.Code, res.errorCode())
	}
	mustStatus(t, e.checkIn(student, code, newToken), http.StatusOK)

	// Rotating is audited, and neither token reaches the audit log or the server log (BR-25).
	var action, entity string
	var entityID *string
	if err := e.pool.QueryRow(t.Context(),
		"SELECT action, entity_type, entity_id::text FROM audit_logs WHERE action = 'ROOM_QR_ROTATED'").Scan(&action, &entity, &entityID); err != nil {
		t.Fatalf("no audit line for the rotation: %v", err)
	}
	if entity != "ROOM" || entityID == nil || *entityID != room {
		t.Errorf("audit line = %s %s %v; want ROOM %s", action, entity, entityID, room)
	}
	for _, tok := range []string{oldToken, newToken} {
		if n := e.countRows("SELECT count(*) FROM audit_logs WHERE before::text LIKE '%' || $1 || '%' OR after::text LIKE '%' || $1 || '%'", tok); n != 0 {
			t.Errorf("a QR token is in the audit log")
		}
		if strings.Contains(e.logs.String(), tok) {
			t.Errorf("a QR token is in the server log")
		}
	}
}

// Whoever may not manage the type cannot read or rotate its QR, and a refused rotation changes nothing.
func TestRotateQRByAnOutsiderChangesNothing(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom() // KELAS: managed by TU and ADMIN
	_, before := e.roomQR(room)

	for name, tok := range map[string]string{"KALAB": e.kalabToken(), "OB": e.obToken(), "MAHASISWA": e.login(mahasiswaA).accessToken, "ORGANISASI": e.orgToken()} {
		if res := e.rotateQR(tok, room); res.Code != http.StatusForbidden {
			t.Errorf("%s rotating: %d; want 403", name, res.Code)
		}
		if res := e.getQR(tok, room); res.Code != http.StatusForbidden {
			t.Errorf("%s reading: %d; want 403", name, res.Code)
		}
	}
	if _, after := e.roomQR(room); after != before {
		t.Error("a refused rotation changed the token")
	}
	if n := e.countRows("SELECT count(*) FROM audit_logs WHERE action = 'ROOM_QR_ROTATED'"); n != 0 {
		t.Errorf("%d audit lines for refused rotations", n)
	}
}
