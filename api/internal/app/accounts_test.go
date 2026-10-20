package app_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/seed"
)

// Seed accounts (api/internal/seed/seed.go). The acct prefix keeps these names clear of other test files.
const (
	acctAdmin   = "197501012000011001"
	acctTU      = "198203152008012002"
	acctKalab   = "197805202005011003"
	acctOB      = "199001102015011003"
	acctStudent = "2021101001"
	acctOrg     = "HIMA-TI"

	acctNewPassword = "Passw0rd-baru-2026"
)

func (e *env) acctLogin(identifier, pw string) response {
	e.t.Helper()
	return e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": identifier, "password": pw})
}

func (e *env) acctChangePassword(s session, current, next string) response {
	e.t.Helper()
	return e.do("POST", "/api/v1/auth/change-password",
		map[string]string{"current_password": current, "new_password": next},
		bearer(s.accessToken), refreshCookie(s.refresh))
}

// acctInsertUser creates an account directly in the database with the seed password.
func (e *env) acctInsertUser(identifier, role, name string, mustChange bool) string {
	e.t.Helper()
	var picName, picPhone *string
	if role == "ORGANISASI" {
		n, p := "Penanggung Jawab", "081234567890"
		picName, picPhone = &n, &p
	}
	var id string
	err := e.pool.QueryRow(e.t.Context(),
		`INSERT INTO users (org_unit_id, identifier, name, password_hash, role, must_change_password, pic_name, pic_phone)
		 VALUES ($1, $2, $3, (SELECT password_hash FROM users WHERE identifier = $4), $5::user_role, $6, $7, $8)
		 RETURNING id`,
		e.orgUnitID(), identifier, name, acctStudent, role, mustChange, picName, picPhone).Scan(&id)
	if err != nil {
		e.t.Fatalf("inserting user %s: %v", identifier, err)
	}
	return id
}

func (e *env) acctCountAudit(action string) int {
	e.t.Helper()
	return e.countRows("SELECT count(*) FROM audit_logs WHERE action = $1", action)
}

func TestChangePasswordReplacesThePasswordAndEndsOtherSessions(t *testing.T) {
	e := newEnv(t)
	here := e.login(acctStudent)
	elsewhere := e.login(acctStudent)

	mustStatus(t, e.acctChangePassword(here, seed.DevPassword, acctNewPassword), http.StatusNoContent)

	if r := e.acctLogin(acctStudent, seed.DevPassword); r.Code != http.StatusUnauthorized {
		t.Errorf("old password: status %d, want 401", r.Code)
	}
	if r := e.acctLogin(acctStudent, acctNewPassword); r.Code != http.StatusOK {
		t.Errorf("new password: status %d, want 200", r.Code)
	}
	// The session that made the change survives; the other device is signed out.
	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(elsewhere.refresh)); r.Code != http.StatusUnauthorized {
		t.Errorf("other session refresh: status %d, want 401", r.Code)
	}
	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(here.refresh)); r.Code != http.StatusOK {
		t.Errorf("own session refresh: status %d, want 200", r.Code)
	}
}

func TestChangePasswordValidation(t *testing.T) {
	e := newEnv(t)
	s := e.login(acctStudent)

	cases := []struct {
		name          string
		current, next string
		field         string
	}{
		{"wrong current password", "bukan-passwordnya", acctNewPassword, "current_password"},
		{"empty current password", "", acctNewPassword, "current_password"},
		{"new password too short", seed.DevPassword, "pendek", "new_password"},
		{"new password equals current", seed.DevPassword, seed.DevPassword, "new_password"},
	}
	for _, c := range cases {
		res := e.acctChangePassword(s, c.current, c.next)
		if res.Code != http.StatusUnprocessableEntity || res.errorCode() != "VALIDATION_ERROR" {
			t.Errorf("%s: %d %s, want 422 VALIDATION_ERROR", c.name, res.Code, res.Body.String())
			continue
		}
		if !strings.Contains(res.Body.String(), `"field":"`+c.field+`"`) {
			t.Errorf("%s: body does not name %s: %s", c.name, c.field, res.Body.String())
		}
	}

	// A refused change leaves the old password working.
	if r := e.acctLogin(acctStudent, seed.DevPassword); r.Code != http.StatusOK {
		t.Errorf("password changed by a refused request: login status %d", r.Code)
	}
	if r := e.do("POST", "/api/v1/auth/change-password", nil); r.Code != http.StatusUnauthorized {
		t.Errorf("anonymous change-password: %d, want 401", r.Code)
	}
}

