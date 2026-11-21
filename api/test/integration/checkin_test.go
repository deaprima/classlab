package integration_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"
	"time"
)

// wibAt is a moment on a day in WIB, e.g. wibAt(tue, "07:45:00").
func wibAt(date, hms string) time.Time {
	t, err := time.Parse(time.RFC3339, date+"T"+hms+"+07:00")
	if err != nil {
		panic(err)
	}
	return t
}

// setNow moves the injected clock to a moment on a day in WIB.
func (e *env) setNow(date, hms string) {
	e.t.Helper()
	e.clock.Set(wibAt(date, hms))
}

// roomQR reads the QR code and token of a room straight from the database.
func (e *env) roomQR(roomID string) (code, token string) {
	e.t.Helper()
	if err := e.pool.QueryRow(e.t.Context(), "SELECT code, qr_token FROM rooms WHERE id = $1", roomID).Scan(&code, &token); err != nil {
		e.t.Fatal(err)
	}
	return code, token
}

func (e *env) checkIn(token, code, qr string) response {
	e.t.Helper()
	opts := []reqOpt{}
	if token != "" {
		opts = append(opts, bearer(token))
	}
	return e.do("POST", "/api/v1/check-in", map[string]string{"room_code": code, "qr_token": qr}, opts...)
}

// approvedIn inserts a DISETUJUI booking of the student in a fresh room, bypassing the request rules.
func (e *env) approvedIn(roomID, student, start, end string) string {
	e.t.Helper()
	return e.insertBooking(roomID, student, "DISETUJUI", "Fixture", start, end)
}

// inUse brings a fresh approved booking (Tuesday 08:00-10:00) of the student to BERLANGSUNG through the API and
// leaves the clock at the moment of check-in.
func (e *env) inUse(roomID, student string) string {
	e.t.Helper()
	id := e.approvedIn(roomID, student, at(tue, "08:00"), at(tue, "10:00"))
	e.setNow(tue, "08:00:00")
	code, qr := e.roomQR(roomID)
	mustStatus(e.t, e.checkIn(e.login(student).accessToken, code, qr), http.StatusOK)
	return id
}

// postPhotoForm sends a multipart request and checks the response against the contract. token "" is anonymous.
func (e *env) postPhotoForm(token, path string, fields map[string]string, photo *photoPart) response {
	e.t.Helper()
	req, raw, rec := e.sendPhotoForm(token, path, fields, photo)
	e.checkContract(req, raw, rec)
	return response{rec}
}

