package app_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"classlab/api/internal/auth"
	"classlab/api/internal/config"
	"classlab/api/internal/seed"
)

func TestLoginSuccess(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.CookieSecure = true })

	res := e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": tuKalabID, "password": seed.DevPassword})
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", res.Code, res.Body.String())
	}
	body := res.json()

	if body["expires_in"] != float64(900) {
		t.Errorf("expires_in = %v; want 900", body["expires_in"])
	}
	if tok, _ := body["access_token"].(string); strings.Count(tok, ".") != 2 {
		t.Errorf("access_token is not a JWT: %q", tok)
	}
	user, _ := body["user"].(map[string]any)
	if user["role"] != "TU_KALAB" || user["identifier"] != tuKalabID {
		t.Errorf("unexpected user: %v", user)
	}
	if !strings.HasSuffix(user["created_at"].(string), "+07:00") {
		t.Errorf("created_at should use the +07:00 offset, got %v", user["created_at"])
	}
	for _, leaked := range []string{"password", "password_hash", "hash"} {
		if _, ok := user[leaked]; ok {
			t.Errorf("response exposes %q", leaked)
		}
	}

	c := res.cookie(auth.RefreshCookieName)
	if c == nil {
		t.Fatal("no refresh cookie set")
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/api/v1/auth" {
		t.Errorf("cookie attributes wrong: %+v", c)
	}
	if strings.Contains(res.Body.String(), c.Value) {
		t.Error("refresh token must never appear in the response body")
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Error("auth responses must be Cache-Control: no-store")
	}

	// Only the hash may be stored.
	var stored string
	if err := e.pool.QueryRow(t.Context(), "SELECT token_hash FROM refresh_tokens").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == c.Value || len(stored) != 64 {
		t.Errorf("stored token_hash = %q; want a SHA-256 hex digest distinct from the cookie value", stored)
	}
}

func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	e := newEnv(t)
	if _, err := e.pool.Exec(t.Context(), "UPDATE users SET is_active = false WHERE identifier = $1", mahasiswaB); err != nil {
		t.Fatal(err)
	}

	cases := map[string]map[string]string{
		"wrong password": {"identifier": tuKalabID, "password": "not-the-password"},
		"unknown user":   {"identifier": "9999999999", "password": seed.DevPassword},
		"inactive user":  {"identifier": mahasiswaB, "password": seed.DevPassword},
	}
	var messages []string
	for name, creds := range cases {
		res := e.do("POST", "/api/v1/auth/login", creds, fromIP("198.51.100."+string(rune('1'+len(messages)))))
		if res.Code != http.StatusUnauthorized || res.errorCode() != "UNAUTHENTICATED" {
			t.Errorf("%s: got %d %s; want 401 UNAUTHENTICATED", name, res.Code, res.errorCode())
		}
		if res.cookie(auth.RefreshCookieName) != nil {
			t.Errorf("%s: failed login must not set a cookie", name)
		}
		messages = append(messages, res.json()["error"].(map[string]any)["message"].(string))
	}
	for _, m := range messages[1:] {
		if m != messages[0] {
			t.Errorf("failure messages differ, which leaks whether the account exists: %q vs %q", m, messages[0])
		}
	}
}

func TestLoginRejectsBadRequests(t *testing.T) {
	e := newEnv(t)
	cases := []struct {
		name, body string
		status     int
		code       string
	}{
		{"empty fields", `{"identifier":"","password":""}`, 422, "VALIDATION_ERROR"},
		{"missing password", `{"identifier":"x"}`, 422, "VALIDATION_ERROR"},
		{"not json", `identifier=x`, 400, "BAD_REQUEST"},
		{"unknown field", `{"identifier":"x","password":"y","admin":true}`, 400, "BAD_REQUEST"},
		{"two objects", `{"identifier":"x","password":"y"}{}`, 400, "BAD_REQUEST"},
	}
	for _, tc := range cases {
		res := e.do("POST", "/api/v1/auth/login", tc.body)
		if res.Code != tc.status || res.errorCode() != tc.code {
			t.Errorf("%s: got %d %s; want %d %s", tc.name, res.Code, res.errorCode(), tc.status, tc.code)
		}
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	bad := map[string]string{"identifier": mahasiswaA, "password": "wrong"}
	good := map[string]string{"identifier": mahasiswaA, "password": seed.DevPassword}

	for i := 1; i <= 5; i++ {
		if res := e.do("POST", "/api/v1/auth/login", bad); res.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status %d; want 401", i, res.Code)
		}
	}

	res := e.do("POST", "/api/v1/auth/login", bad)
	if res.Code != http.StatusTooManyRequests || res.errorCode() != "RATE_LIMITED" {
		t.Fatalf("6th attempt: %d %s; want 429 RATE_LIMITED", res.Code, res.errorCode())
	}
	if res.Header().Get("Retry-After") == "" {
		t.Error("429 must carry Retry-After")
	}

	// Locked out even with the right password: otherwise the limiter would not stop guessing.
	if res := e.do("POST", "/api/v1/auth/login", good); res.Code != http.StatusTooManyRequests {
		t.Errorf("correct password while limited: %d; want 429", res.Code)
	}

	// The budget is per (identifier, IP).
	if res := e.do("POST", "/api/v1/auth/login", good, fromIP("203.0.113.7")); res.Code != http.StatusOK {
		t.Errorf("same identifier from another IP: %d; want 200", res.Code)
	}
	if res := e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": mahasiswaB, "password": seed.DevPassword}); res.Code != http.StatusOK {
		t.Errorf("other identifier from the limited IP: %d; want 200", res.Code)
	}

	e.clock.Add(15*time.Minute + time.Second)
	if res := e.do("POST", "/api/v1/auth/login", good); res.Code != http.StatusOK {
		t.Errorf("after the window: %d; want 200", res.Code)
	}
}

