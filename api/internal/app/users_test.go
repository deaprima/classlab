package app_test

import (
	"net/http"
	"strings"
	"testing"

	"classlab/api/internal/auth"
	"classlab/api/internal/seed"
)

func (e *env) orgUnitID() string {
	e.t.Helper()
	var id string
	if err := e.pool.QueryRow(e.t.Context(), "SELECT id FROM org_units LIMIT 1").Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func (e *env) newUserBody(identifier string) map[string]any {
	return map[string]any{
		"org_unit_id": e.orgUnitID(),
		"identifier":  identifier,
		"name":        "Pengguna Baru",
		"email":       identifier + "@kampus.test",
		"password":    "password-baru-1",
		"role":        "MAHASISWA",
	}
}

func fieldsOf(res response) map[string]bool {
	out := map[string]bool{}
	details, _ := res.json()["error"].(map[string]any)["details"].([]any)
	for _, d := range details {
		if f, ok := d.(map[string]any)["field"].(string); ok {
			out[f] = true
		}
	}
	return out
}

func TestCreateUser(t *testing.T) {
	e := newEnv(t)
	admin := e.login(kaprodiID)

	res := e.do("POST", "/api/v1/users", e.newUserBody("2022202001"), bearer(admin.accessToken))
	if res.Code != http.StatusCreated {
		t.Fatalf("status %d, body %s", res.Code, res.Body.String())
	}
	u := res.json()
	if u["identifier"] != "2022202001" || u["role"] != "MAHASISWA" || u["is_active"] != true {
		t.Errorf("unexpected user: %v", u)
	}
	for _, leaked := range []string{"password", "password_hash"} {
		if _, ok := u[leaked]; ok {
			t.Errorf("response exposes %q", leaked)
		}
	}

	var hash string
	if err := e.pool.QueryRow(t.Context(), "SELECT password_hash FROM users WHERE identifier = '2022202001'").Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") || strings.Contains(hash, "password-baru-1") {
		t.Errorf("password not stored as argon2id: %q", hash)
	}

	// The new account can sign in with the password it was created with.
	login := e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": "2022202001", "password": "password-baru-1"})
	if login.Code != http.StatusOK {
		t.Errorf("new user login: %d", login.Code)
	}
}

func TestCreateUserValidation(t *testing.T) {
	e := newEnv(t)
	admin := e.login(kaprodiID)
	post := func(mutate func(map[string]any)) response {
		body := e.newUserBody("2022202002")
		mutate(body)
		return e.do("POST", "/api/v1/users", body, bearer(admin.accessToken))
	}

	cases := []struct {
		name   string
		mutate func(map[string]any)
		field  string
	}{
		{"short password", func(b map[string]any) { b["password"] = "short" }, "password"},
		{"bad email", func(b map[string]any) { b["email"] = "not-an-email" }, "email"},
		{"display-name email", func(b map[string]any) { b["email"] = "Budi <budi@kampus.test>" }, "email"},
		{"bad identifier", func(b map[string]any) { b["identifier"] = "has space" }, "identifier"},
		{"blank name", func(b map[string]any) { b["name"] = "   " }, "name"},
		{"unknown role", func(b map[string]any) { b["role"] = "SUPERADMIN" }, "role"},
		{"missing org unit", func(b map[string]any) { delete(b, "org_unit_id") }, "org_unit_id"},
		{"nonexistent org unit", func(b map[string]any) { b["org_unit_id"] = "00000000-0000-4000-8000-000000000001" }, "org_unit_id"},
		{"duplicate identifier", func(b map[string]any) { b["identifier"] = mahasiswaA }, "identifier"},
		{"duplicate email, other case", func(b map[string]any) { b["email"] = "ANDI@classlab.test" }, "email"},
	}
	for _, tc := range cases {
		res := post(tc.mutate)
		if res.Code != http.StatusUnprocessableEntity || res.errorCode() != "VALIDATION_ERROR" {
			t.Errorf("%s: %d %s; want 422 VALIDATION_ERROR", tc.name, res.Code, res.errorCode())
			continue
		}
		if !fieldsOf(res)[tc.field] {
			t.Errorf("%s: details should name field %q, got %s", tc.name, tc.field, res.Body.String())
		}
	}

	// Several problems are reported together, not one at a time.
	res := post(func(b map[string]any) { b["email"] = "x"; b["password"] = "y" })
	if f := fieldsOf(res); !f["email"] || !f["password"] {
		t.Errorf("expected both email and password errors, got %v", f)
	}

	if res := e.do("POST", "/api/v1/users", `{"identifier":"x",`, bearer(admin.accessToken)); res.Code != http.StatusBadRequest {
		t.Errorf("malformed JSON: %d; want 400", res.Code)
	}
}