func TestChangePasswordGuessingIsRateLimited(t *testing.T) {
	e := newEnv(t)
	s := e.login(acctStudent)

	for i := 0; i < 5; i++ {
		if r := e.acctChangePassword(s, "tebakan-salah", acctNewPassword); r.Code != http.StatusUnprocessableEntity {
			t.Fatalf("guess %d: %d, want 422", i+1, r.Code)
		}
	}
	// The sixth attempt is refused even with the right password.
	res := e.acctChangePassword(s, seed.DevPassword, acctNewPassword)
	if res.Code != http.StatusTooManyRequests || res.Header().Get("Retry-After") == "" {
		t.Errorf("sixth attempt: %d retry-after=%q, want 429 with Retry-After", res.Code, res.Header().Get("Retry-After"))
	}
}

func TestTemporaryPasswordBlocksEverythingButPasswordChange(t *testing.T) {
	e := newEnv(t)
	admin := e.login(acctAdmin)

	created := e.do("POST", "/api/v1/users", map[string]any{
		"org_unit_id": e.orgUnitID(), "identifier": "MHSBARU1", "name": "Mahasiswa Baru",
		"password": "sementara-12345", "role": "MAHASISWA",
	}, bearer(admin.accessToken))
	mustStatus(t, created, http.StatusCreated)
	if created.json()["must_change_password"] != true {
		t.Errorf("created account must_change_password = %v, want true", created.json()["must_change_password"])
	}

	login := e.acctLogin("MHSBARU1", "sementara-12345")
	mustStatus(t, login, http.StatusOK)
	if login.json()["must_change_password"] != true {
		t.Errorf("login must_change_password = %v, want true", login.json()["must_change_password"])
	}
	token, _ := login.json()["access_token"].(string)
	cookie := login.cookie("refresh_token")
	if cookie == nil {
		t.Fatal("no refresh cookie")
	}

	for _, path := range []string{"/api/v1/rooms", "/api/v1/bookings", "/api/v1/notifications", "/api/v1/reports"} {
		res := e.do("GET", path, nil, bearer(token))
		if res.Code != http.StatusForbidden || res.errorCode() != "PASSWORD_CHANGE_REQUIRED" {
			t.Errorf("GET %s: %d %s, want 403 PASSWORD_CHANGE_REQUIRED", path, res.Code, res.errorCode())
		}
	}
	// The three routes that stay open.
	me := e.do("GET", "/api/v1/auth/me", nil, bearer(token))
	mustStatus(t, me, http.StatusOK)
	if me.json()["must_change_password"] != true {
		t.Errorf("/auth/me must_change_password = %v, want true", me.json()["must_change_password"])
	}

	s := session{accessToken: token, refresh: cookie.Value}
	mustStatus(t, e.acctChangePassword(s, "sementara-12345", acctNewPassword), http.StatusNoContent)
	mustStatus(t, e.do("GET", "/api/v1/rooms", nil, bearer(token)), http.StatusOK)
	mustStatus(t, e.do("POST", "/api/v1/auth/logout", nil, bearer(token)), http.StatusOK)
}

func TestMeReportsActivePointsForRequesters(t *testing.T) {
	e := newEnv(t)
	student := e.do("GET", "/api/v1/auth/me", nil, bearer(e.login(acctStudent).accessToken))
	mustStatus(t, student, http.StatusOK)
	if got, ok := student.json()["active_points"].(float64); !ok || got != 0 {
		t.Errorf("student active_points = %v, want 0", student.json()["active_points"])
	}
	if _, present := student.json()["booking_blocked_until"]; !present {
		t.Error("student booking_blocked_until missing")
	}

	tu := e.do("GET", "/api/v1/auth/me", nil, bearer(e.login(acctTU).accessToken))
	mustStatus(t, tu, http.StatusOK)
	if _, present := tu.json()["active_points"]; present {
		t.Error("TU /auth/me carries active_points; it is for requesters only")
	}
}

