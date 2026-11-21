package integration_test

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (e *env) noAttachmentRows() int { return e.countRows("SELECT count(*) FROM attachments") }

func TestCheckOutSuccess(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	id := e.inUse(room, mahasiswaA)
	e.setNow(tue, "09:30:00")
	student := e.login(mahasiswaA).accessToken // after the clock moves: access tokens expire in 15 minutes

	res := e.checkOut(student, id, map[string]string{"condition": "NORMAL", "note": "  Semua rapi  "}, goodPhoto())
	mustStatus(t, res, http.StatusOK)
	body := res.json()
	b := body["booking"].(map[string]any)
	if b["status"] != "SELESAI" || b["checkout_condition"] != "NORMAL" || b["checkout_note"] != "Semua rapi" ||
		b["checked_out_at"] != "2026-10-20T09:30:00+07:00" || b["checkout_attachment_id"] == nil {
		t.Errorf("booking = %v; want SELESAI, NORMAL, the server time and a photo", b)
	}
	if body["report_id"] != nil {
		t.Errorf("report_id = %v; want null for a normal check-out", body["report_id"])
	}
	if n := e.countRows("SELECT count(*) FROM damage_reports"); n != 0 {
		t.Errorf("%d reports; want none", n)
	}
	if n := e.countRows("SELECT count(*) FROM booking_events WHERE booking_id = $1 AND from_status = 'BERLANGSUNG' AND to_status = 'SELESAI' AND actor_id = $2", id, e.userIDOf(mahasiswaA)); n != 1 {
		t.Errorf("%d check-out events by the student; want 1", n)
	}

	// The photo is a re-encoded JPEG served to its owner and to OB, who verifies rooms.
	photo := b["checkout_attachment_id"].(string)
	for name, tok := range map[string]string{"owner": student, "OB": e.obToken(), "ADMIN": e.adminToken()} {
		got := e.do("GET", "/api/v1/attachments/"+photo, nil, bearer(tok))
		if got.Code != http.StatusOK || got.Header().Get("Content-Type") != "image/jpeg" {
			t.Errorf("%s opening the photo: %d %s; want 200 image/jpeg", name, got.Code, got.Header().Get("Content-Type"))
		}
	}
	if got := e.do("GET", "/api/v1/attachments/"+photo, nil, bearer(e.login(mahasiswaB).accessToken)); got.Code != http.StatusNotFound {
		t.Errorf("another student opening the photo: %d; want 404", got.Code)
	}
}

func TestCheckOutValidation(t *testing.T) {
	e := newEnv(t)
	id := e.inUse(e.freshRoom(), mahasiswaA)
	e.setNow(tue, "09:00:00")
	student := e.login(mahasiswaA).accessToken

	notAPicture := &photoPart{"kondisi.jpg", "image/jpeg", []byte("ini bukan gambar sama sekali")}
	tooBig := &photoPart{"besar.jpg", "image/jpeg", jpegBytes(int(e.cfg.Business.MaxUploadBytes) + 1)}
	longNote := strings.Repeat("x", 2001)

	for _, tc := range []struct {
		name   string
		fields map[string]string
		photo  *photoPart
		field  string
	}{
		{"no photo", map[string]string{"condition": "NORMAL"}, nil, "photo"},
		{"photo that is not a picture", map[string]string{"condition": "NORMAL"}, notAPicture, "photo"},
		{"photo over the limit", map[string]string{"condition": "NORMAL"}, tooBig, "photo"},
		{"no condition", map[string]string{}, goodPhoto(), "condition"},
		{"unknown condition", map[string]string{"condition": "RUSAK"}, goodPhoto(), "condition"},
		{"problem without a note", map[string]string{"condition": "ADA_MASALAH"}, goodPhoto(), "note"},
		{"problem with a blank note", map[string]string{"condition": "ADA_MASALAH", "note": "   "}, goodPhoto(), "note"},
		{"note too long", map[string]string{"condition": "NORMAL", "note": longNote}, goodPhoto(), "note"},
	} {
		res := e.checkOut(student, id, tc.fields, tc.photo)
		if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), `"field":"`+tc.field+`"`) {
			t.Errorf("%s: %d %s; want 422 on %s", tc.name, res.Code, res.Body.String(), tc.field)
		}
	}
	if e.bookingStatus(id) != "BERLANGSUNG" || e.noAttachmentRows() != 0 || len(e.uploadedFiles()) != 0 || e.countRows("SELECT count(*) FROM damage_reports") != 0 {
		t.Error("a refused check-out left traces")
	}

	// It is not a multipart request at all.
	if res := e.do("POST", "/api/v1/bookings/"+id+"/check-out", map[string]string{"condition": "NORMAL"}, bearer(student)); res.Code != http.StatusBadRequest {
		t.Errorf("JSON body: %d; want 400", res.Code)
	}
}

