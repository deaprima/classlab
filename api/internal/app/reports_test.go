package app_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"classlab/api/internal/config"
)

// photoPart describes the file part of a report upload.
type photoPart struct {
	filename    string
	contentType string
	data        []byte
}

// The photo fixtures are real pictures, because uploads are decoded and written again as JPEG.

// jpegBytes is a real JPEG padded with trailing zeros up to size. Bytes after the end marker are kept,
// which lets the size-limit tests hit an exact length.
func jpegBytes(size int) []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 8, 8)), nil); err != nil {
		panic(err)
	}
	return pad(buf.Bytes(), size)
}

// pngBytes is a real PNG. size is ignored because anything after the end chunk is discarded.
func pngBytes(size int) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 8, 8))); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func pad(data []byte, size int) []byte {
	if size <= len(data) {
		return data
	}
	return append(data, make([]byte, size-len(data))...)
}

func (e *env) multipartReport(token string, fields map[string]string, photo *photoPart) (*http.Request, []byte, *httptest.ResponseRecorder) {
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

	req := httptest.NewRequest("POST", "/api/v1/reports", bytes.NewReader(buf.Bytes()))
	req.RemoteAddr = "192.0.2.10:50000"
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return req, buf.Bytes(), rec
}

// postReport files a report and checks the response against the contract.
func (e *env) postReport(token, room, description string, photo *photoPart) response {
	e.t.Helper()
	fields := map[string]string{"room_id": room, "description": description}
	req, raw, rec := e.multipartReport(token, fields, photo)
	e.checkContract(req, raw, rec)
	return response{rec}
}

func (e *env) patchReport(token, id string, body any) response {
	e.t.Helper()
	return e.do("PATCH", "/api/v1/reports/"+id+"/status", body, bearer(token))
}

func (e *env) reportStatus(id string) string {
	e.t.Helper()
	var s string
	if err := e.pool.QueryRow(e.t.Context(), "SELECT status FROM damage_reports WHERE id = $1", id).Scan(&s); err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *env) reportEventCount(id string) int {
	e.t.Helper()
	return e.countRows("SELECT count(*) FROM report_events WHERE report_id = $1", id)
}

// reportAt creates a report for the student in a KELAS room and advances it (as the TU, who approves that type) to the wanted status.
func (e *env) reportAt(status, student string) string {
	e.t.Helper()
	res := e.postReport(e.login(student).accessToken, e.freshRoom(), "Proyektor rusak", nil)
	mustStatus(e.t, res, http.StatusCreated)
	id := idOf(res)
	tu := e.tuToken()
	for _, step := range []string{"DITERIMA", "DIPROSES", "SELESAI"} {
		if e.reportStatus(id) == status {
			break
		}
		mustStatus(e.t, e.patchReport(tu, id, map[string]string{"status": step}), http.StatusOK)
	}
	if e.reportStatus(id) != status {
		e.t.Fatalf("could not bring report to %s", status)
	}
	return id
}

// addTU creates another active TU account through the API. The account still has a temporary password, which
// is fine for the tests that only need it to exist as a notification recipient.
func (e *env) addTU(identifier string) string {
	e.t.Helper()
	body := e.newUserBody(identifier)
	body["role"] = "TU"
	res := e.do("POST", "/api/v1/users", body, bearer(e.adminToken()))
	mustStatus(e.t, res, http.StatusCreated)
	return res.json()["id"].(string)
}

