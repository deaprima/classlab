package integration_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"classlab/api/internal/config"
	"classlab/api/internal/httpx"
	"classlab/api/internal/maintenance"
)

// --- per-student booking quota ----------------------------------------------------------------

func TestBookingQuotaPerStudent(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Business.MaxActiveBookings = 5 })
	a, b := e.login(mahasiswaA).accessToken, e.login(mahasiswaB).accessToken
	tu := e.tuToken()

	book := func(tok string) response {
		return e.postBooking(tok, e.freshRoom(), at(tue, "08:00"), at(tue, "09:00"), nil)
	}
	var mine []string
	for range 5 {
		res := book(a)
		mustStatus(t, res, http.StatusCreated)
		mine = append(mine, idOf(res))
	}

	res := book(a)
	if res.Code != http.StatusConflict || res.errorCode() != "QUOTA_EXCEEDED" {
		t.Fatalf("sixth booking: %d %s; want 409 QUOTA_EXCEEDED", res.Code, res.Body.String())
	}
	if msg, _ := res.json()["error"].(map[string]any)["message"].(string); !strings.Contains(msg, "5 pengajuan aktif") {
		t.Errorf("message = %q; want it to state the limit", msg)
	}
	if f := fieldsOf(res); len(f) != 0 {
		t.Errorf("the quota is not about one field, got %v", f)
	}

	// The limit is per student.
	mustStatus(t, book(b), http.StatusCreated)

	// Cancelling frees a place; approving does not (an approved booking still holds a room).
	mustStatus(t, e.action(a, mine[0], "cancel", nil), http.StatusOK)
	mustStatus(t, book(a), http.StatusCreated)
	if res := book(a); res.Code != http.StatusConflict {
		t.Errorf("after refilling: %d; want 409", res.Code)
	}
	mustStatus(t, e.action(tu, mine[1], "approve", nil), http.StatusOK)
	if res := book(a); res.Code != http.StatusConflict {
		t.Errorf("an approved booking must still count: %d; want 409", res.Code)
	}

	// Rejection frees a place too.
	mustStatus(t, e.action(tu, mine[2], "reject", map[string]string{"reason": "Dipakai ujian"}), http.StatusOK)
	mustStatus(t, book(a), http.StatusCreated)

	// Bookings that are over do not count. Student B already holds one live booking.
	for range 10 {
		e.insertBooking(e.freshRoom(), mahasiswaB, "DISETUJUI", "Sudah lewat", at("2026-10-10", "08:00"), at("2026-10-10", "09:00"))
	}
	for range 4 {
		mustStatus(t, book(b), http.StatusCreated)
	}
	if res := book(b); res.Code != http.StatusConflict {
		t.Errorf("student B's fifth live booking was allowed to be followed by a sixth: %d", res.Code)
	}
}

// A booking in progress still holds a place: BERLANGSUNG counts towards the quota like MENUNGGU and DISETUJUI.
func TestBookingInProgressCountsTowardsTheQuota(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Business.MaxActiveBookings = 2 })
	tok := e.login(mahasiswaA).accessToken

	// 09:00-12:00 WIB today, so it has not ended at the test clock's 10:00 WIB.
	e.insertBooking(e.freshRoom(), mahasiswaA, "BERLANGSUNG", "Sedang dipakai", at("2026-10-14", "09:00"), at("2026-10-14", "12:00"))
	mustStatus(t, e.postBooking(tok, e.freshRoom(), at(tue, "08:00"), at(tue, "09:00"), nil), http.StatusCreated)
	if res := e.postBooking(tok, e.freshRoom(), at(tue, "10:00"), at(tue, "11:00"), nil); res.Code != http.StatusConflict {
		t.Errorf("a third live booking, one of them in progress: %d %s; want 409", res.Code, res.Body.String())
	}
}

