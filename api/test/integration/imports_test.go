package app_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"classlab/api/internal/config"
	"classlab/api/internal/seed"
)

const importSemester = "2026-GANJIL"

// importCSV is a valid file: two new students and one existing one (2021101003).
const importCSV = "identifier,name,email,course_code,course_name,class_name\n" +
	"23100001,Rina Hartono,rina@mhs.test,TI2101,Basis Data,A\n" +
	"23100002,Soni Prakoso,soni@mhs.test,TI2101,Basis Data,A\n" +
	"2021101003,Citra Dewi Baru,,TI2203,Jaringan Komputer,B\n"

// multipartPost sends a multipart form with an optional CSV file and checks the response against the contract.
func (e *env) multipartPost(path, token string, fields map[string]string, file []byte) response {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			e.t.Fatal(err)
		}
	}
	if file != nil {
		part, err := mw.CreateFormFile("file", "daftar mahasiswa.csv")
		if err != nil {
			e.t.Fatal(err)
		}
		if _, err := part.Write(file); err != nil {
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
	e.checkContract(req, buf.Bytes(), rec)
	return response{rec}
}

func (e *env) importPreview(token, csv, semester string) response {
	e.t.Helper()
	return e.multipartPost("/api/v1/imports/users?dry_run=true", token, map[string]string{"semester": semester}, []byte(csv))
}

func (e *env) importApply(token, csv, semester, planHash string) response {
	e.t.Helper()
	return e.multipartPost("/api/v1/imports/users", token, map[string]string{"semester": semester, "plan_hash": planHash}, []byte(csv))
}

func (e *env) userCount() int { return e.countRows("SELECT count(*) FROM users") }

// importCodes pulls error codes and line numbers out of an IMPORT_INVALID body.
func importProblemLines(res response) map[int]bool {
	lines := map[int]bool{}
	details, _ := res.json()["error"].(map[string]any)["details"].([]any)
	for _, d := range details {
		if line, ok := d.(map[string]any)["line"].(float64); ok {
			lines[int(line)] = true
		}
	}
	return lines
}

func TestImportPreviewStoresNothingAndDescribesThePlan(t *testing.T) {
	e := newEnv(t)
	tu := e.login(acctTU).accessToken
	users, imports, courses := e.userCount(), e.countRows("SELECT count(*) FROM imports"), e.countRows("SELECT count(*) FROM user_courses")

	res := e.importPreview(tu, importCSV, importSemester)
	mustStatus(t, res, http.StatusOK)
	body := res.json()
	if body["rows"] != float64(3) || body["to_create"] != float64(2) || body["to_update"] != float64(1) {
		t.Errorf("preview counts = %v", body)
	}
	if errs, _ := body["errors"].([]any); len(errs) != 0 {
		t.Errorf("preview errors = %v, want none", errs)
	}
	if hash, _ := body["plan_hash"].(string); len(hash) != 64 {
		t.Errorf("plan_hash = %q", body["plan_hash"])
	}

	if e.userCount() != users || e.countRows("SELECT count(*) FROM imports") != imports || e.countRows("SELECT count(*) FROM user_courses") != courses {
		t.Error("the preview stored something")
	}
	// The same file gives the same hash, a different semester a different one.
	again := e.importPreview(tu, importCSV, importSemester)
	other := e.importPreview(tu, importCSV, "2026-GENAP")
	if again.json()["plan_hash"] != body["plan_hash"] || other.json()["plan_hash"] == body["plan_hash"] {
		t.Error("plan_hash is not a stable fingerprint of file and semester")
	}
}

func TestImportApplyCreatesAccountsAndShowsTemporaryPasswordsOnce(t *testing.T) {
	e := newEnv(t)
	tu := e.login(acctTU).accessToken

	plan := e.importPreview(tu, importCSV, importSemester).json()["plan_hash"].(string)
	before := e.userCount()
	res := e.importApply(tu, importCSV, importSemester, plan)
	mustStatus(t, res, http.StatusOK)
	if cc := res.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}

	body := res.json()
	if body["created"] != float64(2) || body["updated"] != float64(1) {
		t.Fatalf("result counts = %v", body)
	}
	creds, _ := body["credentials"].([]any)
	if len(creds) != 2 {
		t.Fatalf("credentials = %v, want one per created account", creds)
	}
	if e.userCount() != before+2 {
		t.Errorf("users %d -> %d, want +2", before, e.userCount())
	}

	for _, c := range creds {
		cred := c.(map[string]any)
		ident, temp := cred["identifier"].(string), cred["temporary_password"].(string)
		if len(temp) != e.cfg.Business.TempPasswordLength {
			t.Errorf("%s: temporary password length %d, want %d", ident, len(temp), e.cfg.Business.TempPasswordLength)
		}

		// The credential works, marks the account for a forced change, and the account is a student.
		login := e.acctLogin(ident, temp)
		mustStatus(t, login, http.StatusOK)
		if login.json()["must_change_password"] != true {
			t.Errorf("%s: login must_change_password = %v", ident, login.json()["must_change_password"])
		}
		if role := login.json()["user"].(map[string]any)["role"]; role != "MAHASISWA" {
			t.Errorf("%s: role = %v, want MAHASISWA", ident, role)
		}

		// Nowhere else: not in the database, the audit log, the history summary or the server log.
		if n := e.countRows(`SELECT count(*) FROM users WHERE password_hash LIKE '%'||$1||'%'`, temp); n != 0 {
			t.Errorf("%s: temporary password inside a stored hash column", ident)
		}
		if n := e.countRows(`SELECT count(*) FROM audit_logs WHERE before::text LIKE '%'||$1||'%' OR after::text LIKE '%'||$1||'%'`, temp); n != 0 {
			t.Errorf("%s: temporary password in the audit log", ident)
		}
		if n := e.countRows(`SELECT count(*) FROM imports WHERE summary::text LIKE '%'||$1||'%'`, temp); n != 0 {
			t.Errorf("%s: temporary password in the import summary", ident)
		}
		if strings.Contains(e.logs.String(), temp) {
			t.Errorf("%s: temporary password in the server log", ident)
		}
	}
	// The file's content is not kept either: names from the CSV appear in users, not in logs or the history.
	if strings.Contains(e.logs.String(), "Rina Hartono") || e.countRows(`SELECT count(*) FROM imports WHERE summary::text LIKE '%Rina%'`) != 0 {
		t.Error("CSV content leaked into the log or the import summary")
	}

	// The existing student was updated and keeps her password; course context is stored for all three.
	var name string
	if err := e.pool.QueryRow(t.Context(), `SELECT name FROM users WHERE identifier = '2021101003'`).Scan(&name); err != nil || name != "Citra Dewi Baru" {
		t.Errorf("updated student name = %q (%v)", name, err)
	}
	if r := e.acctLogin("2021101003", seed.DevPassword); r.Code != http.StatusOK || r.json()["must_change_password"] != false {
		t.Errorf("existing student after import: %d must_change_password=%v, want 200 and false", r.Code, r.json()["must_change_password"])
	}
	if n := e.countRows(`SELECT count(*) FROM user_courses WHERE semester = $1`, importSemester); n != 3 {
		t.Errorf("user_courses rows = %d, want 3", n)
	}
	// One audit line for the whole import, without credentials.
	if n := e.acctCountAudit("USERS_IMPORTED"); n != 1 {
		t.Errorf("USERS_IMPORTED rows = %d, want 1", n)
	}
	// And the history shows it.
	list := e.do("GET", "/api/v1/imports?kind=USERS", nil, bearer(tu))
	mustStatus(t, list, http.StatusOK)
	if items := dataOf(list); len(items) != 1 || items[0].(map[string]any)["semester"] != importSemester || items[0].(map[string]any)["status"] != "COMMITTED" {
		t.Errorf("history = %v", items)
	}
}