func TestListUsers(t *testing.T) {
	e := newEnv(t)
	admin := e.login(kaprodiID)
	get := func(query string) (data []any, meta map[string]any, code int) {
		res := e.do("GET", "/api/v1/users"+query, nil, bearer(admin.accessToken))
		if res.Code != http.StatusOK {
			return nil, nil, res.Code
		}
		body := res.json()
		data, _ = body["data"].([]any)
		meta, _ = body["meta"].(map[string]any)
		return data, meta, res.Code
	}

	data, meta, _ := get("")
	if len(data) != 8 || meta["total"] != float64(8) || meta["page"] != float64(1) || meta["per_page"] != float64(20) {
		t.Errorf("default listing: %d items, meta %v", len(data), meta)
	}
	for _, item := range data {
		if _, ok := item.(map[string]any)["password_hash"]; ok {
			t.Fatal("list exposes password_hash")
		}
	}

	data, meta, _ = get("?per_page=3&page=3")
	if len(data) != 2 || meta["total"] != float64(8) {
		t.Errorf("page 3 of 3: %d items, meta %v; want 2 items of 8", len(data), meta)
	}

	if data, meta, _ = get("?role=MAHASISWA"); len(data) != 5 || meta["total"] != float64(5) {
		t.Errorf("role filter: %d items", len(data))
	}
	if data, _, _ = get("?q=BUNGA"); len(data) != 1 {
		t.Errorf("q matches name case-insensitively: %d items; want 1", len(data))
	}
	if data, _, _ = get("?q=2021101003"); len(data) != 1 {
		t.Errorf("q matches identifier: %d items; want 1", len(data))
	}
	if data, _, _ = get("?q=tukalab@"); len(data) != 1 {
		t.Errorf("q matches email: %d items; want 1", len(data))
	}
	// LIKE wildcards in user input must be literal, not "match everything".
	if data, _, _ = get("?q=%25"); len(data) != 0 {
		t.Errorf("q=%% matched %d users; wildcards must be escaped", len(data))
	}
	if data, _, _ = get("?q=_"); len(data) != 0 {
		t.Errorf("q=_ matched %d users; wildcards must be escaped", len(data))
	}

	if _, err := e.pool.Exec(t.Context(), "UPDATE users SET is_active = false WHERE identifier = $1", mahasiswaB); err != nil {
		t.Fatal(err)
	}
	if data, _, _ = get("?is_active=false"); len(data) != 1 {
		t.Errorf("is_active=false: %d items; want 1", len(data))
	}
	if data, _, _ = get("?is_active=true&role=MAHASISWA"); len(data) != 4 {
		t.Errorf("combined filters: %d items; want 4", len(data))
	}

	for _, bad := range []string{"?role=NOPE", "?is_active=maybe", "?page=0", "?page=x", "?per_page=101", "?per_page=0"} {
		if _, _, code := get(bad); code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d; want 422", bad, code)
		}
	}
}

func TestUpdateUser(t *testing.T) {
	e := newEnv(t)
	admin := e.login(kaprodiID)
	target := e.login(mahasiswaA)
	path := "/api/v1/users/" + target.userID

	res := e.do("PATCH", path, map[string]any{"name": "  Nama Baru  ", "role": "OB", "email": "baru@kampus.test"}, bearer(admin.accessToken))
	if res.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", res.Code, res.Body.String())
	}
	u := res.json()
	if u["name"] != "Nama Baru" || u["role"] != "OB" || u["email"] != "baru@kampus.test" || u["identifier"] != mahasiswaA {
		t.Errorf("unexpected user: %v", u)
	}

	// A role change applies to tokens that are already issued.
	if r := e.do("GET", "/api/v1/users", nil, bearer(target.accessToken)); r.Code != http.StatusForbidden {
		t.Errorf("former role should not matter and OB must be forbidden: %d", r.Code)
	}
	if _, err := e.pool.Exec(t.Context(), "UPDATE users SET role = 'KAPRODI' WHERE id = $1", target.userID); err != nil {
		t.Fatal(err)
	}
	if r := e.do("GET", "/api/v1/users", nil, bearer(target.accessToken)); r.Code != http.StatusOK {
		t.Errorf("promoted user's existing token should reflect the new role at once: %d", r.Code)
	}
}