func TestOrganisationAccountsNeedAContactPerson(t *testing.T) {
	e := newEnv(t)
	admin := bearer(e.login(acctAdmin).accessToken)
	body := func(extra map[string]any) map[string]any {
		m := map[string]any{"org_unit_id": e.orgUnitID(), "identifier": "ORG-X", "name": "Himpunan X", "password": "sementara-12345", "role": "ORGANISASI"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}

	res := e.do("POST", "/api/v1/users", body(nil), admin)
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "pic_name") || !strings.Contains(res.Body.String(), "pic_phone") {
		t.Errorf("organisation without contact: %d %s, want 422 naming pic_name and pic_phone", res.Code, res.Body.String())
	}

	ok := e.do("POST", "/api/v1/users", body(map[string]any{"pic_name": "Rizky", "pic_phone": "081234567890"}), admin)
	mustStatus(t, ok, http.StatusCreated)
	if ok.json()["pic_name"] != "Rizky" || ok.json()["email"] != nil {
		t.Errorf("created organisation = %v", ok.json())
	}

	// Contact person data belongs to organisations only; it is refused, not silently dropped.
	res = e.do("POST", "/api/v1/users", map[string]any{
		"org_unit_id": e.orgUnitID(), "identifier": "MHS-PIC", "name": "Mahasiswa", "password": "sementara-12345",
		"role": "MAHASISWA", "pic_name": "Rizky",
	}, admin)
	if res.Code != http.StatusUnprocessableEntity {
		t.Errorf("student with pic_name: %d, want 422", res.Code)
	}

	// Email is optional, and two accounts without one do not collide.
	for _, id := range []string{"NOEMAIL-1", "NOEMAIL-2"} {
		mustStatus(t, e.do("POST", "/api/v1/users", map[string]any{
			"org_unit_id": e.orgUnitID(), "identifier": id, "name": id, "password": "sementara-12345", "role": "MAHASISWA",
		}, admin), http.StatusCreated)
	}
}

func TestTUManagesStudentAccountsOnly(t *testing.T) {
	e := newEnv(t)
	tu := bearer(e.login(acctTU).accessToken)
	adminID := e.userIDOf(acctAdmin)
	studentID := e.userIDOf(acctStudent)

	list := e.do("GET", "/api/v1/users?per_page=100", nil, tu)
	mustStatus(t, list, http.StatusOK)
	for _, item := range dataOf(list) {
		if role := item.(map[string]any)["role"]; role != "MAHASISWA" {
			t.Errorf("TU sees a %v account in the list", role)
		}
	}
	if res := e.do("GET", "/api/v1/users?role=ADMIN", nil, tu); res.Code != http.StatusForbidden {
		t.Errorf("TU listing ADMIN accounts: %d, want 403", res.Code)
	}
	if res := e.do("POST", "/api/v1/users", map[string]any{
		"org_unit_id": e.orgUnitID(), "identifier": "TU-MADE", "name": "X", "password": "sementara-12345", "role": "MAHASISWA",
	}, tu); res.Code != http.StatusForbidden {
		t.Errorf("TU creating an account: %d, want 403", res.Code)
	}

	// What TU may change on a student, and what it may not.
	mustStatus(t, e.do("PATCH", "/api/v1/users/"+studentID, map[string]any{"name": "Nama Baru"}, tu), http.StatusOK)
	for name, body := range map[string]map[string]any{
		"role":                 {"role": "OB"},
		"contact person":       {"pic_name": "X"},
		"clearing the block":   {"clear_booking_block": true},
		"role with a name too": {"name": "Y", "role": "ADMIN"},
	} {
		if res := e.do("PATCH", "/api/v1/users/"+studentID, body, tu); res.Code != http.StatusForbidden {
			t.Errorf("TU changing %s: %d, want 403", name, res.Code)
		}
	}
	if res := e.do("PATCH", "/api/v1/users/"+adminID, map[string]any{"name": "Dihack"}, tu); res.Code != http.StatusForbidden {
		t.Errorf("TU editing an administrator: %d, want 403", res.Code)
	}
	if res := e.do("POST", "/api/v1/users/"+adminID+"/reset-password", nil, tu); res.Code != http.StatusForbidden {
		t.Errorf("TU resetting an administrator's password: %d, want 403", res.Code)
	}
	if res := e.do("POST", "/api/v1/users/00000000-0000-0000-0000-000000000000/reset-password", nil, tu); res.Code != http.StatusNotFound {
		t.Errorf("TU resetting an unknown account: %d, want 404", res.Code)
	}

	// Deactivation works and ends the student's sessions.
	s := e.login("2021101002")
	mustStatus(t, e.do("PATCH", "/api/v1/users/"+s.userID, map[string]any{"is_active": false}, tu), http.StatusOK)
	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(s.refresh)); r.Code != http.StatusUnauthorized {
		t.Errorf("deactivated student refresh: %d, want 401", r.Code)
	}
	if r := e.acctLogin("2021101002", seed.DevPassword); r.Code != http.StatusUnauthorized {
		t.Errorf("deactivated student login: %d, want 401", r.Code)
	}

	// Other roles have no access to account management at all.
	for _, ident := range []string{acctStudent, acctOB, acctKalab} {
		if res := e.do("GET", "/api/v1/users", nil, bearer(e.login(ident).accessToken)); res.Code != http.StatusForbidden {
			t.Errorf("%s listing users: %d, want 403", ident, res.Code)
		}
	}
}

