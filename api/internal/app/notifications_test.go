package app_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"classlab/api/internal/config"
)

// insertNotification writes a notification straight into the database for the user.
func (e *env) insertNotification(userID, title string, createdAt time.Time, read bool) string {
	e.t.Helper()
	var readAt *time.Time
	if read {
		readAt = &createdAt
	}
	var id string
	err := e.pool.QueryRow(e.t.Context(),
		`INSERT INTO notifications (user_id, type, title, body, read_at, created_at)
		 VALUES ($1, 'TEST', $2, 'isi', $3, $4) RETURNING id`, userID, title, readAt, createdAt).Scan(&id)
	if err != nil {
		e.t.Fatalf("inserting notification: %v", err)
	}
	return id
}

func titlesOf(res response) []string {
	var out []string
	for _, it := range dataOf(res) {
		out = append(out, it.(map[string]any)["title"].(string))
	}
	return out
}

func TestListNotifications(t *testing.T) {
	e := newEnv(t)
	a, b := e.userIDOf(mahasiswaA), e.userIDOf(mahasiswaB)
	base := time.Date(2026, 10, 14, 1, 0, 0, 0, time.UTC)
	for i, n := range []struct {
		title string
		read  bool
	}{{"n1", true}, {"n2", false}, {"n3", true}, {"n4", false}, {"n5", false}} {
		e.insertNotification(a, n.title, base.Add(time.Duration(i)*time.Minute), n.read)
	}
	e.insertNotification(b, "milik-b", base, false)
	tok := bearer(e.login(mahasiswaA).accessToken)

	res := e.do("GET", "/api/v1/notifications", nil, tok)
	mustStatus(t, res, http.StatusOK)
	if got := titlesOf(res); !slices.Equal(got, []string{"n5", "n4", "n3", "n2", "n1"}) {
		t.Errorf("titles = %v; want own notifications only, newest first", got)
	}
	if m := res.json()["meta"].(map[string]any); m["total"] != float64(5) || m["page"] != float64(1) {
		t.Errorf("meta = %v", m)
	}

	res = e.do("GET", "/api/v1/notifications?unread=true", nil, tok)
	mustStatus(t, res, http.StatusOK)
	if got := titlesOf(res); !slices.Equal(got, []string{"n5", "n4", "n2"}) || res.json()["meta"].(map[string]any)["total"] != float64(3) {
		t.Errorf("unread = %v (meta %v); want n5 n4 n2", got, res.json()["meta"])
	}
	if got := titlesOf(e.do("GET", "/api/v1/notifications?unread=false", nil, tok)); len(got) != 5 {
		t.Errorf("unread=false returned %v; want all 5", got)
	}

	res = e.do("GET", "/api/v1/notifications?per_page=2&page=3", nil, tok)
	if got := titlesOf(res); !slices.Equal(got, []string{"n1"}) || res.json()["meta"].(map[string]any)["total"] != float64(5) {
		t.Errorf("page 3 = %v; want [n1] of 5", got)
	}

	first := dataOf(e.do("GET", "/api/v1/notifications?per_page=1", nil, tok))[0].(map[string]any)
	if first["read_at"] != nil || first["ref_type"] != nil || first["ref_id"] != nil {
		t.Errorf("unset optional fields must be omitted: %v", first)
	}
	if first["created_at"] != "2026-10-14T08:04:00+07:00" {
		t.Errorf("created_at = %v; want WIB", first["created_at"])
	}

	for name, q := range map[string]string{"bad unread": "?unread=maybe", "bad page": "?page=0", "huge page": "?per_page=101"} {
		if res := e.do("GET", "/api/v1/notifications"+q, nil, tok); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d; want 422", name, res.Code)
		}
	}
	if res := e.do("GET", "/api/v1/notifications", nil); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d; want 401", res.Code)
	}
}