func TestSuccessfulLoginResetsRateLimit(t *testing.T) {
	e := newEnv(t)
	bad := map[string]string{"identifier": mahasiswaA, "password": "wrong"}
	for round := 0; round < 3; round++ {
		for i := 0; i < 4; i++ {
			if res := e.do("POST", "/api/v1/auth/login", bad); res.Code != http.StatusUnauthorized {
				t.Fatalf("round %d attempt %d: %d; want 401", round, i, res.Code)
			}
		}
		e.login(mahasiswaA)
	}
}

func TestAccessTokenChecks(t *testing.T) {
	e := newEnv(t)
	s := e.login(mahasiswaA)

	cases := map[string][]reqOpt{
		"no header":      nil,
		"garbage token":  {bearer("abc.def.ghi")},
		"basic scheme":   {func(r *http.Request) { r.Header.Set("Authorization", "Basic "+s.accessToken) }},
		"empty bearer":   {func(r *http.Request) { r.Header.Set("Authorization", "Bearer ") }},
		"token in query": nil,
	}
	for name, opts := range cases {
		path := "/api/v1/auth/me"
		if name == "token in query" {
			path += "?access_token=" + s.accessToken
		}
		res := e.do("GET", path, nil, opts...)
		if res.Code != http.StatusUnauthorized || res.errorCode() != "UNAUTHENTICATED" {
			t.Errorf("%s: %d %s; want 401 UNAUTHENTICATED", name, res.Code, res.errorCode())
		}
		if res.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("%s: 401 should carry WWW-Authenticate", name)
		}
	}

	if res := e.do("GET", "/api/v1/auth/me", nil, bearer(s.accessToken)); res.Code != http.StatusOK {
		t.Fatalf("valid token: %d", res.Code)
	} else if res.json()["identifier"] != mahasiswaA {
		t.Errorf("me returned %v", res.json())
	}

	e.clock.Add(15 * time.Minute)
	if res := e.do("GET", "/api/v1/auth/me", nil, bearer(s.accessToken)); res.Code != http.StatusUnauthorized {
		t.Errorf("expired token: %d; want 401", res.Code)
	}
}

func TestDeactivatedUserLosesAccessImmediately(t *testing.T) {
	e := newEnv(t)
	s := e.login(mahasiswaA)

	if _, err := e.pool.Exec(t.Context(), "UPDATE users SET is_active = false WHERE identifier = $1", mahasiswaA); err != nil {
		t.Fatal(err)
	}
	if res := e.do("GET", "/api/v1/auth/me", nil, bearer(s.accessToken)); res.Code != http.StatusUnauthorized {
		t.Errorf("deactivated user with a live token: %d; want 401", res.Code)
	}
}

func TestRefreshRotation(t *testing.T) {
	e := newEnv(t)
	s := e.login(mahasiswaA)

	res := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(s.refresh))
	if res.Code != http.StatusOK {
		t.Fatalf("refresh: %d %s", res.Code, res.Body.String())
	}
	next := res.cookie(auth.RefreshCookieName)
	if next == nil || next.Value == "" || next.Value == s.refresh {
		t.Fatalf("refresh must rotate the cookie, got %+v", next)
	}
	if !next.HttpOnly || next.Path != "/api/v1/auth" {
		t.Errorf("rotated cookie attributes wrong: %+v", next)
	}
	body := res.json()
	if _, ok := body["refresh_token"]; ok || strings.Contains(res.Body.String(), next.Value) {
		t.Error("refresh token leaked into the body")
	}
	access, _ := body["access_token"].(string)
	if r := e.do("GET", "/api/v1/auth/me", nil, bearer(access)); r.Code != http.StatusOK {
		t.Errorf("new access token rejected: %d", r.Code)
	}

	// The rotated token chains: it can be used once more.
	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(next.Value)); r.Code != http.StatusOK {
		t.Errorf("second rotation: %d", r.Code)
	}
}