func TestResetPasswordIssuesATemporaryPasswordOnce(t *testing.T) {
	e := newEnv(t)
	admin := bearer(e.login(acctAdmin).accessToken)
	stu := e.login(acctStudent)

	res := e.do("POST", "/api/v1/users/"+stu.userID+"/reset-password", nil, admin)
	mustStatus(t, res, http.StatusOK)
	if cc := res.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	temp, _ := res.json()["temporary_password"].(string)
	if len(temp) != e.cfg.Business.TempPasswordLength {
		t.Fatalf("temporary password %q has length %d, want %d", temp, len(temp), e.cfg.Business.TempPasswordLength)
	}

	if r := e.acctLogin(acctStudent, seed.DevPassword); r.Code != http.StatusUnauthorized {
		t.Errorf("old password after reset: %d, want 401", r.Code)
	}
	login := e.acctLogin(acctStudent, temp)
	mustStatus(t, login, http.StatusOK)
	if login.json()["must_change_password"] != true {
		t.Error("login with a temporary password must report must_change_password")
	}
	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(stu.refresh)); r.Code != http.StatusUnauthorized {
		t.Errorf("session from before the reset: refresh %d, want 401", r.Code)
	}

	// The reset is recorded, but the password appears nowhere: not in the audit log, not in the logs.
	if n := e.acctCountAudit("USER_PASSWORD_RESET"); n != 1 {
		t.Errorf("USER_PASSWORD_RESET rows = %d, want 1", n)
	}
	if n := e.countRows(`SELECT count(*) FROM audit_logs WHERE before::text LIKE '%'||$1||'%' OR after::text LIKE '%'||$1||'%'`, temp); n != 0 {
		t.Errorf("temporary password found in %d audit rows", n)
	}
	if strings.Contains(e.logs.String(), temp) {
		t.Error("temporary password found in the server log")
	}
	if n := e.countRows(`SELECT count(*) FROM users WHERE password_hash = $1`, temp); n != 0 {
		t.Errorf("temporary password stored in clear for %d users", n)
	}
}