// uploadedFiles lists the files stored under <UPLOAD_DIR>/attachments.
func (e *env) uploadedFiles() []string {
	e.t.Helper()
	entries, err := os.ReadDir(filepath.Join(e.cfg.UploadDir, "attachments"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		e.t.Fatal(err)
	}
	var names []string
	for _, ent := range entries {
		names = append(names, ent.Name())
	}
	return names
}

var storedPhotoName = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}.jpg$`)

func TestCreateReportAsEveryRole(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()

	for _, who := range []string{mahasiswaA, orgID, obID, tuID, kalabID, adminID} {
		sess := e.login(who)
		res := e.postReport(sess.accessToken, room, "  AC bocor  ", nil)
		mustStatus(t, res, http.StatusCreated)

		r := res.json()
		if r["status"] != "BARU" || r["description"] != "AC bocor" || r["reporter_id"] != sess.userID || r["room_id"] != room {
			t.Errorf("%s: unexpected report %v", who, r)
		}
		if rep, _ := r["reporter"].(map[string]any); rep["identifier"] != who || rep["email"] != nil {
			t.Errorf("%s: reporter should expose identifier and name only: %v", who, r["reporter"])
		}
		if r["attachment_id"] != nil || r["handled_by"] != nil {
			t.Errorf("%s: a new report has no photo or handler: %v", who, r)
		}

		var from *string
		var to, actor string
		if err := e.pool.QueryRow(t.Context(), "SELECT from_status, to_status, actor_id FROM report_events WHERE report_id = $1", idOf(res)).Scan(&from, &to, &actor); err != nil {
			t.Fatal(err)
		}
		if from != nil || to != "BARU" || actor != sess.userID {
			t.Errorf("%s: creation event = %v -> %s by %s", who, from, to, actor)
		}
	}
	if res := e.postReport("", room, "x", nil); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d; want 401", res.Code)
	}
}

func TestCreateReportValidation(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()
	tok := e.login(mahasiswaA).accessToken

	deleted := e.freshRoom()
	if _, err := e.pool.Exec(t.Context(), "UPDATE rooms SET deleted_at = now(), is_active = false WHERE id = $1", deleted); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		fields map[string]string
		field  string
	}{
		{"missing description", map[string]string{"room_id": room}, "description"},
		{"blank description", map[string]string{"room_id": room, "description": "   "}, "description"},
		{"huge description", map[string]string{"room_id": room, "description": strings.Repeat("x", 2001)}, "description"},
		{"missing room", map[string]string{"description": "rusak"}, "room_id"},
		{"room not a uuid", map[string]string{"room_id": "ruang-1", "description": "rusak"}, "room_id"},
		{"unknown room", map[string]string{"room_id": "00000000-0000-4000-8000-000000000001", "description": "rusak"}, "room_id"},
		{"deleted room", map[string]string{"room_id": deleted, "description": "rusak"}, "room_id"},
	}
	for _, tc := range cases {
		req, raw, rec := e.multipartReport(tok, tc.fields, nil)
		e.checkContract(req, raw, rec)
		res := response{rec}
		if res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)[tc.field] {
			t.Errorf("%s: %d %s; want 422 naming %q", tc.name, res.Code, res.Body.String(), tc.field)
		}
	}

	if res := e.do("POST", "/api/v1/reports", map[string]string{"room_id": room, "description": "x"}, bearer(tok)); res.Code != http.StatusBadRequest {
		t.Errorf("JSON body: %d; want 400 (multipart required)", res.Code)
	}

	// Damage in a room that is switched off can still be reported.
	inactive := e.freshRoom()
	if _, err := e.pool.Exec(t.Context(), "UPDATE rooms SET is_active = false WHERE id = $1", inactive); err != nil {
		t.Fatal(err)
	}
	if res := e.postReport(tok, inactive, "Jendela pecah", nil); res.Code != http.StatusCreated {
		t.Errorf("inactive room: %d; want 201", res.Code)
	}
	if n := e.countRows("SELECT count(*) FROM damage_reports"); n != 1 {
		t.Errorf("%d reports stored; want only the valid one", n)
	}
}

// realWebP is a 1x1 lossless WebP (a hand-written file, because the standard library only reads WebP).
func realWebP() []byte {
	b, err := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	if err != nil {
		panic(err)
	}
	return b
}

// Every accepted format comes back as a JPEG (BR-16): the upload is decoded and written again.
func TestReportPhotoUploadAndDownload(t *testing.T) {
	e := newEnv(t)
	tok := e.login(mahasiswaA).accessToken
	room := e.freshRoom()

	for _, tc := range []struct {
		name  string
		photo *photoPart
	}{
		{"jpeg", &photoPart{"foto.jpg", "image/jpeg", jpegBytes(300)}},
		{"png", &photoPart{"foto.png", "image/png", pngBytes(300)}},
		{"webp", &photoPart{"foto.webp", "image/webp", realWebP()}},
		// The file's own bytes decide, not what the client claims.
		{"png sent as .php with a text/plain type", &photoPart{"../../evil.php", "text/plain", pngBytes(64)}},
	} {
		before := e.uploadedFiles()
		res := e.postReport(tok, room, "Papan tulis rusak", tc.photo)
		mustStatus(t, res, http.StatusCreated)
		id := idOf(res)

		attachmentID, _ := res.json()["attachment_id"].(string)
		if attachmentID == "" {
			t.Errorf("%s: attachment_id = %v; want the photo's id", tc.name, res.json()["attachment_id"])
		}
		if _, has := res.json()["photo_path"]; has {
			t.Errorf("%s: the response still has photo_path", tc.name)
		}
		added, _ := diff(before, e.uploadedFiles())
		if len(added) != 1 || !storedPhotoName.MatchString(added[0]) || filepath.Ext(added[0]) != ".jpg" {
			t.Errorf("%s: stored files %v; want one UUID name ending .jpg", tc.name, added)
		}
		if strings.Contains(strings.Join(added, ""), "evil") {
			t.Errorf("%s: client file name leaked into the stored name %v", tc.name, added)
		}

		got := e.do("GET", "/api/v1/attachments/"+attachmentID, nil, bearer(tok))
		mustStatus(t, got, http.StatusOK)
		if got.Header().Get("Content-Type") != "image/jpeg" || got.Header().Get("X-Content-Type-Options") != "nosniff" ||
			!strings.HasPrefix(got.Header().Get("Content-Disposition"), "inline") {
			t.Errorf("%s: served %q, nosniff=%q, disposition=%q", tc.name, got.Header().Get("Content-Type"),
				got.Header().Get("X-Content-Type-Options"), got.Header().Get("Content-Disposition"))
		}
		if _, err := jpeg.Decode(bytes.NewReader(got.Body.Bytes())); err != nil {
			t.Errorf("%s: the served photo is not a JPEG: %v", tc.name, err)
		}
		if strings.Contains(res.Body.String(), filepath.Join(e.cfg.UploadDir)) || strings.Contains(res.Body.String(), added[0]) {
			t.Errorf("%s: the response reveals where the file is stored: %s", tc.name, res.Body.String())
		}
		// The old per-report route is gone.
		if old := e.do("GET", "/api/v1/reports/"+id+"/photo", nil, bearer(tok)); old.Code != http.StatusNotFound && old.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: GET /reports/{id}/photo = %d; want it removed", tc.name, old.Code)
		}
	}

	// An empty file part, as a browser sends when no file was chosen, means no photo.
	res := e.postReport(tok, room, "Tanpa foto", &photoPart{"", "application/octet-stream", nil})
	mustStatus(t, res, http.StatusCreated)
	if res.json()["attachment_id"] != nil {
		t.Errorf("empty file part produced a photo: %v", res.json()["attachment_id"])
	}
}

func diff(before, after []string) (added, removed []string) {
	for _, a := range after {
		if !slices.Contains(before, a) {
			added = append(added, a)
		}
	}
	for _, b := range before {
		if !slices.Contains(after, b) {
			removed = append(removed, b)
		}
	}
	return added, removed
}

func TestReportPhotoRejections(t *testing.T) {
	e := newEnv(t)
	tok := e.login(mahasiswaA).accessToken
	room := e.freshRoom()
	limit := int(e.cfg.Business.MaxUploadBytes)

	for _, tc := range []struct {
		name  string
		photo *photoPart
	}{
		{"gif posing as jpeg", &photoPart{"a.jpg", "image/jpeg", []byte("GIF89a" + strings.Repeat("\x00", 64))}},
		{"text posing as png", &photoPart{"a.png", "image/png", []byte("bukan gambar")}},
		{"svg", &photoPart{"a.svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)}},
		{"html polyglot named .jpg", &photoPart{"a.jpg", "image/jpeg", []byte("<html><script>alert(1)</script></html>")}},
		{"riff that is not webp", &photoPart{"a.webp", "image/webp", pad([]byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'A', 'V', 'E'}, 64)}},
		{"truncated jpeg signature", &photoPart{"a.jpg", "image/jpeg", []byte{0xFF, 0xD8}}},
		{"named but empty file", &photoPart{"a.jpg", "image/jpeg", nil}},
		{"one byte over the limit", &photoPart{"a.jpg", "image/jpeg", jpegBytes(limit + 1)}},
		{"far over the limit", &photoPart{"a.jpg", "image/jpeg", jpegBytes(limit * 2)}},
	} {
		res := e.postReport(tok, room, "Rusak", tc.photo)
		if res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)["photo"] {
			t.Errorf("%s: %d %s; want 422 naming photo", tc.name, res.Code, res.Body.String())
		}
	}
	if n := e.countRows("SELECT count(*) FROM damage_reports"); n != 0 {
		t.Errorf("%d reports stored by rejected uploads", n)
	}
	if files := e.uploadedFiles(); len(files) != 0 {
		t.Errorf("rejected uploads left files behind: %v", files)
	}

	// Exactly at the limit is fine.
	if res := e.postReport(tok, room, "Rusak", &photoPart{"a.jpg", "image/jpeg", jpegBytes(limit)}); res.Code != http.StatusCreated {
		t.Errorf("photo of exactly the limit: %d %s; want 201", res.Code, res.Body.String())
	}
}

func TestReportPhotoAccess(t *testing.T) {
	e := newEnv(t)
	owner := e.login(mahasiswaA).accessToken
	withPhotoRes := e.postReport(owner, e.freshRoom(), "Rusak", &photoPart{"a.jpg", "image/jpeg", jpegBytes(100)})
	mustStatus(t, withPhotoRes, http.StatusCreated)
	photoID, _ := withPhotoRes.json()["attachment_id"].(string)
	without := e.postReport(owner, e.freshRoom(), "Rusak", nil)
	mustStatus(t, without, http.StatusCreated)
	if without.json()["attachment_id"] != nil {
		t.Errorf("report without a photo has attachment_id %v", without.json()["attachment_id"])
	}

	get := func(tok, id string) int {
		return e.do("GET", "/api/v1/attachments/"+id, nil, bearer(tok)).Code
	}
	// The uploader, the approver of the room's type, ADMIN, and OB (who handles every damage photo).
	for name, tok := range map[string]string{"owner": owner, "TU": e.tuToken(), "ADMIN": e.adminToken(), "OB": e.login(obID).accessToken} {
		if code := get(tok, photoID); code != http.StatusOK {
			t.Errorf("%s: %d; want 200", name, code)
		}
	}
	for name, tok := range map[string]string{"another student": e.login(mahasiswaB).accessToken, "KALAB (not the approver of KELAS)": e.kalabToken()} {
		if code := get(tok, photoID); code != http.StatusNotFound {
			t.Errorf("%s: %d; want 404", name, code)
		}
	}
	if res := e.do("GET", "/api/v1/attachments/"+photoID, nil); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d; want 401", res.Code)
	}

	// A database row that names something other than one of our own files must never reach the disk.
	for _, name := range []string{"../../../etc/passwd", `..\..\secret.jpg`, "/etc/passwd", "x.jpg", "attachments/00000000-0000-4000-8000-000000000000.sh"} {
		if _, err := e.pool.Exec(t.Context(), "UPDATE attachments SET storage_path = $1 WHERE id = $2", name, photoID); err != nil {
			t.Fatal(err)
		}
		if code := get(owner, photoID); code != http.StatusNotFound {
			t.Errorf("stored name %q: %d; want 404", name, code)
		}
	}
}

func TestReportVisibilityAndListing(t *testing.T) {
	e := newEnv(t)
	a, b, ob := e.login(mahasiswaA).accessToken, e.login(mahasiswaB).accessToken, e.login(obID).accessToken
	tu, admin := e.tuToken(), e.adminToken()
	roomX, roomY := e.freshRoom(), e.freshRoom()

	a1 := idOf(e.postReport(a, roomX, "a1", nil))
	e.clock.Add(1e9) // distinct created_at values so the order is defined
	a2 := idOf(e.postReport(a, roomY, "a2", nil))
	e.clock.Add(1e9)
	o1 := idOf(e.postReport(ob, roomX, "o1", nil))
	mustStatus(t, e.patchReport(tu, a2, map[string]string{"status": "DITERIMA"}), http.StatusOK)

	list := func(tok, query string) []string {
		res := e.do("GET", "/api/v1/reports"+query, nil, bearer(tok))
		mustStatus(t, res, http.StatusOK)
		var ids []string
		for _, it := range dataOf(res) {
			ids = append(ids, it.(map[string]any)["id"].(string))
		}
		return ids
	}
	same := func(name string, got []string, want ...string) {
		t.Helper()
		if !slices.Equal(got, want) {
			t.Errorf("%s = %v; want %v", name, got, want)
		}
	}

	same("student A", list(a, ""), a2, a1)
	same("student B", list(b, ""))
	same("OB sees only their own", list(ob, ""), o1)
	same("TU_KALAB sees all", list(tu, ""), o1, a2, a1)
	same("KAPRODI sees all", list(admin, ""), o1, a2, a1)
	same("status filter", list(tu, "?status=DITERIMA"), a2)
	same("room filter", list(tu, "?room_id="+roomX), o1, a1)
	same("student cannot reach others by filter", list(a, "?room_id="+roomX), a1)

	res := e.do("GET", "/api/v1/reports?per_page=2&page=2", nil, bearer(tu))
	if got := dataOf(res); len(got) != 1 || res.json()["meta"].(map[string]any)["total"] != float64(3) {
		t.Errorf("page 2 = %v (meta %v); want 1 of 3", got, res.json()["meta"])
	}
	for name, q := range map[string]string{"bad status": "?status=SELESAIX", "bad room": "?room_id=x", "bad page": "?page=0"} {
		if res := e.do("GET", "/api/v1/reports"+q, nil, bearer(tu)); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d; want 422", name, res.Code)
		}
	}

	// Detail: the reporter and the handlers see it with its history; anyone else gets 404.
	for name, tok := range map[string]string{"reporter": a, "TU": tu, "ADMIN": admin} {
		res := e.do("GET", "/api/v1/reports/"+a2, nil, bearer(tok))
		mustStatus(t, res, http.StatusOK)
		events, _ := res.json()["events"].([]any)
		if len(events) != 2 || res.json()["status"] != "DITERIMA" || res.json()["handled_by"] != e.userIDOf(tuID) {
			t.Errorf("%s: detail = %v", name, res.json())
		}
	}
	for name, tok := range map[string]string{"another student": b, "OB": ob} {
		if res := e.do("GET", "/api/v1/reports/"+a2, nil, bearer(tok)); res.Code != http.StatusNotFound {
			t.Errorf("%s: %d; want 404", name, res.Code)
		}
	}
	if res := e.do("GET", "/api/v1/reports/00000000-0000-4000-8000-0000000000aa", nil, bearer(tu)); res.Code != http.StatusNotFound {
		t.Errorf("missing report: %d; want 404", res.Code)
	}
}

// BR-08: a report moves exactly one step forward. Every pair of statuses is tried.
func TestReportStatusLifecycle(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	order := []string{"BARU", "DITERIMA", "DIPROSES", "SELESAI"}

	for fi, from := range order {
		for ti, to := range order {
			id := e.reportAt(from, mahasiswaA)
			events := e.reportEventCount(id)
			res := e.patchReport(tu, id, map[string]string{"status": to})

			if ti == fi+1 {
				mustStatus(t, res, http.StatusOK)
				if res.json()["status"] != to || e.reportStatus(id) != to {
					t.Errorf("%s -> %s: status %v", from, to, res.json()["status"])
				}
				if n := e.reportEventCount(id); n != events+1 {
					t.Errorf("%s -> %s: events %d -> %d; want exactly one more", from, to, events, n)
				}
				continue
			}
			if res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
				t.Errorf("%s -> %s: %d %s; want 409 INVALID_STATE", from, to, res.Code, res.Body.String())
			}
			if e.reportStatus(id) != from || e.reportEventCount(id) != events {
				t.Errorf("%s -> %s: a refused change altered the report", from, to)
			}
		}
	}
}

func TestReportStatusUpdateDetails(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	reporter := e.userIDOf(mahasiswaA)
	id := e.reportAt("BARU", mahasiswaA)

	for name, body := range map[string]any{
		"unknown status": map[string]string{"status": "RUSAK"},
		"empty status":   map[string]string{},
		"long note":      map[string]string{"status": "DITERIMA", "note": strings.Repeat("x", 501)},
	} {
		if res := e.patchReport(tu, id, body); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d; want 422", name, res.Code)
		}
	}
	for name, body := range map[string]string{"not json": "status=DITERIMA", "unknown field": `{"status":"DITERIMA","x":1}`} {
		if res := e.do("PATCH", "/api/v1/reports/"+id+"/status", body, bearer(tu)); res.Code != http.StatusBadRequest {
			t.Errorf("%s: %d; want 400", name, res.Code)
		}
	}
	if e.reportStatus(id) != "BARU" {
		t.Fatal("a rejected request changed the report")
	}

	res := e.patchReport(tu, id, map[string]string{"status": "DITERIMA", "note": "  Teknisi dijadwalkan  "})
	mustStatus(t, res, http.StatusOK)
	if res.json()["handled_by"] != e.userIDOf(tuID) {
		t.Errorf("handled_by = %v; want the TU/Kalab who acted", res.json()["handled_by"])
	}
	var note string
	if err := e.pool.QueryRow(t.Context(), "SELECT note FROM report_events WHERE report_id = $1 AND to_status = 'DITERIMA'", id).Scan(&note); err != nil || note != "Teknisi dijadwalkan" {
		t.Errorf("event note = %q (%v); want it trimmed", note, err)
	}

	// The reporter is told, with the note.
	var body string
	if err := e.pool.QueryRow(t.Context(),
		"SELECT body FROM notifications WHERE user_id = $1 AND type = 'REPORT_STATUS_CHANGED' AND ref_id = $2", reporter, id).Scan(&body); err != nil {
		t.Fatalf("reporter was not notified: %v", err)
	}
	if !strings.Contains(body, "DITERIMA") || !strings.Contains(body, "Teknisi dijadwalkan") {
		t.Errorf("notification body = %q", body)
	}

	// Only the approver of the room type changes status; others are refused.
	for name, tok := range map[string]string{
		"reporter": e.login(mahasiswaA).accessToken, "OB": e.login(obID).accessToken,
		"ADMIN": e.adminToken(), "KALAB": e.kalabToken(), // neither approves a KELAS room (BR-10)
	} {
		if res := e.patchReport(tok, id, map[string]string{"status": "DIPROSES"}); res.Code != http.StatusForbidden {
			t.Errorf("%s: %d; want 403", name, res.Code)
		}
	}
	if res := e.patchReport(tu, "00000000-0000-4000-8000-0000000000aa", map[string]string{"status": "DITERIMA"}); res.Code != http.StatusNotFound {
		t.Errorf("missing report: %d; want 404", res.Code)
	}
	if res := e.patchReport(tu, "nope", map[string]string{"status": "DITERIMA"}); res.Code != http.StatusNotFound {
		t.Errorf("malformed id: %d; want 404", res.Code)
	}
}

func TestReportNotifications(t *testing.T) {
	e := newEnv(t)
	other := e.addTU("198505052010012004")
	gone := e.addTU("198505052010012005")
	mustStatus(t, e.do("PATCH", "/api/v1/users/"+gone, map[string]any{"is_active": false}, bearer(e.adminToken())), http.StatusOK)
	tuUserID, studentID, adminUserID, kalabUserID := e.userIDOf(tuID), e.userIDOf(mahasiswaA), e.userIDOf(adminID), e.userIDOf(kalabID)
	room := e.freshRoom()

	long := strings.Repeat("kerusakan ", 30)
	res := e.postReport(e.login(mahasiswaA).accessToken, room, long, nil)
	mustStatus(t, res, http.StatusCreated)
	for name, who := range map[string]string{"first TU": tuUserID, "second TU": other} {
		if n := e.notificationCount(who, "REPORT_CREATED"); n != 1 {
			t.Errorf("%s got %d REPORT_CREATED; want 1", name, n)
		}
	}
	for name, who := range map[string]string{"deactivated TU": gone, "ADMIN": adminUserID, "KALAB": kalabUserID, "reporter": studentID} {
		if n := e.notificationCount(who, "REPORT_CREATED"); n != 0 {
			t.Errorf("%s got %d REPORT_CREATED; want 0", name, n)
		}
	}
	var refType, refID, body string
	if err := e.pool.QueryRow(t.Context(), "SELECT ref_type, ref_id::text, body FROM notifications WHERE user_id = $1 AND type = 'REPORT_CREATED'", tuUserID).Scan(&refType, &refID, &body); err != nil {
		t.Fatal(err)
	}
	if refType != "REPORT" || refID != idOf(res) || !strings.Contains(body, "…") || len([]rune(body)) > 200 {
		t.Errorf("notification = %s %s %q; want REPORT ref and a shortened body", refType, refID, body)
	}

	// A TU who files a report tells the other approver but not themselves.
	before := e.notificationCount(tuUserID, "REPORT_CREATED")
	mustStatus(t, e.postReport(e.tuToken(), room, "Kursi patah", nil), http.StatusCreated)
	if n := e.notificationCount(tuUserID, "REPORT_CREATED"); n != before {
		t.Errorf("reporting notified the reporter themselves")
	}
	if n := e.notificationCount(other, "REPORT_CREATED"); n != 2 {
		t.Errorf("second TU has %d REPORT_CREATED; want 2", n)
	}

	// ...and handling their own report does not notify them either.
	own := idOf(e.postReport(e.tuToken(), room, "Lampu mati", nil))
	notes := e.countRows("SELECT count(*) FROM notifications")
	mustStatus(t, e.patchReport(e.tuToken(), own, map[string]string{"status": "DITERIMA"}), http.StatusOK)
	if after := e.countRows("SELECT count(*) FROM notifications"); after != notes {
		t.Errorf("a TU updating their own report created %d notifications", after-notes)
	}

	// Who is told follows the type of the room (BR-10): the KALAB for a LAB, the TU and the ADMIN for a special room.
	createdFor := func(roomID, desc string) map[string]int {
		t.Helper()
		rep := idOf(e.postReport(e.login(mahasiswaA).accessToken, roomID, desc, nil))
		out := map[string]int{}
		for name, who := range map[string]string{"TU": tuUserID, "second TU": other, "deactivated TU": gone, "KALAB": kalabUserID, "ADMIN": adminUserID, "OB": e.userIDOf(obID)} {
			out[name] = e.countRows("SELECT count(*) FROM notifications WHERE user_id = $1 AND type = 'REPORT_CREATED' AND ref_id = $2", who, rep)
		}
		return out
	}
	for _, tc := range []struct {
		typ  string
		want map[string]int
	}{
		{"LAB", map[string]int{"KALAB": 1}},
		{"AUDITORIUM", map[string]int{"TU": 1, "second TU": 1, "ADMIN": 1}},
		{"RUANG_UJIAN", map[string]int{"TU": 1, "second TU": 1, "ADMIN": 1}},
		{"RUANG_SIDANG", map[string]int{"TU": 1, "second TU": 1, "ADMIN": 1}},
	} {
		got := createdFor(e.insertRoom("RN-"+tc.typ, tc.typ, 20), "Rusak di "+tc.typ)
		for name, n := range got {
			if n != tc.want[name] {
				t.Errorf("%s report: %s got %d REPORT_CREATED; want %d", tc.typ, name, n, tc.want[name])
			}
		}
	}
}

func TestReportStatusConcurrentUpdates(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	id := e.reportAt("BARU", mahasiswaA)

	results := make([]response, 8)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = e.patchReport(tu, id, map[string]string{"status": "DITERIMA"})
		}()
	}
	wg.Wait()

	ok := 0
	for _, res := range results {
		switch {
		case res.Code == http.StatusOK:
			ok++
		case res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE":
			t.Errorf("parallel update: %d %s", res.Code, res.Body.String())
		}
	}
	if ok != 1 {
		t.Errorf("%d updates succeeded; want 1", ok)
	}
	if n := e.reportEventCount(id); n != 2 {
		t.Errorf("events = %d; want 2 (created, accepted)", n)
	}
	if n := e.notificationCount(e.userIDOf(mahasiswaA), "REPORT_STATUS_CHANGED"); n != 1 {
		t.Errorf("status notifications = %d; want 1", n)
	}
}

func TestReportEventsAreAppendOnly(t *testing.T) {
	e := newEnv(t)
	id := e.reportAt("DIPROSES", mahasiswaA)
	before := e.reportEventCount(id)

	for name, stmt := range map[string]string{
		"update note":   "UPDATE report_events SET note = 'diubah'",
		"update status": "UPDATE report_events SET to_status = 'SELESAI'",
		"delete one":    "DELETE FROM report_events WHERE report_id = '" + id + "'",
		"delete all":    "DELETE FROM report_events",
	} {
		_, err := e.pool.Exec(t.Context(), stmt)
		if err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s: err = %v; want the append-only trigger to refuse", name, err)
		}
	}
	if n := e.reportEventCount(id); n != before {
		t.Errorf("events %d -> %d after tampering attempts", before, n)
	}
}

// A report, its history, its notification and its photo file succeed or fail together.
func TestReportWritesAreAtomic(t *testing.T) {
	e := newEnv(t)
	student := e.login(mahasiswaA).accessToken
	tu := e.tuToken()
	room := e.freshRoom()

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
	create := func() response {
		_, _, rec := e.multipartReport(student, map[string]string{"room_id": room, "description": "Rusak"}, &photoPart{"a.jpg", "image/jpeg", jpegBytes(100)})
		return response{rec}
	}

	// Creating: a failing notification insert leaves no report, no event and no file.
	breakTable("notifications")
	if res := create(); res.Code != http.StatusInternalServerError || strings.Contains(res.Body.String(), "injected") {
		t.Fatalf("create with failing notification: %d %s; want a generic 500", res.Code, res.Body.String())
	}
	if r, ev, files := e.countRows("SELECT count(*) FROM damage_reports"), e.countRows("SELECT count(*) FROM report_events"), e.uploadedFiles(); r != 0 || ev != 0 || len(files) != 0 {
		t.Errorf("after a failed create: %d reports, %d events, files %v; want none", r, ev, files)
	}
	repair("notifications")

	// Updating: a failing notification insert leaves status and history untouched.
	id := e.reportAt("BARU", mahasiswaA)
	events, notes := e.reportEventCount(id), e.countRows("SELECT count(*) FROM notifications")
	breakTable("notifications")
	if res := e.doUnchecked("PATCH", "/api/v1/reports/"+id+"/status", map[string]string{"status": "DITERIMA"}, bearer(tu)); res.Code != http.StatusInternalServerError {
		t.Fatalf("update with failing notification: %d; want 500", res.Code)
	}
	if e.reportStatus(id) != "BARU" || e.reportEventCount(id) != events || e.countRows("SELECT count(*) FROM notifications") != notes {
		t.Error("a failed update left traces")
	}
	repair("notifications")

	// ...and so does a failing event insert.
	breakTable("report_events")
	if res := e.doUnchecked("PATCH", "/api/v1/reports/"+id+"/status", map[string]string{"status": "DITERIMA"}, bearer(tu)); res.Code != http.StatusInternalServerError {
		t.Fatalf("update with failing event: %d; want 500", res.Code)
	}
	if e.reportStatus(id) != "BARU" || e.countRows("SELECT count(*) FROM notifications") != notes {
		t.Error("a failed event insert left a status change or notification behind")
	}
	repair("report_events")

	mustStatus(t, e.patchReport(tu, id, map[string]string{"status": "DITERIMA"}), http.StatusOK)
}

// If the photo cannot be written, the report is not created.
func TestReportIsNotCreatedWhenThePhotoCannotBeStored(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := newEnv(t, func(c *config.Config) { c.UploadDir = blocker }) // MkdirAll under a regular file fails
	room := e.freshRoom()
	tok := e.login(mahasiswaA).accessToken

	_, _, rec := e.multipartReport(tok, map[string]string{"room_id": room, "description": "Rusak"}, &photoPart{"a.jpg", "image/jpeg", jpegBytes(100)})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want 500", rec.Code)
	}
	if n := e.countRows("SELECT count(*) FROM damage_reports"); n != 0 {
		t.Errorf("%d reports stored although the photo was lost", n)
	}
	// Reports without a photo do not touch the disk and still work.
	mustStatus(t, e.postReport(tok, room, "Tanpa foto", nil), http.StatusCreated)
}

// Phones put the GPS position of a photo into its EXIF block; it must not be stored or served. Uploads are
// decoded and written again as JPEG, so nothing but pixels survives (BR-16).
func TestReportPhotoMetadataIsStripped(t *testing.T) {
	e := newEnv(t)
	tok := e.login(mahasiswaA).accessToken
	const secret = "GPS-LAT-6.2088-LON-106.8456"

	// EXIF: big-endian TIFF, one Orientation entry (6 = rotated), the "GPS data" after it.
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0, 0, 0, 0, 0}
	payload := append(append([]byte("Exif\x00\x00"), tiff...), secret...)
	segment := append([]byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}, payload...)
	plain := jpegBytes(0)
	withGPS := append(append(append([]byte{}, plain[:2]...), segment...), plain[2:]...)
	// A payload hidden after the end of the picture must go too.
	withGPS = append(withGPS, []byte("HIDDEN-PAYLOAD-"+secret)...)

	res := e.postReport(tok, e.freshRoom(), "Rusak", &photoPart{"IMG_0001.jpg", "image/jpeg", withGPS})
	mustStatus(t, res, http.StatusCreated)
	id, _ := res.json()["attachment_id"].(string)

	served := e.do("GET", "/api/v1/attachments/"+id, nil, bearer(tok))
	mustStatus(t, served, http.StatusOK)
	if served.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("content type = %q; want image/jpeg", served.Header().Get("Content-Type"))
	}
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
	img, err := jpeg.Decode(bytes.NewReader(served.Body.Bytes()))
	if err != nil {
		t.Fatalf("the served photo no longer decodes: %v", err)
	}
	// Orientation 6 turns the picture on its side; the pixels are rotated and the tag is gone. The fixture is
	// square (8x8), so only the absence of the tag can be checked here; the unit tests of internal/attachments
	// check that a non-square picture comes out the right way round.
	if b := img.Bounds(); b.Dx() != 8 || b.Dy() != 8 {
		t.Errorf("served photo is %dx%d; want 8x8", b.Dx(), b.Dy())
	}
}

func TestReportPhotoWithBrokenStructureIsRefused(t *testing.T) {
	e := newEnv(t)
	tok := e.login(mahasiswaA).accessToken
	good := jpegBytes(0)

	for name, photo := range map[string]*photoPart{
		"jpeg cut before the scan":      {"a.jpg", "image/jpeg", good[:12]},
		"jpeg with garbage after start": {"a.jpg", "image/jpeg", []byte{0xFF, 0xD8, 0xFF, 0x12, 0x34, 0x56, 0x78}},
		"png cut in half":               {"a.png", "image/png", pngBytes(0)[:20]},
		"webp claiming a larger size":   {"a.webp", "image/webp", append([]byte("RIFF\xFF\x00\x00\x00WEBP"), 'V', 'P', '8', 'L', 4, 0, 0, 0, 1, 2, 3, 4)},
	} {
		res := e.postReport(tok, e.freshRoom(), "Rusak", photo)
		if res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)["photo"] {
			t.Errorf("%s: %d %s; want 422 naming photo", name, res.Code, res.Body.String())
		}
	}
	if files := e.uploadedFiles(); len(files) != 0 {
		t.Errorf("refused uploads left files: %v", files)
	}
}