func TestUpdateUserErrors(t *testing.T) {
	e := newEnv(t)
	admin := e.login(kaprodiID)
	b := bearer(admin.accessToken)

	if r := e.do("PATCH", "/api/v1/users/00000000-0000-4000-8000-000000000001", map[string]any{"name": "x"}, b); r.Code != http.StatusNotFound {
		t.Errorf("unknown id: %d; want 404", r.Code)
	}
	if r := e.do("PATCH", "/api/v1/users/not-a-uuid", map[string]any{"name": "x"}, b); r.Code != http.StatusNotFound {
		t.Errorf("malformed id: %d; want 404", r.Code)
	}
	if r := e.do("PATCH", "/api/v1/users/"+admin.userID, map[string]any{}, b); r.Code != http.StatusUnprocessableEntity {
		t.Errorf("empty patch: %d; want 422", r.Code)
	}
	if r := e.do("PATCH", "/api/v1/users/"+admin.userID, map[string]any{"identifier": "changed"}, b); r.Code != http.StatusBadRequest {
		t.Errorf("identifier is immutable and not in the contract: %d; want 400", r.Code)
	}

	// Lockout guards.
	if r := e.do("PATCH", "/api/v1/users/"+admin.userID, map[string]any{"is_active": false}, b); r.Code != http.StatusUnprocessableEntity {
		t.Errorf("self-deactivation: %d; want 422", r.Code)
	}
	if r := e.do("PATCH", "/api/v1/users/"+admin.userID, map[string]any{"role": "MAHASISWA"}, b); r.Code != http.StatusUnprocessableEntity {
		t.Errorf("self-demotion: %d; want 422", r.Code)
	}
	if r := e.do("PATCH", "/api/v1/users/"+admin.userID, map[string]any{"role": "KAPRODI", "name": "Tetap Kaprodi"}, b); r.Code != http.StatusOK {
		t.Errorf("a no-op role change on yourself is fine: %d", r.Code)
	}

	// Uniqueness is enforced on update too.
	other := e.login(mahasiswaB)
	if r := e.do("PATCH", "/api/v1/users/"+other.userID, map[string]any{"email": "andi@classlab.test"}, b); r.Code != http.StatusUnprocessableEntity || !fieldsOf(r)["email"] {
		t.Errorf("duplicate email on update: %d %s", r.Code, r.Body.String())
	}
}

func TestDeactivatingUserEndsTheirSessions(t *testing.T) {
	e := newEnv(t)
	admin := e.login(kaprodiID)
	victim := e.login(mahasiswaA)

	res := e.do("PATCH", "/api/v1/users/"+victim.userID, map[string]any{"is_active": false}, bearer(admin.accessToken))
	if res.Code != http.StatusOK || res.json()["is_active"] != false {
		t.Fatalf("deactivate: %d %s", res.Code, res.Body.String())
	}

	if r := e.do("GET", "/api/v1/auth/me", nil, bearer(victim.accessToken)); r.Code != http.StatusUnauthorized {
		t.Errorf("access token after deactivation: %d; want 401", r.Code)
	}
	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(victim.refresh)); r.Code != http.StatusUnauthorized {
		t.Errorf("refresh after deactivation: %d; want 401", r.Code)
	}
	if r := e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": mahasiswaA, "password": seed.DevPassword}); r.Code != http.StatusUnauthorized {
		t.Errorf("login after deactivation: %d; want 401", r.Code)
	}

	// Reactivating restores login.
	e.do("PATCH", "/api/v1/users/"+victim.userID, map[string]any{"is_active": true}, bearer(admin.accessToken))
	if r := e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": mahasiswaA, "password": seed.DevPassword}); r.Code != http.StatusOK {
		t.Errorf("login after reactivation: %d; want 200", r.Code)
	}
}

func TestPasswordResetEndsSessionsAndSwitchesPassword(t *testing.T) {
	e := newEnv(t)
	admin := e.login(kaprodiID)
	victim := e.login(mahasiswaA)

	if r := e.do("PATCH", "/api/v1/users/"+victim.userID, map[string]any{"password": "short"}, bearer(admin.accessToken)); r.Code != http.StatusUnprocessableEntity {
		t.Errorf("weak reset password: %d; want 422", r.Code)
	}
	if r := e.do("PATCH", "/api/v1/users/"+victim.userID, map[string]any{"password": "reset-password-9"}, bearer(admin.accessToken)); r.Code != http.StatusOK {
		t.Fatalf("reset: %d", r.Code)
	}

	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(victim.refresh)); r.Code != http.StatusUnauthorized {
		t.Errorf("old session survived a password reset: %d", r.Code)
	}
	if r := e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": mahasiswaA, "password": seed.DevPassword}); r.Code != http.StatusUnauthorized {
		t.Errorf("old password still works: %d", r.Code)
	}
	if r := e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": mahasiswaA, "password": "reset-password-9"}); r.Code != http.StatusOK {
		t.Errorf("new password rejected: %d", r.Code)
	}
}

func TestRefreshCookieOnlyAppliesToAuthPath(t *testing.T) {
	e := newEnv(t)
	res := e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": mahasiswaA, "password": seed.DevPassword})
	if c := res.cookie(auth.RefreshCookieName); c == nil || c.Path != "/api/v1/auth" {
		t.Fatalf("cookie path = %+v; want /api/v1/auth", c)
	}
}