func TestMarkNotificationRead(t *testing.T) {
	e := newEnv(t)
	a, b := e.userIDOf(mahasiswaA), e.userIDOf(mahasiswaB)
	mine := e.insertNotification(a, "mine", startTime, false)
	theirs := e.insertNotification(b, "theirs", startTime, false)
	tok := bearer(e.login(mahasiswaA).accessToken)

	res := e.do("POST", "/api/v1/notifications/"+mine+"/read", nil, tok)
	mustStatus(t, res, http.StatusOK)
	first := res.json()["read_at"]
	if first != "2026-10-14T10:00:00+07:00" || res.json()["id"] != mine {
		t.Errorf("read response = %v", res.json())
	}

	// Marking again is harmless and keeps the first timestamp.
	e.clock.Add(time.Hour)
	tok = bearer(e.login(mahasiswaA).accessToken) // the old token expired with the clock jump
	res = e.do("POST", "/api/v1/notifications/"+mine+"/read", nil, tok)
	mustStatus(t, res, http.StatusOK)
	if res.json()["read_at"] != first {
		t.Errorf("read_at moved from %v to %v on a repeat", first, res.json()["read_at"])
	}

	// Another user's notification and a missing one look the same, and the former stays untouched.
	for name, id := range map[string]string{"someone else's": theirs, "missing": "00000000-0000-4000-8000-0000000000aa", "malformed": "x"} {
		if res := e.do("POST", "/api/v1/notifications/"+id+"/read", nil, tok); res.Code != http.StatusNotFound {
			t.Errorf("%s: %d; want 404", name, res.Code)
		}
	}
	if n := e.countRows("SELECT count(*) FROM notifications WHERE id = $1 AND read_at IS NULL", theirs); n != 1 {
		t.Error("a stranger marked another user's notification as read")
	}
	if res := e.do("POST", "/api/v1/notifications/"+mine+"/read", nil); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d; want 401", res.Code)
	}
}

func TestMarkAllNotificationsRead(t *testing.T) {
	e := newEnv(t)
	a, b := e.userIDOf(mahasiswaA), e.userIDOf(mahasiswaB)
	e.insertNotification(a, "a1", startTime, false)
	e.insertNotification(a, "a2", startTime, false)
	e.insertNotification(a, "a3", startTime, true)
	e.insertNotification(b, "b1", startTime, false)
	tok := bearer(e.login(mahasiswaA).accessToken)

	res := e.do("POST", "/api/v1/notifications/read-all", nil, tok)
	mustStatus(t, res, http.StatusOK)
	if res.json()["updated"] != float64(2) {
		t.Errorf("updated = %v; want 2 (the one already read does not count)", res.json()["updated"])
	}
	if n := e.countRows("SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL", a); n != 0 {
		t.Errorf("%d notifications left unread", n)
	}
	if n := e.countRows("SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL", b); n != 1 {
		t.Error("read-all touched another user's notifications")
	}
	if res := e.do("POST", "/api/v1/notifications/read-all", nil, tok); res.json()["updated"] != float64(0) {
		t.Errorf("second call updated %v; want 0", res.json()["updated"])
	}
}

// --- live stream -----------------------------------------------------------------------------

type sseFrame struct {
	event, id, data string
	comment         bool
}

// stream is an open GET /notifications/stream, read through a real HTTP server because the
// recorder used elsewhere cannot flush.
type stream struct {
	t      *testing.T
	frames chan sseFrame
	cancel context.CancelFunc
	status int
}