func TestCheckOutRemovesMetadataFromThePhoto(t *testing.T) {
	e := newEnv(t)
	id := e.inUse(e.freshRoom(), mahasiswaA)
	student := e.login(mahasiswaA).accessToken
	const secret = "GPS-LAT-6.2088-LON-106.8456"

	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0, 0, 0, 0, 0}
	payload := append(append([]byte("Exif\x00\x00"), tiff...), secret...)
	segment := append([]byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}, payload...)
	plain := jpegBytes(0)
	withGPS := append(append(append([]byte{}, plain[:2]...), segment...), plain[2:]...)
	withGPS = append(withGPS, []byte("HIDDEN-PAYLOAD-"+secret)...)

	res := e.checkOut(student, id, map[string]string{"condition": "NORMAL"}, &photoPart{"IMG_0001.jpg", "image/jpeg", withGPS})
	mustStatus(t, res, http.StatusOK)
	photo := res.json()["booking"].(map[string]any)["checkout_attachment_id"].(string)

	served := e.do("GET", "/api/v1/attachments/"+photo, nil, bearer(student))
	mustStatus(t, served, http.StatusOK)
	for _, marker := range [][]byte{[]byte(secret), []byte("HIDDEN-PAYLOAD"), []byte("Exif")} {
		if bytes.Contains(served.Body.Bytes(), marker) {
			t.Errorf("the served photo still carries %q", marker)
		}
	}
	for _, f := range e.uploadedFiles() {
		raw, err := os.ReadFile(filepath.Join(e.cfg.UploadDir, "attachments", f))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte(secret)) || bytes.Contains(raw, []byte("Exif")) {
			t.Errorf("the stored file %s still carries metadata", f)
		}
	}
}