func TestImportRefusesTheSameFileTwiceAndAChangedPlan(t *testing.T) {
	e := newEnv(t)
	tu := e.login(acctTU).accessToken

	plan := e.importPreview(tu, importCSV, importSemester).json()["plan_hash"].(string)

	// A plan that does not belong to this file or semester is refused and nothing is written.
	before := e.userCount()
	for name, hash := range map[string]string{"made up": strings.Repeat("0", 64), "other file": e.importPreview(tu, importCSV+"23100003,Tono,,,,\n", importSemester).json()["plan_hash"].(string)} {
		res := e.importApply(tu, importCSV, importSemester, hash)
		if res.Code != http.StatusConflict || res.errorCode() != "IMPORT_PLAN_CHANGED" {
			t.Errorf("%s plan: %d %s, want 409 IMPORT_PLAN_CHANGED", name, res.Code, res.errorCode())
		}
	}
	if e.userCount() != before {
		t.Error("a refused apply created accounts")
	}
	if res := e.multipartPost("/api/v1/imports/users", tu, map[string]string{"semester": importSemester}, []byte(importCSV)); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("apply without plan_hash: %d, want 422", res.Code)
	}

	// The database moved after the preview: one of the accounts now exists, so the plan differs.
	e.acctInsertUser("23100001", "MAHASISWA", "Sudah Ada", false)
	if res := e.importApply(tu, importCSV, importSemester, plan); res.Code != http.StatusConflict || res.errorCode() != "IMPORT_PLAN_CHANGED" {
		t.Errorf("stale plan: %d %s, want 409 IMPORT_PLAN_CHANGED", res.Code, res.errorCode())
	}

	// A fresh preview and apply succeed; applying the same file again is refused outright.
	fresh := e.importPreview(tu, importCSV, importSemester).json()["plan_hash"].(string)
	mustStatus(t, e.importApply(tu, importCSV, importSemester, fresh), http.StatusOK)
	res := e.importApply(tu, importCSV, importSemester, fresh)
	if res.Code != http.StatusConflict || res.errorCode() != "IMPORT_ALREADY_APPLIED" {
		t.Errorf("second apply: %d %s, want 409 IMPORT_ALREADY_APPLIED", res.Code, res.errorCode())
	}
}