// sendPhotoForm is postPhotoForm without the contract check, for provoking a 500 the contract does not describe.
func (e *env) sendPhotoForm(token, path string, fields map[string]string, photo *photoPart) (*http.Request, []byte, *httptest.ResponseRecorder) {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			e.t.Fatal(err)
		}
	}
	if photo != nil {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="photo"; filename="`+photo.filename+`"`)
		h.Set("Content-Type", photo.contentType)
		part, err := mw.CreatePart(h)
		if err != nil {
			e.t.Fatal(err)
		}
		if _, err := part.Write(photo.data); err != nil {
			e.t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		e.t.Fatal(err)
	}
	req := httptest.NewRequest("POST", path, bytes.NewReader(buf.Bytes()))
	req.RemoteAddr = "192.0.2.10:50000"
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return req, buf.Bytes(), rec
}

func (e *env) checkOut(token, bookingID string, fields map[string]string, photo *photoPart) response {
	e.t.Helper()
	return e.postPhotoForm(token, "/api/v1/bookings/"+bookingID+"/check-out", fields, photo)
}

func goodPhoto() *photoPart { return &photoPart{"kondisi.jpg", "image/jpeg", jpegBytes(0)} }

// The window is [start - 15 min, start + 15 min], judged from the server clock, edges included (BR-11).
func TestCheckInWindowBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		now  string
		want int
	}{
		{"one second before the window opens", "07:44:59", http.StatusConflict},
		{"the moment the window opens", "07:45:00", http.StatusOK},
		{"at the start", "08:00:00", http.StatusOK},
		{"the last moment of the window", "08:15:00", http.StatusOK},
		{"one second after the window closes", "08:15:01", http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			room := e.freshRoom()
			id := e.approvedIn(room, mahasiswaA, at(tue, "08:00"), at(tue, "10:00"))
			code, qr := e.roomQR(room)
			e.setNow(tue, tc.now)

			res := e.checkIn(e.login(mahasiswaA).accessToken, code, qr)
			mustStatus(t, res, tc.want)
			if tc.want == http.StatusConflict {
				if res.errorCode() != "NO_ELIGIBLE_BOOKING" || e.bookingStatus(id) != "DISETUJUI" {
					t.Errorf("code %s, status %s; want NO_ELIGIBLE_BOOKING and DISETUJUI", res.errorCode(), e.bookingStatus(id))
				}
				return
			}
			if e.bookingStatus(id) != "BERLANGSUNG" {
				t.Errorf("status = %s; want BERLANGSUNG", e.bookingStatus(id))
			}
		})
	}
}

func TestCheckInResult(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	id := e.approvedIn(room, mahasiswaA, at(tue, "08:00"), at(tue, "10:00"))
	code, qr := e.roomQR(room)
	e.setNow(tue, "07:58:30")

	res := e.checkIn(e.login(mahasiswaA).accessToken, code, qr)
	mustStatus(t, res, http.StatusOK)
	body := res.json()
	booking := body["booking"].(map[string]any)
	if booking["id"] != id || booking["status"] != "BERLANGSUNG" || booking["checked_in_at"] != "2026-10-20T07:58:30+07:00" {
		t.Errorf("booking = %v; want %s BERLANGSUNG checked in at the server time", booking, id)
	}
	if body["previous_checkout"] != nil {
		t.Errorf("previous_checkout = %v; want null for a room nobody left yet", body["previous_checkout"])
	}

	// The history records who did it; the stored time is the server's.
	var actor, from, to string
	if err := e.pool.QueryRow(t.Context(),
		"SELECT actor_id::text, from_status::text, to_status::text FROM booking_events WHERE booking_id = $1 AND to_status = 'BERLANGSUNG'", id).Scan(&actor, &from, &to); err != nil {
		t.Fatal(err)
	}
	if actor != e.userIDOf(mahasiswaA) || from != "DISETUJUI" {
		t.Errorf("event = actor %s from %s; want the student and DISETUJUI", actor, from)
	}
}

func TestCheckInRejections(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	code, qr := e.roomQR(room)
	approved := e.approvedIn(room, mahasiswaA, at(tue, "08:00"), at(tue, "10:00"))
	e.setNow(tue, "08:00:00")
	owner, other := e.login(mahasiswaA).accessToken, e.login(mahasiswaB).accessToken

	check := func(name, token, roomCode, qrToken string, wantStatus int, wantCode string) {
		t.Helper()
		res := e.checkIn(token, roomCode, qrToken)
		if res.Code != wantStatus || (wantCode != "" && res.errorCode() != wantCode) {
			t.Errorf("%s: %d %s; want %d %s", name, res.Code, res.errorCode(), wantStatus, wantCode)
		}
		if e.bookingStatus(approved) != "DISETUJUI" {
			t.Fatalf("%s: the booking changed to %s", name, e.bookingStatus(approved))
		}
	}
	check("wrong token", owner, code, "salah", http.StatusUnprocessableEntity, "QR_INVALID")
	check("token with different case", owner, code, upper(qr), http.StatusUnprocessableEntity, "QR_INVALID")
	check("unknown room", owner, "TIDAK-ADA", qr, http.StatusUnprocessableEntity, "QR_INVALID")
	check("missing fields", owner, "", "", http.StatusUnprocessableEntity, "VALIDATION_ERROR")
	check("someone else's booking", other, code, qr, http.StatusConflict, "NO_ELIGIBLE_BOOKING")
	check("anonymous", "", code, qr, http.StatusUnauthorized, "")
	for name, tok := range map[string]string{"TU": e.tuToken(), "KALAB": e.kalabToken(), "OB": e.obToken(), "ADMIN": e.adminToken()} {
		check(name, tok, code, qr, http.StatusForbidden, "FORBIDDEN")
	}

	// A booking that is not approved cannot be checked in: waiting, rejected, cancelled.
	for _, status := range []string{"MENUNGGU", "DITOLAK", "DIBATALKAN"} {
		r := e.freshRoom()
		e.insertBooking(r, mahasiswaA, status, "Fixture", at(tue, "08:00"), at(tue, "10:00"))
		c, q := e.roomQR(r)
		if res := e.checkIn(owner, c, q); res.Code != http.StatusConflict || res.errorCode() != "NO_ELIGIBLE_BOOKING" {
			t.Errorf("%s booking: %d %s; want 409 NO_ELIGIBLE_BOOKING", status, res.Code, res.errorCode())
		}
	}

	// The owner succeeds once; a second scan finds nothing left to check in.
	mustStatus(t, e.checkIn(owner, code, qr), http.StatusOK)
	if res := e.checkIn(owner, code, qr); res.Code != http.StatusConflict || res.errorCode() != "NO_ELIGIBLE_BOOKING" {
		t.Errorf("second check-in: %d %s; want 409 NO_ELIGIBLE_BOOKING", res.Code, res.errorCode())
	}
	if n := e.countRows("SELECT count(*) FROM booking_events WHERE booking_id = $1 AND to_status = 'BERLANGSUNG'", approved); n != 1 {
		t.Errorf("%d check-in events; want 1", n)
	}
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

// A booking in another room is not eligible with this room's QR, and neither is one on another day.
func TestCheckInNeedsTheBookedRoom(t *testing.T) {
	e := newEnv(t)
	booked, other := e.freshRoom(), e.freshRoom()
	e.approvedIn(booked, mahasiswaA, at(tue, "08:00"), at(tue, "10:00"))
	e.setNow(tue, "08:00:00")

	code, qr := e.roomQR(other)
	if res := e.checkIn(e.login(mahasiswaA).accessToken, code, qr); res.Code != http.StatusConflict || res.errorCode() != "NO_ELIGIBLE_BOOKING" {
		t.Errorf("other room's QR: %d %s; want 409 NO_ELIGIBLE_BOOKING", res.Code, res.errorCode())
	}
	// A QR token that belongs to another room does not open this one.
	_, otherQR := e.roomQR(other)
	bookedCode, _ := e.roomQR(booked)
	if res := e.checkIn(e.login(mahasiswaA).accessToken, bookedCode, otherQR); res.errorCode() != "QR_INVALID" {
		t.Errorf("token of another room: %d %s; want QR_INVALID", res.Code, res.errorCode())
	}
}

// When the user has a later booking in the room the error says when its window opens (brief section 6.2).
func TestCheckInTellsTheNextWindow(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	e.approvedIn(room, mahasiswaA, at(tue, "13:00"), at(tue, "15:00"))
	code, qr := e.roomQR(room)
	e.setNow(tue, "09:00:00")

	res := e.checkIn(e.login(mahasiswaA).accessToken, code, qr)
	mustStatus(t, res, http.StatusConflict)
	details := res.json()["error"].(map[string]any)["details"].([]any)
	got := map[string]string{}
	for _, d := range details {
		m := d.(map[string]any)
		got[m["field"].(string)] = m["message"].(string)
	}
	if got["next_window.opens_at"] != "2026-10-20T12:45:00+07:00" || got["next_window.closes_at"] != "2026-10-20T13:15:00+07:00" {
		t.Errorf("details = %v; want the window 12:45-13:15", got)
	}
}

// Organisations check in like students (brief section 3; C1 in the task list).
func TestOrganisationCanCheckIn(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	id := e.approvedIn(room, orgID, at(tue, "08:00"), at(tue, "10:00"))
	code, qr := e.roomQR(room)
	e.setNow(tue, "08:05:00")

	mustStatus(t, e.checkIn(e.orgToken(), code, qr), http.StatusOK)
	if e.bookingStatus(id) != "BERLANGSUNG" {
		t.Errorf("status = %s; want BERLANGSUNG", e.bookingStatus(id))
	}
}

// A user with two back-to-back bookings whose windows are both open checks into the earlier one.
func TestCheckInPicksTheEarliestOpenWindow(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	first := e.approvedIn(room, mahasiswaA, at(tue, "08:00"), at(tue, "08:20"))
	second := e.approvedIn(room, mahasiswaA, at(tue, "08:20"), at(tue, "09:30"))
	code, qr := e.roomQR(room)
	e.setNow(tue, "08:10:00") // both windows are open: 08:00 + 15 min = 08:15, 08:20 - 15 min = 08:05

	mustStatus(t, e.checkIn(e.login(mahasiswaA).accessToken, code, qr), http.StatusOK)
	if e.bookingStatus(first) != "BERLANGSUNG" || e.bookingStatus(second) != "DISETUJUI" {
		t.Errorf("first %s, second %s; want BERLANGSUNG and DISETUJUI", e.bookingStatus(first), e.bookingStatus(second))
	}
}