// A room left with a problem gets a report in the same transaction, using the same photo, and its approvers
// are told (BR-12).
func TestCheckOutWithAProblemFilesAReport(t *testing.T) {
	e := newEnv(t)
	tuUser, kalabUser := e.userIDOf(tuID), e.userIDOf(kalabID)

	for _, tc := range []struct {
		typ      string
		notified string
		not      string
	}{
		{"KELAS", tuUser, kalabUser},
		{"LAB", kalabUser, tuUser},
	} {
		room := e.insertRoom("CO-"+tc.typ, tc.typ, 30)
		id := e.inUse(room, mahasiswaA)
		e.setNow(tue, "09:00:00")

		res := e.checkOut(e.login(mahasiswaA).accessToken, id, map[string]string{"condition": "ADA_MASALAH", "note": "Proyektor tidak menyala"}, goodPhoto())
		mustStatus(t, res, http.StatusOK)
		report, _ := res.json()["report_id"].(string)
		if report == "" {
			t.Fatalf("%s: no report_id in %s", tc.typ, res.Body.String())
		}

		var status, desc, room2, reporter string
		var attachment string
		if err := e.pool.QueryRow(t.Context(),
			"SELECT status::text, description, room_id::text, reporter_id::text, attachment_id::text FROM damage_reports WHERE id = $1", report).
			Scan(&status, &desc, &room2, &reporter, &attachment); err != nil {
			t.Fatal(err)
		}
		booking := res.json()["booking"].(map[string]any)
		if status != "BARU" || desc != "Proyektor tidak menyala" || room2 != room || reporter != e.userIDOf(mahasiswaA) || attachment != booking["checkout_attachment_id"] {
			t.Errorf("%s: report = %s %q %s %s %s; want BARU, the note, the room, the student and the check-out photo", tc.typ, status, desc, room2, reporter, attachment)
		}
		if n := e.countRows("SELECT count(*) FROM report_events WHERE report_id = $1 AND to_status = 'BARU'", report); n != 1 {
			t.Errorf("%s: %d first report events; want 1", tc.typ, n)
		}
		if n := e.countRows("SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'REPORT_CREATED' AND ref_id = $2", tc.notified, report); n != 1 {
			t.Errorf("%s: approver got %d REPORT_CREATED; want 1", tc.typ, n)
		}
		if n := e.countRows("SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'REPORT_CREATED' AND ref_id = $2", tc.not, report); n != 0 {
			t.Errorf("%s: the other manager got %d REPORT_CREATED; want 0", tc.typ, n)
		}

		// The approver reads the report and the photo like any other report of their type.
		tok := e.tuToken()
		if tc.typ == "LAB" {
			tok = e.kalabToken()
		}
		mustStatus(t, e.do("GET", "/api/v1/reports/"+report, nil, bearer(tok)), http.StatusOK)
		mustStatus(t, e.do("GET", "/api/v1/attachments/"+attachment, nil, bearer(tok)), http.StatusOK)
	}
}

// The booking, its history, the photo, the report and the notification succeed or fail together (agent.md 7, #9).
func TestCheckOutIsAtomic(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	id := e.inUse(room, mahasiswaA)
	e.setNow(tue, "09:00:00")
	student := e.login(mahasiswaA).accessToken
	events := e.eventCount(id)

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
	for _, tc := range []struct {
		table  string
		fields map[string]string
	}{
		{"booking_events", map[string]string{"condition": "NORMAL"}},
		{"attachments", map[string]string{"condition": "NORMAL"}},
		{"damage_reports", map[string]string{"condition": "ADA_MASALAH", "note": "Rusak"}},
		{"report_events", map[string]string{"condition": "ADA_MASALAH", "note": "Rusak"}},
		{"notifications", map[string]string{"condition": "ADA_MASALAH", "note": "Rusak"}},
	} {
		breakTable(tc.table)
		res := e.rawCheckOut(student, id, tc.fields, goodPhoto())
		if res.Code != http.StatusInternalServerError || strings.Contains(res.Body.String(), "injected") {
			t.Errorf("%s failing: %d %s; want a generic 500", tc.table, res.Code, res.Body.String())
		}
		if e.bookingStatus(id) != "BERLANGSUNG" || e.eventCount(id) != events ||
			e.countRows("SELECT count(*) FROM damage_reports") != 0 || e.countRows("SELECT count(*) FROM report_events") != 0 ||
			e.countRows("SELECT count(*) FROM notifications WHERE type = 'REPORT_CREATED'") != 0 ||
			e.noAttachmentRows() != 0 || len(e.uploadedFiles()) != 0 {
			t.Errorf("%s failing: the check-out left traces", tc.table)
		}
		repair(tc.table)
	}

	// With everything repaired the same request goes through.
	mustStatus(t, e.checkOut(student, id, map[string]string{"condition": "ADA_MASALAH", "note": "Rusak"}, goodPhoto()), http.StatusOK)
}

// rawCheckOut is checkOut without the contract check, for provoking a 500 that the contract does not describe.
func (e *env) rawCheckOut(token, bookingID string, fields map[string]string, photo *photoPart) response {
	e.t.Helper()
	_, _, rec := e.sendPhotoForm(token, "/api/v1/bookings/"+bookingID+"/check-out", fields, photo)
	return response{rec}
}