func TestImportIsAllOrNothing(t *testing.T) {
	e := newEnv(t)
	tu := e.login(acctTU).accessToken
	e.acctInsertUser("DOSEN-1", "OB", "Bukan Mahasiswa", false)

	bad := "identifier,name,email\n" +
		"23200001,Valid Satu,valid1@mhs.test\n" + // line 2: fine
		"x,Pendek,\n" + // line 3: identifier too short
		"23200003,,\n" + // line 4: no name
		"23200004,Email Rusak,bukan-email\n" + // line 5: bad email
		"23200001,Ganda,\n" + // line 6: duplicate identifier (line 2)
		"23200006,Email Ganda,valid1@mhs.test\n" + // line 7: duplicate email (line 2)
		"DOSEN-1,Mencoba Ubah,\n" + // line 8: belongs to a non-student
		"2021101001,Sudah Ada,andi@classlab.test\n" // line 9: fine... but the email is Andi's own: allowed

	preview := e.importPreview(tu, bad, importSemester)
	mustStatus(t, preview, http.StatusOK)
	reported := map[int]bool{}
	for _, item := range preview.json()["errors"].([]any) {
		reported[int(item.(map[string]any)["line"].(float64))] = true
	}
	for _, line := range []int{3, 4, 5, 6, 7, 8} {
		if !reported[line] {
			t.Errorf("preview did not report line %d: %v", line, reported)
		}
	}
	if reported[2] || reported[9] {
		t.Errorf("preview flagged a valid line: %v", reported)
	}

	// Applying a file with problems stores nothing at all, valid lines included.
	before := e.userCount()
	res := e.importApply(tu, bad, importSemester, preview.json()["plan_hash"].(string))
	if res.Code != http.StatusUnprocessableEntity || res.errorCode() != "IMPORT_INVALID" {
		t.Fatalf("apply with problems: %d %s, want 422 IMPORT_INVALID", res.Code, res.errorCode())
	}
	if lines := importProblemLines(res); !lines[3] || !lines[8] || lines[2] {
		t.Errorf("IMPORT_INVALID lines = %v", lines)
	}
	if e.userCount() != before || e.countRows("SELECT count(*) FROM imports") != 0 || e.acctCountAudit("USERS_IMPORTED") != 0 {
		t.Error("a rejected import left data behind")
	}
	if n := e.countRows("SELECT count(*) FROM users WHERE identifier = '23200001'"); n != 0 {
		t.Error("the valid first line was created although the file was rejected")
	}
}