func TestRefreshReuseRevokesWholeFamily(t *testing.T) {
	e := newEnv(t)
	s := e.login(mahasiswaA)
	other := e.login(mahasiswaB)

	rotated := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(s.refresh))
	legit := rotated.cookie(auth.RefreshCookieName).Value

	// An attacker replays the original, already-rotated token.
	replay := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(s.refresh))
	if replay.Code != http.StatusUnauthorized || replay.errorCode() != "UNAUTHENTICATED" {
		t.Fatalf("replay: %d %s; want 401 UNAUTHENTICATED", replay.Code, replay.errorCode())
	}

	// The legitimate successor is now dead as well: the family was burned.
	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(legit)); r.Code != http.StatusUnauthorized {
		t.Errorf("successor after reuse detection: %d; want 401", r.Code)
	}

	// Other users' sessions are untouched.
	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(other.refresh)); r.Code != http.StatusOK {
		t.Errorf("unrelated family: %d; want 200", r.Code)
	}

	var active int
	if err := e.pool.QueryRow(t.Context(),
		"SELECT count(*) FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL", s.userID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Errorf("%d token(s) of the compromised family are still active", active)
	}
}

func TestRefreshConcurrentUseYieldsExactlyOneSuccess(t *testing.T) {
	e := newEnv(t)
	s := e.login(mahasiswaA)

	const n = 8
	var wg sync.WaitGroup
	codes := make(chan int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(s.refresh)).Code
		}()
	}
	wg.Wait()
	close(codes)

	ok := 0
	for c := range codes {
		if c == http.StatusOK {
			ok++
		}
	}
	if ok != 1 {
		t.Errorf("%d concurrent refreshes succeeded; want exactly 1", ok)
	}
}

func TestRefreshRejections(t *testing.T) {
	e := newEnv(t)
	s := e.login(mahasiswaA)

	if r := e.do("POST", "/api/v1/auth/refresh", nil); r.Code != http.StatusUnauthorized {
		t.Errorf("no cookie: %d; want 401", r.Code)
	}
	r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie("made-up"))
	if r.Code != http.StatusUnauthorized {
		t.Errorf("unknown token: %d; want 401", r.Code)
	}
	if c := r.cookie(auth.RefreshCookieName); c == nil || c.MaxAge >= 0 {
		t.Error("a rejected refresh should clear the cookie")
	}

	e.clock.Add(7*24*time.Hour + time.Second)
	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(s.refresh)); r.Code != http.StatusUnauthorized {
		t.Errorf("expired refresh token: %d; want 401", r.Code)
	}
}

func TestRefreshFailsForDeactivatedUser(t *testing.T) {
	e := newEnv(t)
	s := e.login(mahasiswaA)
	if _, err := e.pool.Exec(t.Context(), "UPDATE users SET is_active = false WHERE identifier = $1", mahasiswaA); err != nil {
		t.Fatal(err)
	}
	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(s.refresh)); r.Code != http.StatusUnauthorized {
		t.Errorf("refresh for deactivated user: %d; want 401", r.Code)
	}
}

func TestLogout(t *testing.T) {
	e := newEnv(t)
	s := e.login(mahasiswaA)
	rotated := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(s.refresh))
	current := rotated.cookie(auth.RefreshCookieName).Value
	access := rotated.json()["access_token"].(string)

	res := e.do("POST", "/api/v1/auth/logout", nil, bearer(access), refreshCookie(current))
	if res.Code != http.StatusOK {
		t.Fatalf("logout: %d %s", res.Code, res.Body.String())
	}
	if c := res.cookie(auth.RefreshCookieName); c == nil || c.MaxAge >= 0 {
		t.Error("logout must clear the refresh cookie")
	}
	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(current)); r.Code != http.StatusUnauthorized {
		t.Errorf("refresh after logout: %d; want 401", r.Code)
	}
}

func TestLogoutCannotRevokeAnotherUsersSession(t *testing.T) {
	e := newEnv(t)
	victim := e.login(mahasiswaA)
	attacker := e.login(mahasiswaB)

	e.do("POST", "/api/v1/auth/logout", nil, bearer(attacker.accessToken), refreshCookie(victim.refresh))

	if r := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(victim.refresh)); r.Code != http.StatusOK {
		t.Errorf("victim's session was revoked by someone else's logout: %d", r.Code)
	}
}

func TestLogoutRequiresAuthentication(t *testing.T) {
	e := newEnv(t)
	s := e.login(mahasiswaA)
	if r := e.do("POST", "/api/v1/auth/logout", nil, refreshCookie(s.refresh)); r.Code != http.StatusUnauthorized {
		t.Errorf("logout without bearer: %d; want 401", r.Code)
	}
}