func TestUserChangesAreAuditedWithTheirActorAndOrigin(t *testing.T) {
	e := newEnv(t)
	admin := bearer(e.login(acctAdmin).accessToken)
	stu := e.userIDOf(acctStudent)
	// The test clock is frozen, so every audit row would share one created_at; stepping it before each
	// request keeps "latest" well defined.
	patch := func(body map[string]any) response {
		e.t.Helper()
		e.clock.Add(time.Second)
		return e.do("PATCH", "/api/v1/users/"+stu, body, admin)
	}

	type line struct{ action, actorRole, before, after, afterName, ip string }
	last := func() line {
		var l line
		err := e.pool.QueryRow(e.t.Context(),
			`SELECT action, actor_role::text, coalesce(before->>'role', ''), coalesce(after->>'role', ''), coalesce(after->>'name', ''), host(ip)
			 FROM audit_logs WHERE entity_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`, stu).Scan(&l.action, &l.actorRole, &l.before, &l.after, &l.afterName, &l.ip)
		if err != nil {
			t.Fatalf("reading the latest audit row: %v", err)
		}
		return l
	}

	mustStatus(t, patch(map[string]any{"role": "OB"}), http.StatusOK)
	if l := last(); l.action != "USER_ROLE_CHANGED" || l.actorRole != "ADMIN" || l.before != "MAHASISWA" || l.after != "OB" || l.ip != "192.0.2.10" {
		t.Errorf("role change audited as %+v", l)
	}

	mustStatus(t, patch(map[string]any{"is_active": false}), http.StatusOK)
	if l := last(); l.action != "USER_DEACTIVATED" {
		t.Errorf("deactivation audited as %+v", l)
	}
	mustStatus(t, patch(map[string]any{"is_active": true}), http.StatusOK)
	if l := last(); l.action != "USER_ACTIVATED" {
		t.Errorf("activation audited as %+v", l)
	}

	if _, err := e.pool.Exec(e.t.Context(), `UPDATE users SET booking_blocked_until = $1 WHERE id = $2`, e.clock.Now().Add(72*3600e9), stu); err != nil {
		t.Fatal(err)
	}
	cleared := patch(map[string]any{"clear_booking_block": true})
	mustStatus(t, cleared, http.StatusOK)
	if cleared.json()["booking_blocked_until"] != nil {
		t.Errorf("booking_blocked_until = %v after clearing", cleared.json()["booking_blocked_until"])
	}
	if l := last(); l.action != "USER_BLOCK_CLEARED" {
		t.Errorf("clearing the block audited as %+v", l)
	}

	mustStatus(t, patch(map[string]any{"name": "Nama Baru"}), http.StatusOK)
	if l := last(); l.action != "USER_UPDATED" || l.afterName != "Nama Baru" {
		t.Errorf("name change audited as %+v", l)
	}

	// One request that changes two things leaves one line for each, so either can be found by action.
	before := e.acctCountAudit("USER_ROLE_CHANGED") + e.acctCountAudit("USER_DEACTIVATED")
	mustStatus(t, patch(map[string]any{"role": "MAHASISWA", "is_active": false}), http.StatusOK)
	if after := e.acctCountAudit("USER_ROLE_CHANGED") + e.acctCountAudit("USER_DEACTIVATED"); after != before+2 {
		t.Errorf("combined change wrote %d audit lines, want 2", after-before)
	}
}

func TestAuditLineIsRolledBackWithItsChange(t *testing.T) {
	e := newEnv(t)
	admin := bearer(e.login(acctAdmin).accessToken)
	a, b := e.userIDOf(acctStudent), e.userIDOf("2021101002")

	mustStatus(t, e.do("PATCH", "/api/v1/users/"+a, map[string]any{"email": "sama@kampus.test"}, admin), http.StatusOK)
	updates := e.acctCountAudit("USER_UPDATED")

	// The second account cannot take an email that is already in use: the change fails and leaves no line.
	res := e.do("PATCH", "/api/v1/users/"+b, map[string]any{"email": "sama@kampus.test"}, admin)
	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate email: %d, want 422", res.Code)
	}
	if n := e.acctCountAudit("USER_UPDATED"); n != updates {
		t.Errorf("failed change left an audit line: %d -> %d", updates, n)
	}
}