func TestCheckOutAccessAndState(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	id := e.inUse(room, mahasiswaA)
	e.setNow(tue, "09:00:00")
	fields := map[string]string{"condition": "NORMAL"}

	if res := e.checkOut(e.login(mahasiswaB).accessToken, id, fields, goodPhoto()); res.Code != http.StatusNotFound {
		t.Errorf("another student: %d; want 404", res.Code)
	}
	if res := e.checkOut(e.orgToken(), id, fields, goodPhoto()); res.Code != http.StatusNotFound {
		t.Errorf("an organisation: %d; want 404", res.Code)
	}
	for name, tok := range map[string]string{"TU": e.tuToken(), "KALAB": e.kalabToken(), "OB": e.obToken(), "ADMIN": e.adminToken()} {
		if res := e.checkOut(tok, id, fields, goodPhoto()); res.Code != http.StatusForbidden {
			t.Errorf("%s: %d; want 403", name, res.Code)
		}
	}
	if res := e.checkOut("", id, fields, goodPhoto()); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d; want 401", res.Code)
	}
	if res := e.checkOut(e.login(mahasiswaA).accessToken, "tidak-valid", fields, goodPhoto()); res.Code != http.StatusNotFound {
		t.Errorf("malformed id: %d; want 404", res.Code)
	}
	if e.bookingStatus(id) != "BERLANGSUNG" {
		t.Fatalf("a refused check-out changed the booking to %s", e.bookingStatus(id))
	}

	// Only a booking in use can be checked out of.
	owner := e.login(mahasiswaA).accessToken
	for _, status := range []string{"MENUNGGU", "DISETUJUI", "DITOLAK", "DIBATALKAN", "TIDAK_HADIR", "KEDALUWARSA"} {
		other := e.insertBooking(e.freshRoom(), mahasiswaA, status, "Fixture", at(tue, "13:00"), at(tue, "14:00"))
		if res := e.checkOut(owner, other, fields, goodPhoto()); res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
			t.Errorf("%s booking: %d %s; want 409 INVALID_STATE", status, res.Code, res.errorCode())
		}
	}

	// ...and only once.
	mustStatus(t, e.checkOut(owner, id, fields, goodPhoto()), http.StatusOK)
	if res := e.checkOut(owner, id, fields, goodPhoto()); res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
		t.Errorf("second check-out: %d %s; want 409 INVALID_STATE", res.Code, res.errorCode())
	}
	if e.noAttachmentRows() != 1 {
		t.Errorf("%d attachments; want only the first photo", e.noAttachmentRows())
	}
}

// Leaving early frees the rest of the slot for somebody else (BR-12).
func TestEarlyCheckOutReleasesTheSlot(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	id := e.approvedIn(room, mahasiswaA, at(tue, "08:00"), at(tue, "12:00"))
	code, qr := e.roomQR(room)
	e.setNow(tue, "08:00:00")
	mustStatus(t, e.checkIn(e.login(mahasiswaA).accessToken, code, qr), http.StatusOK)

	e.setNow(tue, "09:00:00")
	other := e.login(mahasiswaB).accessToken
	if res := e.postBooking(other, room, at(tue, "09:30"), at(tue, "11:00"), nil); res.Code != http.StatusConflict {
		t.Fatalf("while in use: %d; want 409", res.Code)
	}
	mustStatus(t, e.checkOut(e.login(mahasiswaA).accessToken, id, map[string]string{"condition": "NORMAL"}, goodPhoto()), http.StatusOK)
	mustStatus(t, e.postBooking(other, room, at(tue, "09:30"), at(tue, "11:00"), nil), http.StatusCreated)
}