func (e *env) openStream(token string) *stream {
	e.t.Helper()
	srv := httptest.NewServer(e.router)
	e.t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(e.t.Context())
	e.t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/v1/notifications/stream", nil)
	if err != nil {
		e.t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("opening stream: %v", err)
	}

	s := &stream{t: e.t, frames: make(chan sseFrame, 64), cancel: cancel, status: resp.StatusCode}
	if resp.StatusCode != http.StatusOK {
		if err := resp.Body.Close(); err != nil {
			e.t.Fatal(err)
		}
		close(s.frames)
		return s
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		e.t.Fatalf("stream Content-Type = %q", ct)
	}
	go func() {
		defer close(s.frames)
		defer resp.Body.Close()
		var cur sseFrame
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if cur.event != "" {
					s.frames <- cur
				}
				cur = sseFrame{}
			case strings.HasPrefix(line, ":"):
				s.frames <- sseFrame{comment: true, data: strings.TrimSpace(line[1:])}
			case strings.HasPrefix(line, "event: "):
				cur.event = line[len("event: "):]
			case strings.HasPrefix(line, "id: "):
				cur.id = line[len("id: "):]
			case strings.HasPrefix(line, "data: "):
				cur.data = line[len("data: "):]
			}
		}
	}()
	return s
}

// next returns the next frame, failing the test if none arrives or the stream has ended.
func (s *stream) next() sseFrame {
	s.t.Helper()
	select {
	case f, ok := <-s.frames:
		if !ok {
			s.t.Fatal("stream ended; wanted another frame")
		}
		return f
	case <-time.After(10 * time.Second):
		s.t.Fatal("timed out waiting for a stream frame")
		return sseFrame{}
	}
}

// nextEvent skips keep-alive comments.
func (s *stream) nextEvent() sseFrame {
	s.t.Helper()
	for {
		if f := s.next(); !f.comment {
			return f
		}
	}
}

func (s *stream) expectEnd() {
	s.t.Helper()
	select {
	case f, ok := <-s.frames:
		if ok && !f.comment {
			s.t.Fatalf("expected the stream to end, got %+v", f)
		}
		if ok {
			s.expectEnd()
		}
	case <-time.After(10 * time.Second):
		s.t.Fatal("stream did not end")
	}
}

func (f sseFrame) json() map[string]any {
	var out map[string]any
	if err := json.Unmarshal([]byte(f.data), &out); err != nil {
		panic("frame data is not a JSON object: " + f.data)
	}
	return out
}

func TestNotificationStreamDeliversOnlyToTheRecipient(t *testing.T) {
	e := newEnv(t)
	e.insertNotification(e.userIDOf(tuKalabID), "lama", startTime, false)
	e.insertNotification(e.userIDOf(tuKalabID), "lama 2", startTime, true)
	student := e.login(mahasiswaA).accessToken
	admin := e.adminToken()

	tu := e.openStream(admin)
	if f := tu.nextEvent(); f.event != "ready" || f.json()["unread"] != float64(1) {
		t.Fatalf("first frame = %+v; want ready with 1 unread", f)
	}
	stu := e.openStream(student)
	if f := stu.nextEvent(); f.event != "ready" || f.json()["unread"] != float64(0) {
		t.Fatalf("student's first frame = %+v; want ready with 0 unread", f)
	}

	// A booking notifies the approvers, not the student who made it.
	booking := idOf(e.postBooking(student, e.freshRoom(), at(tue, "08:00"), at(tue, "10:00"), nil))
	f := tu.nextEvent()
	if f.event != "notification" || f.json()["type"] != "BOOKING_CREATED" || f.json()["ref_id"] != booking {
		t.Fatalf("approver's frame = %+v; want BOOKING_CREATED for %s", f, booking)
	}
	var dbID string
	if err := e.pool.QueryRow(t.Context(), "SELECT id FROM notifications WHERE type = 'BOOKING_CREATED' AND user_id = $1", e.userIDOf(tuKalabID)).Scan(&dbID); err != nil {
		t.Fatal(err)
	}
	if f.id != dbID || f.json()["id"] != dbID {
		t.Errorf("frame id %q / body id %v; want the stored notification %s", f.id, f.json()["id"], dbID)
	}

	// The decision reaches the student; had the earlier notification leaked to them, it would arrive first.
	mustStatus(t, e.action(admin, booking, "approve", nil), http.StatusOK)
	if f := stu.nextEvent(); f.event != "notification" || f.json()["type"] != "BOOKING_APPROVED" {
		t.Fatalf("student's frame = %+v; want BOOKING_APPROVED as the first notification", f)
	}
}