// A retry of a booking the student already made must return it even when they are at the limit.
func TestBookingQuotaDoesNotBlockIdempotentRetries(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Business.MaxActiveBookings = 2 })
	tok := e.login(mahasiswaA).accessToken
	room := e.freshRoom()
	key := func(r *http.Request) { r.Header.Set("Idempotency-Key", "retry-1") }

	first := e.postBooking(tok, room, at(tue, "08:00"), at(tue, "09:00"), nil, key)
	mustStatus(t, first, http.StatusCreated)
	mustStatus(t, e.postBooking(tok, e.freshRoom(), at(tue, "08:00"), at(tue, "09:00"), nil), http.StatusCreated)
	if res := e.postBooking(tok, e.freshRoom(), at(tue, "08:00"), at(tue, "09:00"), nil); res.Code != http.StatusConflict {
		t.Fatalf("third booking: %d; want 409", res.Code)
	}

	again := e.postBooking(tok, room, at(tue, "08:00"), at(tue, "09:00"), nil, key)
	if again.Code != http.StatusCreated || idOf(again) != idOf(first) || again.Header().Get("Idempotency-Replayed") != "true" {
		t.Errorf("retry at the limit: %d %s; want the original booking back", again.Code, again.Body.String())
	}
}

// Two requests for different rooms hold different room locks, so only the quota lock keeps their count honest.
func TestBookingQuotaHoldsUnderConcurrency(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Business.MaxActiveBookings = 5 })
	tok := e.login(mahasiswaA).accessToken

	const n = 12
	rooms := make([]string, n)
	for i := range rooms {
		rooms[i] = e.freshRoom()
	}
	results := make([]response, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = e.postBooking(tok, rooms[i], at(tue, "08:00"), at(tue, "09:00"), nil)
		}()
	}
	close(start)
	wg.Wait()

	created, refused := 0, 0
	for _, res := range results {
		switch res.Code {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			refused++
		default:
			t.Errorf("unexpected response %d: %s", res.Code, res.Body.String())
		}
	}
	if created != 5 || refused != n-5 {
		t.Errorf("%d created, %d refused; want 5 and %d", created, refused, n-5)
	}
	if rows := e.countRows("SELECT count(*) FROM bookings WHERE requester_id = $1", e.userIDOf(mahasiswaA)); rows != 5 {
		t.Errorf("%d bookings stored; want 5", rows)
	}
}

// --- request plumbing ----------------------------------------------------------------------------

func TestPageIsCapped(t *testing.T) {
	e := newEnv(t)
	admin := bearer(e.adminToken())

	for _, path := range []string{"bookings", "rooms", "reports", "notifications"} {
		if res := e.do("GET", "/api/v1/"+path+"?page=10000&per_page=100", nil, admin); res.Code != http.StatusOK {
			t.Errorf("%s page 10000: %d; want 200", path, res.Code)
		}
		for _, page := range []string{"10001", "999999999999", "99999999999999999999"} {
			res := e.do("GET", "/api/v1/"+path+"?page="+page, nil, admin)
			if res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)["page"] {
				t.Errorf("%s page %s: %d %s; want 422 naming page (this used to be a 500)", path, page, res.Code, res.Body.String())
			}
		}
	}
	if res := e.do("GET", "/api/v1/users?page=999999999999", nil, admin); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("users with a huge page: %d; want 422", res.Code)
	}
}

func TestRequestIDFromClientIsValidated(t *testing.T) {
	e := newEnv(t)
	withID := func(id string) reqOpt { return func(r *http.Request) { r.Header.Set("X-Request-Id", id) } }

	if res := e.do("GET", "/health", nil, withID("trace-123.abc_X")); res.Header().Get("X-Request-Id") != "trace-123.abc_X" {
		t.Errorf("a plain id was not kept: %q", res.Header().Get("X-Request-Id"))
	}
	for name, bad := range map[string]string{
		"too long":   strings.Repeat("A", 65),
		"huge":       strings.Repeat("A", 5000),
		"has space":  "a b",
		"has quote":  `a"b`,
		"has slash":  "a/b",
		"unicode":    "idé",
		"only dots?": "../../x",
	} {
		got := e.do("GET", "/health", nil, withID(bad)).Header().Get("X-Request-Id")
		if got == bad || !strings.HasPrefix(got, "req_") {
			t.Errorf("%s: request id %q was accepted; want a generated one", name, got)
		}
	}
	// The same id reaches the error body, so it must be the validated one.
	res := e.do("GET", "/api/v1/auth/me", nil, withID(strings.Repeat("B", 200)))
	if id, _ := res.json()["error"].(map[string]any)["request_id"].(string); !strings.HasPrefix(id, "req_") {
		t.Errorf("error body request_id = %q", id)
	}
}