// The next person to check in sees how the previous one left the room (BR-11).
func TestCheckInShowsThePreviousCheckout(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	first := e.approvedIn(room, mahasiswaA, at(tue, "08:00"), at(tue, "10:00"))
	e.approvedIn(room, mahasiswaB, at(tue, "10:00"), at(tue, "12:00"))
	code, qr := e.roomQR(room)

	e.setNow(tue, "08:00:00")
	mustStatus(t, e.checkIn(e.login(mahasiswaA).accessToken, code, qr), http.StatusOK)
	e.setNow(tue, "09:58:10")
	out := e.checkOut(e.login(mahasiswaA).accessToken, first, map[string]string{"condition": "ADA_MASALAH", "note": "Kursi patah"}, goodPhoto())
	mustStatus(t, out, http.StatusOK)
	photo := out.json()["booking"].(map[string]any)["checkout_attachment_id"]

	e.setNow(tue, "10:00:00")
	res := e.checkIn(e.login(mahasiswaB).accessToken, code, qr)
	mustStatus(t, res, http.StatusOK)
	prev, _ := res.json()["previous_checkout"].(map[string]any)
	if prev["checked_out_at"] != "2026-10-20T09:58:10+07:00" || prev["condition"] != "ADA_MASALAH" || prev["attachment_id"] != photo {
		t.Errorf("previous_checkout = %v; want the first user's check-out", prev)
	}
}

func TestCheckoutFeed(t *testing.T) {
	e := newEnv(t)
	kelas, lab := e.freshRoom(), e.insertRoom("CO-LAB", "LAB", 20)
	e.setNow(tue, "08:00:00")

	a := e.inUse(kelas, mahasiswaA)
	e.setNow(tue, "09:00:00")
	mustStatus(t, e.checkOut(e.login(mahasiswaA).accessToken, a, map[string]string{"condition": "NORMAL"}, goodPhoto()), http.StatusOK)

	b := e.inUse(lab, mahasiswaB)
	e.setNow(tue, "09:30:00")
	mustStatus(t, e.checkOut(e.login(mahasiswaB).accessToken, b, map[string]string{"condition": "ADA_MASALAH", "note": "Kabel putus"}, goodPhoto()), http.StatusOK)

	ids := func(token, query string) []string {
		t.Helper()
		res := e.do("GET", "/api/v1/checkouts"+query, nil, bearer(token))
		mustStatus(t, res, http.StatusOK)
		var out []string
		for _, it := range dataOf(res) {
			out = append(out, it.(map[string]any)["booking_id"].(string))
		}
		return out
	}
	want := func(name string, got []string, ids ...string) {
		t.Helper()
		if strings.Join(got, ",") != strings.Join(ids, ",") {
			t.Errorf("%s sees %v; want %v", name, got, ids)
		}
	}
	// Newest first; OB and ADMIN see every room, TU only KELAS, KALAB only LAB (BR-10).
	want("OB", ids(e.obToken(), ""), b, a)
	want("ADMIN", ids(e.adminToken(), ""), b, a)
	want("TU", ids(e.tuToken(), ""), a)
	want("KALAB", ids(e.kalabToken(), ""), b)
	want("room filter", ids(e.obToken(), "?room_id="+lab), b)
	want("time filter", ids(e.obToken(), "?from="+at(tue, "09:15")), b)
	want("upper bound", ids(e.obToken(), "?to="+at(tue, "09:15")), a)

	first := dataOf(e.do("GET", "/api/v1/checkouts?room_id="+lab, nil, bearer(e.obToken())))[0].(map[string]any)
	if first["condition"] != "ADA_MASALAH" || first["note"] != "Kabel putus" || first["room_code"] != "CO-LAB" || first["room_type"] != "LAB" {
		t.Errorf("item = %v; want the LAB problem with its note", first)
	}

	for name, tok := range map[string]string{"MAHASISWA": e.login(mahasiswaA).accessToken, "ORGANISASI": e.orgToken()} {
		if res := e.do("GET", "/api/v1/checkouts", nil, bearer(tok)); res.Code != http.StatusForbidden {
			t.Errorf("%s: %d; want 403", name, res.Code)
		}
	}
	if res := e.do("GET", "/api/v1/checkouts", nil); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d; want 401", res.Code)
	}
	for _, q := range []string{"?room_id=x", "?from=kemarin", "?page=0", "?from=" + at(tue, "10:00") + "&to=" + at(tue, "09:00")} {
		if res := e.do("GET", "/api/v1/checkouts"+q, nil, bearer(e.obToken())); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d; want 422", q, res.Code)
		}
	}
}