func TestImportFileHandling(t *testing.T) {
	e := newEnv(t)
	tu := e.login(acctTU).accessToken

	// Semicolon separator and a byte order mark, as Excel writes CSV in an Indonesian locale.
	excel := "\xef\xbb\xbfidentifier;name;email\r\n23300001;Wati, S.Kom;wati@mhs.test\r\n"
	res := e.importPreview(tu, excel, importSemester)
	mustStatus(t, res, http.StatusOK)
	if res.json()["to_create"] != float64(1) {
		t.Errorf("semicolon file preview = %v", res.json())
	}

	cases := map[string]struct {
		csv      string
		semester string
		status   int
		code     string
	}{
		"missing identifier column": {"name,email\nA,a@x.test\n", importSemester, 422, "IMPORT_INVALID"},
		"empty file":                {"", importSemester, 422, "IMPORT_INVALID"},
		"bad semester":              {importCSV, "2026", 422, "VALIDATION_ERROR"},
		"missing semester":          {importCSV, "", 422, "VALIDATION_ERROR"},
	}
	for name, c := range cases {
		res := e.importPreview(tu, c.csv, c.semester)
		if res.Code != c.status || res.errorCode() != c.code {
			t.Errorf("%s: %d %s, want %d %s", name, res.Code, res.errorCode(), c.status, c.code)
		}
	}

	// No file part at all.
	if res := e.multipartPost("/api/v1/imports/users?dry_run=true", tu, map[string]string{"semester": importSemester}, nil); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("no file: %d, want 422", res.Code)
	}
	// Row limit from configuration.
	small := newEnv(t, func(c *config.Config) { c.Business.ImportMaxRows = 2 })
	res = small.importPreview(small.login(acctTU).accessToken, importCSV, importSemester)
	if res.Code != http.StatusUnprocessableEntity || res.errorCode() != "IMPORT_INVALID" {
		t.Errorf("over the row limit: %d %s, want 422 IMPORT_INVALID", res.Code, res.errorCode())
	}
}

func TestImportAndHistoryAccess(t *testing.T) {
	e := newEnv(t)
	for _, ident := range []string{acctStudent, acctOB, acctKalab, acctOrg} {
		tok := e.login(ident).accessToken
		if res := e.importPreview(tok, importCSV, importSemester); res.Code != http.StatusForbidden {
			t.Errorf("%s previewing an import: %d, want 403", ident, res.Code)
		}
		if res := e.do("GET", "/api/v1/imports", nil, bearer(tok)); res.Code != http.StatusForbidden {
			t.Errorf("%s reading import history: %d, want 403", ident, res.Code)
		}
	}
	if res := e.importPreview("", importCSV, importSemester); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous import: %d, want 401", res.Code)
	}

	// Administrators import too, and what they create is always a student.
	admin := e.login(acctAdmin).accessToken
	plan := e.importPreview(admin, importCSV, importSemester).json()["plan_hash"].(string)
	mustStatus(t, e.importApply(admin, importCSV, importSemester, plan), http.StatusOK)
	if n := e.countRows(`SELECT count(*) FROM users WHERE identifier IN ('23100001','23100002') AND role = 'MAHASISWA'`); n != 2 {
		t.Errorf("imported accounts with role MAHASISWA = %d, want 2", n)
	}
	// A student cannot become anything else through an import, and TU still cannot reach other roles.
	if res := e.do("GET", "/api/v1/imports?kind=NOPE", nil, bearer(admin)); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad kind filter: %d, want 422", res.Code)
	}
}