func TestAPIResponsesAreNotCached(t *testing.T) {
	e := newEnv(t)
	tok := e.login(mahasiswaA).accessToken

	for name, res := range map[string]response{
		"list":         e.do("GET", "/api/v1/rooms", nil, bearer(tok)),
		"unauthorized": e.do("GET", "/api/v1/rooms", nil),
		"not found":    e.do("GET", "/api/v1/bookings/00000000-0000-4000-8000-0000000000aa", nil, bearer(tok)),
		"login":        e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": "x", "password": "y"}),
	} {
		if cc := res.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%s: Cache-Control = %q; want no-store", name, cc)
		}
	}

	// A report photo is served through the attachments endpoint and is never cached (it can show people).
	rep := e.postReport(tok, e.freshRoom(), "Rusak", &photoPart{"a.png", "image/png", pngBytes(0)})
	mustStatus(t, rep, http.StatusCreated)
	photo := e.do("GET", "/api/v1/attachments/"+rep.json()["attachment_id"].(string), nil, bearer(tok))
	if cc := photo.Header().Get("Cache-Control"); cc != "private, no-store" {
		t.Errorf("photo Cache-Control = %q", cc)
	}
}

func TestLoginLogsDoNotContainTheIdentifier(t *testing.T) {
	e := newEnv(t)
	typo := "rahasia-passwordku-123" // someone typed their password into the identifier field

	for range 7 { // the last attempts hit the rate limit, which logs too
		e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": typo, "password": "x"})
	}
	e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": mahasiswaB, "password": "salah-semua"})

	logs := e.logs.String()
	for _, secret := range []string{typo, mahasiswaB} {
		if strings.Contains(logs, secret) {
			t.Errorf("logs contain the raw identifier %q", secret)
		}
	}
	for _, want := range []string{"login failed", "login rate limited", "identifier_ref="} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs lack %q; the failures should still be traceable", want)
		}
	}
}

// --- housekeeping, indexes, locks ----------------------------------------------------------------

func (e *env) insertRefreshToken(userID, hash string, expiresAt time.Time, revoked bool) {
	e.t.Helper()
	var revokedAt *time.Time
	if revoked {
		revokedAt = &expiresAt
	}
	_, err := e.pool.Exec(e.t.Context(),
		`INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at, revoked_at) VALUES ($1, gen_random_uuid(), $2, $3, $4)`,
		userID, hash, expiresAt, revokedAt)
	if err != nil {
		e.t.Fatalf("inserting refresh token: %v", err)
	}
}

func (e *env) tokenHashes() []string {
	e.t.Helper()
	rows, err := e.pool.Query(e.t.Context(), "SELECT token_hash FROM refresh_tokens WHERE token_hash LIKE 'purge-%' ORDER BY token_hash")
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, h)
	}
	return out
}

func TestPurgeExpiredRefreshTokens(t *testing.T) {
	e := newEnv(t)
	uid := e.userIDOf(mahasiswaA)
	now := e.clock.Now()
	e.insertRefreshToken(uid, "purge-long-expired", now.Add(-48*time.Hour), false)
	e.insertRefreshToken(uid, "purge-long-expired-revoked", now.Add(-72*time.Hour), true)
	e.insertRefreshToken(uid, "purge-just-expired", now.Add(-2*time.Hour), false) // inside the one-day grace
	e.insertRefreshToken(uid, "purge-valid", now.Add(24*time.Hour), false)
	e.insertRefreshToken(uid, "purge-valid-revoked", now.Add(24*time.Hour), true) // kept: reuse detection needs it

	n, err := maintenance.PurgeExpiredRefreshTokens(t.Context(), e.pool, e.clock)
	if err != nil || n != 2 {
		t.Fatalf("purged %d (%v); want 2", n, err)
	}
	if got, want := e.tokenHashes(), []string{"purge-just-expired", "purge-valid", "purge-valid-revoked"}; !slices.Equal(got, want) {
		t.Errorf("remaining = %v; want %v", got, want)
	}
	if n, err := maintenance.PurgeExpiredRefreshTokens(t.Context(), e.pool, e.clock); err != nil || n != 0 {
		t.Errorf("second purge removed %d (%v); want 0", n, err)
	}

	// Live sessions are untouched: a real login still refreshes.
	sess := e.login(mahasiswaA)
	if _, err := maintenance.PurgeExpiredRefreshTokens(t.Context(), e.pool, e.clock); err != nil {
		t.Fatal(err)
	}
	if res := e.do("POST", "/api/v1/auth/refresh", nil, refreshCookie(sess.refresh)); res.Code != http.StatusOK {
		t.Errorf("refresh after a purge: %d; want 200", res.Code)
	}
}