func TestAuditLogsAreReadOnlyAndAdminOnly(t *testing.T) {
	e := newEnv(t)
	admin := bearer(e.login(acctAdmin).accessToken)
	stu := e.userIDOf(acctStudent)

	mustStatus(t, e.do("PATCH", "/api/v1/users/"+stu, map[string]any{"role": "OB"}, admin), http.StatusOK)
	mustStatus(t, e.do("PATCH", "/api/v1/users/"+stu, map[string]any{"name": "Lain"}, admin), http.StatusOK)

	all := e.do("GET", "/api/v1/audit-logs", nil, admin)
	mustStatus(t, all, http.StatusOK)
	if n := len(dataOf(all)); n < 2 {
		t.Fatalf("audit list has %d items, want at least 2", n)
	}
	first := dataOf(all)[0].(map[string]any)
	if first["actor_role"] != "ADMIN" || first["actor_identifier"] != acctAdmin || first["request_id"] == nil || first["ip"] != "192.0.2.10" {
		t.Errorf("audit item = %v", first)
	}

	byAction := e.do("GET", "/api/v1/audit-logs?action=USER_ROLE_CHANGED", nil, admin)
	mustStatus(t, byAction, http.StatusOK)
	if got := dataOf(byAction); len(got) != 1 || got[0].(map[string]any)["entity_id"] != stu {
		t.Errorf("filter by action = %v", got)
	}
	byEntity := e.do("GET", "/api/v1/audit-logs?entity_type=USER&entity_id="+stu+"&per_page=1", nil, admin)
	mustStatus(t, byEntity, http.StatusOK)
	if meta := byEntity.json()["meta"].(map[string]any); meta["per_page"] != float64(1) || meta["total"].(float64) < 2 {
		t.Errorf("pagination meta = %v", meta)
	}
	future := url.QueryEscape(e.clock.Now().Add(24 * 3600e9).Format("2006-01-02T15:04:05Z07:00"))
	if res := e.do("GET", "/api/v1/audit-logs?from="+future, nil, admin); len(dataOf(res)) != 0 {
		t.Errorf("filter from the future returned %d items", len(dataOf(res)))
	}
	if res := e.do("GET", "/api/v1/audit-logs?actor_id=not-a-uuid", nil, admin); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad actor_id: %d, want 422", res.Code)
	}

	// Only administrators read it.
	for _, ident := range []string{acctTU, acctKalab, acctOB, acctStudent} {
		if res := e.do("GET", "/api/v1/audit-logs", nil, bearer(e.login(ident).accessToken)); res.Code != http.StatusForbidden {
			t.Errorf("%s reading the audit log: %d, want 403", ident, res.Code)
		}
	}
	if res := e.do("GET", "/api/v1/audit-logs", nil); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous audit log: %d, want 401", res.Code)
	}

	// The trigger refuses edits and deletes even for a connection with full rights.
	for _, stmt := range []string{`UPDATE audit_logs SET action = 'X'`, `DELETE FROM audit_logs`} {
		if _, err := e.pool.Exec(e.t.Context(), stmt); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%q: err = %v, want the append-only trigger", stmt, err)
		}
	}
}