func TestNotificationStreamIgnoresRolledBackNotifications(t *testing.T) {
	e := newEnv(t)
	student := e.login(mahasiswaA).accessToken
	tu := e.openStream(e.adminToken())
	tu.nextEvent() // ready

	// With the notification insert failing, the whole booking rolls back and nothing is announced.
	if _, err := e.pool.Exec(t.Context(), `CREATE FUNCTION boom() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'injected'; END $$ LANGUAGE plpgsql;
		CREATE TRIGGER boom BEFORE INSERT ON notifications FOR EACH ROW EXECUTE FUNCTION boom()`); err != nil {
		t.Fatal(err)
	}
	res := e.doUnchecked("POST", "/api/v1/bookings", bookingBody(e.freshRoom(), at(tue, "08:00"), at(tue, "10:00"), nil), bearer(student))
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("booking with failing notification: %d; want 500", res.Code)
	}
	if _, err := e.pool.Exec(t.Context(), "DROP TRIGGER boom ON notifications; DROP FUNCTION boom()"); err != nil {
		t.Fatal(err)
	}

	good := idOf(e.postBooking(student, e.freshRoom(), at(tue, "11:00"), at(tue, "12:00"), nil))
	if f := tu.nextEvent(); f.json()["ref_id"] != good {
		t.Errorf("first notification on the stream is for %v; want the committed booking %s", f.json()["ref_id"], good)
	}
}

func TestNotificationStreamRequiresAuthentication(t *testing.T) {
	e := newEnv(t)
	if s := e.openStream(""); s.status != http.StatusUnauthorized {
		t.Errorf("anonymous stream: %d; want 401", s.status)
	}
	if s := e.openStream("not-a-token"); s.status != http.StatusUnauthorized {
		t.Errorf("bad token: %d; want 401", s.status)
	}
}

func TestNotificationStreamKeepAliveAndLifetime(t *testing.T) {
	e := newEnv(t, func(c *config.Config) {
		c.SSEHeartbeat = 20 * time.Millisecond
		c.JWTAccessExpiry = 1500 * time.Millisecond // also the stream lifetime; tokens follow the frozen test clock (whole seconds)
	})
	s := e.openStream(e.login(mahasiswaA).accessToken)
	if f := s.next(); f.event != "ready" {
		t.Fatalf("first frame = %+v", f)
	}
	if f := s.next(); !f.comment || f.data != "keep-alive" {
		t.Errorf("frame = %+v; want a keep-alive comment", f)
	}
	for {
		f := s.next()
		if f.comment {
			continue
		}
		if f.event != "reconnect" {
			t.Fatalf("frame = %+v; want reconnect when the lifetime is up", f)
		}
		break
	}
	s.expectEnd()
}

// If the listener loses its connection, streams are told to refetch once it is back.
func TestNotificationStreamResyncsAfterListenerRecovers(t *testing.T) {
	e := newEnv(t)
	student := e.login(mahasiswaA).accessToken
	tu := e.openStream(e.adminToken())
	tu.nextEvent() // ready

	var killed int
	if err := e.pool.QueryRow(t.Context(),
		`SELECT count(pg_terminate_backend(pid)) FROM pg_stat_activity
		 WHERE query = 'LISTEN classlab_notifications' AND pid <> pg_backend_pid()`).Scan(&killed); err != nil {
		t.Fatal(err)
	}
	if killed != 1 {
		t.Fatalf("terminated %d listener connections; want 1", killed)
	}

	if f := tu.nextEvent(); f.event != "resync" {
		t.Fatalf("frame = %+v; want resync after the listener reconnects", f)
	}
	booking := idOf(e.postBooking(student, e.freshRoom(), at(tue, "08:00"), at(tue, "10:00"), nil))
	if f := tu.nextEvent(); f.event != "notification" || f.json()["ref_id"] != booking {
		t.Errorf("after recovery: %+v; want the new booking's notification", f)
	}
}