func TestMaintenanceRunPurgesOnATimer(t *testing.T) {
	e := newEnv(t)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		maintenance.Run(ctx, e.pool, e.clock, slog.New(slog.NewTextHandler(io.Discard, nil)), 20*time.Millisecond)
	}()

	e.insertRefreshToken(e.userIDOf(mahasiswaA), "purge-stale", e.clock.Now().Add(-48*time.Hour), false)
	deadline := time.Now().Add(10 * time.Second)
	for len(e.tokenHashes()) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the stale token was never purged")
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop when its context ended")
	}
}

func TestMigration000003AddsAndRemovesItsIndexes(t *testing.T) {
	e := newEnv(t)
	indexes := []string{
		"booking_events_booking_idx", "report_events_report_idx", "refresh_tokens_user_idx", "refresh_tokens_family_idx",
		"refresh_tokens_expires_idx", "damage_reports_reporter_idx", "damage_reports_status_idx", "damage_reports_room_idx",
		"bookings_requester_active_idx",
	}
	present := func() int {
		return e.countRows("SELECT count(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname = ANY($1)", indexes)
	}
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations")
	run := func(name string) {
		t.Helper()
		sql, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.pool.Exec(t.Context(), string(sql)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	if present() != len(indexes) {
		t.Fatalf("%d of %d indexes present after migrating up", present(), len(indexes))
	}
	run("000003_indexes_for_existing_lookups.down.sql")
	if n := present(); n != 0 {
		t.Errorf("%d indexes left after migrating down", n)
	}
	run("000003_indexes_for_existing_lookups.down.sql") // down is repeatable
	run("000003_indexes_for_existing_lookups.up.sql")
	if present() != len(indexes) {
		t.Errorf("%d of %d indexes present after migrating up again", present(), len(indexes))
	}
}

// A statement that cannot get a row lock in time fails with a distinct error, which the API reports
// as 503 instead of letting requests pile up behind one stuck transaction.
func TestLockTimeoutBecomesServiceBusy(t *testing.T) {
	e := newEnv(t)
	room := e.freshRoom()

	var configured string
	if err := e.pool.QueryRow(t.Context(), "SHOW lock_timeout").Scan(&configured); err != nil {
		t.Fatal(err)
	}
	if configured != "5s" {
		t.Errorf("pool lock_timeout = %q; want 5s", configured)
	}

	holder, err := e.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := holder.Rollback(t.Context()); err != nil {
			t.Errorf("releasing the lock: %v", err)
		}
	}()
	if _, err := holder.Exec(t.Context(), "SELECT id FROM rooms WHERE id = $1 FOR UPDATE", room); err != nil {
		t.Fatal(err)
	}

	waiter, err := e.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := waiter.Rollback(t.Context()); err != nil {
			t.Logf("rolling back the waiter: %v", err) // the failed statement already aborted it
		}
	}()
	if _, err := waiter.Exec(t.Context(), "SET LOCAL lock_timeout = '50ms'"); err != nil {
		t.Fatal(err)
	}
	_, lockErr := waiter.Exec(t.Context(), "SELECT id FROM rooms WHERE id = $1 FOR UPDATE", room)
	if lockErr == nil {
		t.Fatal("the second lock was granted")
	}

	status, body := httpx.MapError(fmt.Errorf("locking room: %w", lockErr), "req_1")
	if status != http.StatusServiceUnavailable || body.Error.Code != "SERVICE_BUSY" {
		t.Errorf("mapped to %d %s; want 503 SERVICE_BUSY", status, body.Error.Code)
	}
	if strings.Contains(body.Error.Message, "lock") {
		t.Errorf("message leaks internals: %q", body.Error.Message)
	}
}