func TestRoomAndScheduleWritesAreAuditedPerRoomType(t *testing.T) {
	e := newEnv(t)
	tu := bearer(e.login(acctTU).accessToken)
	kalab := bearer(e.login(acctKalab).accessToken)
	room := func(code, typ string) map[string]any {
		return map[string]any{"org_unit_id": e.orgUnitID(), "code": code, "name": "Ruang " + code, "type": typ, "capacity": 20}
	}

	// TU approves KELAS, so it may create one; KALAB approves LAB only.
	created := e.do("POST", "/api/v1/rooms", room("AUD-T1", string(dbgen.RoomTypeKELAS)), tu)
	mustStatus(t, created, http.StatusCreated)
	if n := e.acctCountAudit("ROOM_CREATED"); n != 1 {
		t.Errorf("ROOM_CREATED rows = %d, want 1", n)
	}
	if res := e.do("POST", "/api/v1/rooms", room("AUD-T2", string(dbgen.RoomTypeLAB)), tu); res.Code != http.StatusForbidden {
		t.Errorf("TU creating a LAB room: %d, want 403", res.Code)
	}
	if res := e.do("POST", "/api/v1/rooms", room("AUD-T3", string(dbgen.RoomTypeKELAS)), kalab); res.Code != http.StatusForbidden {
		t.Errorf("KALAB creating a KELAS room: %d, want 403", res.Code)
	}
	if n := e.acctCountAudit("ROOM_CREATED"); n != 1 {
		t.Errorf("refused creations left audit lines: %d", n)
	}

	id, _ := created.json()["id"].(string)
	// The QR token never appears in the API representation of a room.
	if strings.Contains(created.Body.String(), "qr_token") {
		t.Errorf("room response exposes qr_token: %s", created.Body.String())
	}
	mustStatus(t, e.do("PATCH", "/api/v1/rooms/"+id, map[string]any{"capacity": 25}, tu), http.StatusOK)
	if n := e.acctCountAudit("ROOM_UPDATED"); n != 1 {
		t.Errorf("ROOM_UPDATED rows = %d, want 1", n)
	}
	// Moving a room to a type the actor does not manage is refused, and nothing is logged.
	if res := e.do("PATCH", "/api/v1/rooms/"+id, map[string]any{"type": string(dbgen.RoomTypeLAB)}, tu); res.Code != http.StatusForbidden {
		t.Errorf("TU moving a room to LAB: %d, want 403", res.Code)
	}
	if n := e.acctCountAudit("ROOM_UPDATED"); n != 1 {
		t.Errorf("refused update left an audit line: %d", n)
	}
	mustStatus(t, e.do("DELETE", "/api/v1/rooms/"+id, nil, tu), http.StatusNoContent)
	if n := e.acctCountAudit("ROOM_DELETED"); n != 1 {
		t.Errorf("ROOM_DELETED rows = %d, want 1", n)
	}

	// The stored QR token is long and random; only the managers' QR endpoint (Phase 5) will ever return it.
	var tokenLen int
	if err := e.pool.QueryRow(e.t.Context(), `SELECT length(qr_token) FROM rooms WHERE id = $1`, id).Scan(&tokenLen); err != nil || tokenLen != 43 {
		t.Errorf("qr_token length = %d (%v), want 43 (32 bytes, base64url)", tokenLen, err)
	}
	if n := e.countRows(`SELECT count(*) FROM audit_logs WHERE after::text LIKE '%qr_token%' OR before::text LIKE '%qr_token%'`); n != 0 {
		t.Errorf("qr_token reached %d audit rows", n)
	}
}

func TestScheduleWritesFollowTheRoomTypeAuthority(t *testing.T) {
	e := newEnv(t)
	tu := bearer(e.login(acctTU).accessToken)
	kalab := bearer(e.login(acctKalab).accessToken)
	admin := bearer(e.login(acctAdmin).accessToken)

	kelas := e.insertRoom("SCH-K1", "KELAS", 30)
	lab := e.insertRoom("SCH-L1", "LAB", 30)
	slot := func(room string, day string) map[string]any {
		return map[string]any{"room_id": room, "title": "Kuliah", "start_at": at(day, "08:00"), "end_at": at(day, "10:00")}
	}

	mustStatus(t, e.do("POST", "/api/v1/schedules", slot(kelas, mon), tu), http.StatusCreated)
	if res := e.do("POST", "/api/v1/schedules", slot(lab, mon), tu); res.Code != http.StatusForbidden {
		t.Errorf("TU scheduling a LAB: %d, want 403", res.Code)
	}
	mustStatus(t, e.do("POST", "/api/v1/schedules", slot(lab, mon), kalab), http.StatusCreated)
	if res := e.do("POST", "/api/v1/schedules", slot(kelas, tue), kalab); res.Code != http.StatusForbidden {
		t.Errorf("KALAB scheduling a KELAS: %d, want 403", res.Code)
	}
	// Administrators manage every type, even those they do not approve.
	mustStatus(t, e.do("POST", "/api/v1/schedules", slot(kelas, wed), admin), http.StatusCreated)
	mustStatus(t, e.do("POST", "/api/v1/schedules", slot(lab, wed), admin), http.StatusCreated)
	if n := e.acctCountAudit("SCHEDULE_CREATED"); n != 4 {
		t.Errorf("SCHEDULE_CREATED rows = %d, want 4 (refused requests leave none)", n)
	}
}