// --- last Admin ------------------------------------------------------------------------------------

// Two admins deactivating each other at the same moment would each pass the per-request self check.
// The test holds the guard lock so the first request is stuck on it, lets "the other admin" win in
// the meantime, and then checks that the stuck request refuses to remove the last one.
func TestTheLastAdminCannotBeRemovedByAConcurrentChange(t *testing.T) {
	e := newEnv(t)
	aTok := e.login(adminID).accessToken
	aID := e.userIDOf(adminID)

	body := e.newUserBody("197501012000011002")
	body["role"] = "ADMIN"
	res := e.do("POST", "/api/v1/users", body, bearer(aTok))
	mustStatus(t, res, http.StatusCreated)
	bID := res.json()["id"].(string)

	hold, err := e.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := hold.Rollback(t.Context()); err != nil {
			t.Logf("rollback: %v", err) // already committed below
		}
	}()
	if _, err := hold.Exec(t.Context(), "SELECT pg_advisory_xact_lock(hashtextextended('admin-guard', 0))"); err != nil {
		t.Fatal(err)
	}

	done := make(chan response, 1)
	go func() {
		done <- e.do("PATCH", "/api/v1/users/"+bID, map[string]any{"is_active": false}, bearer(aTok))
	}()

	deadline := time.Now().Add(10 * time.Second)
	for e.countRows("SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND NOT granted") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the request never reached the guard lock")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// The other Admin's request wins first: A is deactivated.
	if _, err := hold.Exec(t.Context(), "UPDATE users SET is_active = false WHERE id = $1", aID); err != nil {
		t.Fatal(err)
	}
	if err := hold.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}

	got := <-done
	if got.Code != http.StatusUnprocessableEntity || !strings.Contains(got.Body.String(), "minimal satu Admin aktif") {
		t.Fatalf("second removal: %d %s; want 422 refusing to remove the last Admin", got.Code, got.Body.String())
	}
	if n := e.countRows("SELECT count(*) FROM users WHERE role = 'ADMIN' AND is_active"); n != 1 {
		t.Errorf("%d active Admin left; want exactly 1", n)
	}
	if n := e.countRows("SELECT count(*) FROM users WHERE id = $1 AND is_active", bID); n != 1 {
		t.Error("the refused request still deactivated the account")
	}
}

// Demoting works as long as someone else stays.
func TestAdminMayBeDemotedWhileAnotherRemains(t *testing.T) {
	e := newEnv(t)
	aTok := e.login(adminID).accessToken
	body := e.newUserBody("197501012000011003")
	body["role"] = "ADMIN"
	res := e.do("POST", "/api/v1/users", body, bearer(aTok))
	mustStatus(t, res, http.StatusCreated)
	bID := res.json()["id"].(string)

	mustStatus(t, e.do("PATCH", "/api/v1/users/"+bID, map[string]any{"role": "TU"}, bearer(aTok)), http.StatusOK)
	if n := e.countRows("SELECT count(*) FROM users WHERE role = 'ADMIN' AND is_active"); n != 1 {
		t.Errorf("%d active Admin; want 1", n)
	}
}

// --- notification stream cap ---------------------------------------------------------------------------

func TestNotificationStreamsArePerUserLimited(t *testing.T) {
	e := newEnv(t)
	tu, student := e.tuToken(), e.login(mahasiswaA).accessToken

	var open []*stream
	for range 5 {
		s := e.openStream(tu)
		if s.status != http.StatusOK {
			t.Fatalf("stream %d refused with %d; five are allowed", len(open)+1, s.status)
		}
		s.nextEvent() // ready
		open = append(open, s)
	}
	if s := e.openStream(tu); s.status != http.StatusTooManyRequests {
		t.Errorf("sixth stream: %d; want 429", s.status)
	}
	if s := e.openStream(student); s.status != http.StatusOK {
		t.Errorf("another user's stream: %d; want 200", s.status)
	}

	// Closing one frees its place once the server notices the disconnect.
	open[0].cancel()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if s := e.openStream(tu); s.status == http.StatusOK {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("closing a stream never freed its place")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
