package integration_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"classlab/api/internal/app"
	"classlab/api/internal/config"
	"classlab/api/internal/jobs"
)

func (e *env) jobRunner() *jobs.Runner {
	e.t.Helper()
	return app.NewJobRunner(e.cfg, e.pool, slog.New(slog.NewTextHandler(io.Discard, nil)), e.clock)
}

// runJobs moves the clock to a moment and runs every step once, as the real runner does each minute.
func (e *env) runJobs(r *jobs.Runner, date, hms string) jobs.Result {
	e.t.Helper()
	e.setNow(date, hms)
	res, err := r.RunOnce(e.t.Context(), e.clock.Now())
	if err != nil {
		e.t.Fatalf("RunOnce at %s %s: %v", date, hms, err)
	}
	return res
}

func (e *env) reminders(bookingID, typ string) int {
	e.t.Helper()
	return e.countRows("SELECT count(*) FROM notifications WHERE type = $1 AND ref_id = $2", typ, bookingID)
}

func (e *env) violationPoints(student string) int {
	e.t.Helper()
	return e.countRows("SELECT COALESCE(SUM(points), 0)::int FROM violations WHERE user_id = $1", e.userIDOf(student))
}

// Booking approved, nobody came: TIDAK_HADIR as soon as the window has closed, with two points (BR-13 a).
func TestNoShowJob(t *testing.T) {
	e := newEnv(t)
	r := e.jobRunner()
	room := e.freshRoom()
	id := e.approvedIn(room, mahasiswaA, at(tue, "08:00"), at(tue, "10:00"))
	checkedIn := e.approvedIn(e.freshRoom(), mahasiswaB, at(tue, "08:00"), at(tue, "10:00"))
	code, qr := e.roomQR(e.roomOfBooking(checkedIn))
	e.setNow(tue, "08:05:00")
	mustStatus(t, e.checkIn(e.login(mahasiswaB).accessToken, code, qr), http.StatusOK)

	if res := e.runJobs(r, tue, "08:15:00"); res.NoShows != 0 || e.bookingStatus(id) != "DISETUJUI" {
		t.Fatalf("at the last moment of the window: %+v, status %s; want nothing yet", res, e.bookingStatus(id))
	}
	if res := e.runJobs(r, tue, "08:15:01"); res.NoShows != 1 {
		t.Fatalf("one second later: %+v; want one no-show", res)
	}
	if e.bookingStatus(id) != "TIDAK_HADIR" || e.bookingStatus(checkedIn) != "BERLANGSUNG" {
		t.Errorf("statuses = %s and %s; want TIDAK_HADIR and BERLANGSUNG", e.bookingStatus(id), e.bookingStatus(checkedIn))
	}

	student := e.userIDOf(mahasiswaA)
	if n := e.countRows("SELECT count(*) FROM violations WHERE booking_id = $1 AND kind = 'TIDAK_HADIR' AND points = 2 AND user_id = $2", id, student); n != 1 {
		t.Errorf("%d no-show violations of 2 points; want 1", n)
	}
	var actor *string
	var from, to, reason string
	if err := e.pool.QueryRow(t.Context(),
		"SELECT actor_id::text, from_status::text, to_status::text, COALESCE(reason, '') FROM booking_events WHERE booking_id = $1 AND to_status = 'TIDAK_HADIR'", id).
		Scan(&actor, &from, &to, &reason); err != nil {
		t.Fatal(err)
	}
	if actor != nil || from != "DISETUJUI" {
		t.Errorf("event actor %v from %s; want the system (NULL) from DISETUJUI", actor, from)
	}
	if e.reminders(id, "BOOKING_NO_SHOW") != 1 || e.notificationCount(student, "BOOKING_NO_SHOW") != 1 {
		t.Error("the student was not told once")
	}

	// A second run does nothing: no second violation, event or notification (BR-13, agent.md 7 #10).
	events := e.eventCount(id)
	if res := e.runJobs(r, tue, "08:16:00"); res.NoShows != 0 {
		t.Errorf("second run: %+v; want nothing", res)
	}
	if e.violationPoints(mahasiswaA) != 2 || e.eventCount(id) != events || e.reminders(id, "BOOKING_NO_SHOW") != 1 {
		t.Error("a second run repeated the effects")
	}

	// The slot is free again from the next valid time (BR-01).
	mustStatus(t, e.postBooking(e.login(mahasiswaB).accessToken, room, at(tue, "09:00"), at(tue, "10:00"), nil), http.StatusCreated)
}

func (e *env) roomOfBooking(id string) string {
	e.t.Helper()
	var room string
	if err := e.pool.QueryRow(e.t.Context(), "SELECT room_id::text FROM bookings WHERE id = $1", id).Scan(&room); err != nil {
		e.t.Fatal(err)
	}
	return room
}

// In use and never closed: TANPA_CHECKOUT once end_at + 15 minutes has passed, with one point (BR-13 b).
func TestMissedCheckoutJob(t *testing.T) {
	e := newEnv(t)
	r := e.jobRunner()
	id := e.inUse(e.freshRoom(), mahasiswaA)
	left := e.inUse(e.freshRoom(), mahasiswaB)
	e.setNow(tue, "09:30:00")
	mustStatus(t, e.checkOut(e.login(mahasiswaB).accessToken, left, map[string]string{"condition": "NORMAL"}, goodPhoto()), http.StatusOK)

	if res := e.runJobs(r, tue, "10:15:00"); res.MissedCheckouts != 0 || e.bookingStatus(id) != "BERLANGSUNG" {
		t.Fatalf("at end + grace: %+v, status %s; want nothing yet", res, e.bookingStatus(id))
	}
	if res := e.runJobs(r, tue, "10:15:01"); res.MissedCheckouts != 1 {
		t.Fatalf("one second later: %+v; want one", res)
	}
	if e.bookingStatus(id) != "TANPA_CHECKOUT" || e.bookingStatus(left) != "SELESAI" {
		t.Errorf("statuses = %s and %s; want TANPA_CHECKOUT and SELESAI", e.bookingStatus(id), e.bookingStatus(left))
	}
	if n := e.countRows("SELECT count(*) FROM violations WHERE booking_id = $1 AND kind = 'TANPA_CHECKOUT' AND points = 1", id); n != 1 {
		t.Errorf("%d violations of 1 point; want 1", n)
	}
	if n := e.countRows("SELECT count(*) FROM booking_events WHERE booking_id = $1 AND to_status = 'TANPA_CHECKOUT' AND actor_id IS NULL", id); n != 1 {
		t.Errorf("%d system events; want 1", n)
	}
	if e.reminders(id, "BOOKING_AUTO_CLOSED") != 1 || e.violationPoints(mahasiswaB) != 0 {
		t.Error("expected one notice for the first student and no points for the second")
	}

	e.runJobs(r, tue, "10:20:00")
	if e.violationPoints(mahasiswaA) != 1 || e.reminders(id, "BOOKING_AUTO_CLOSED") != 1 {
		t.Error("a second run repeated the effects")
	}
}

// Enough points block new requests for BLOCK_DAYS, announced once; existing approved bookings stay (BR-14, #11).
func TestViolationsBlockTheStudent(t *testing.T) {
	e := newEnv(t)
	r := e.jobRunner()
	var ids []string
	for i := 0; i < 3; i++ {
		ids = append(ids, e.approvedIn(e.freshRoom(), mahasiswaA, at(tue, "08:00"), at(tue, "10:00")))
	}
	future := e.approvedIn(e.freshRoom(), mahasiswaA, at(wed, "08:00"), at(wed, "10:00"))
	student := e.userIDOf(mahasiswaA)

	res := e.runJobs(r, tue, "08:15:01")
	if res.NoShows != 3 {
		t.Fatalf("%+v; want three no-shows", res)
	}
	if e.violationPoints(mahasiswaA) != 6 {
		t.Fatalf("points = %d; want 6", e.violationPoints(mahasiswaA))
	}
	var until time.Time
	if err := e.pool.QueryRow(t.Context(), "SELECT booking_blocked_until FROM users WHERE id = $1", student).Scan(&until); err != nil {
		t.Fatalf("not blocked: %v", err)
	}
	if want := wibAt(tue, "08:15:01").Add(14 * 24 * time.Hour); !until.Equal(want) {
		t.Errorf("blocked until %v; want %v", until, want)
	}
	if n := e.notificationCount(student, "BOOKING_BLOCKED"); n != 1 {
		t.Errorf("%d BOOKING_BLOCKED notifications; want exactly 1", n)
	}

	tok := e.login(mahasiswaA).accessToken
	blocked := e.postBooking(tok, e.freshRoom(), at(wed, "13:00"), at(wed, "14:00"), nil)
	if blocked.Code != http.StatusForbidden || blocked.errorCode() != "BOOKING_BLOCKED" {
		t.Errorf("new booking: %d %s; want 403 BOOKING_BLOCKED", blocked.Code, blocked.errorCode())
	}
	if e.bookingStatus(future) != "DISETUJUI" {
		t.Errorf("the approved booking became %s; it must stay", e.bookingStatus(future))
	}
	me := e.do("GET", "/api/v1/auth/me", nil, bearer(tok))
	if me.json()["active_points"] != float64(6) || me.json()["booking_blocked_until"] == nil {
		t.Errorf("/auth/me = %v; want 6 points and the block", me.json())
	}

	// Annulling a violation takes its points away; the block itself is lifted by the administrator.
	var first string
	if err := e.pool.QueryRow(t.Context(), "SELECT id::text FROM violations WHERE booking_id = $1", ids[0]).Scan(&first); err != nil {
		t.Fatal(err)
	}
	mustStatus(t, e.do("POST", "/api/v1/violations/"+first+"/annul", map[string]string{"reason": "Salah catat"}, bearer(e.adminToken())), http.StatusOK)
	if got := e.do("GET", "/api/v1/auth/me", nil, bearer(tok)).json()["active_points"]; got != float64(4) {
		t.Errorf("active_points after the annulment = %v; want 4", got)
	}
	mustStatus(t, e.do("PATCH", "/api/v1/users/"+student, map[string]any{"clear_booking_block": true}, bearer(e.adminToken())), http.StatusOK)
	mustStatus(t, e.postBooking(tok, e.freshRoom(), at(wed, "13:00"), at(wed, "14:00"), nil), http.StatusCreated)

	// Running again later does not announce the block a second time.
	e.runJobs(r, tue, "08:30:00")
	if n := e.notificationCount(student, "BOOKING_BLOCKED"); n != 1 {
		t.Errorf("%d BOOKING_BLOCKED notifications after another run; want 1", n)
	}
}

// Waiting past expires_at: KEDALUWARSA with no points, slot and quota released (BR-22, #14).
func TestPendingRequestsExpire(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Business.MaxActiveBookings = 1 })
	r := e.jobRunner()
	room := e.freshRoom()
	student := e.login(mahasiswaA).accessToken

	// Filed at 2026-10-14 10:00 for next week: it waits 24 hours.
	id := idOf(e.postBooking(student, room, at(tue, "08:00"), at(tue, "10:00"), nil))
	if id == "" {
		t.Fatal("could not file the request")
	}
	var expires time.Time
	if err := e.pool.QueryRow(t.Context(), "SELECT expires_at FROM bookings WHERE id = $1", id).Scan(&expires); err != nil {
		t.Fatal(err)
	}
	if want := wibAt("2026-10-14", "10:00:00").Add(24 * time.Hour); !expires.Equal(want) {
		t.Fatalf("expires_at = %v; want %v", expires, want)
	}
	if res := e.postBooking(student, e.freshRoom(), at(wed, "08:00"), at(wed, "10:00"), nil); res.Code != http.StatusConflict || res.errorCode() != "QUOTA_EXCEEDED" {
		t.Fatalf("second request while waiting: %d %s; want 409 QUOTA_EXCEEDED", res.Code, res.errorCode())
	}

	if res := e.runJobs(r, "2026-10-15", "10:00:00"); res.Expired != 0 || e.bookingStatus(id) != "MENUNGGU" {
		t.Fatalf("at expires_at: %+v, status %s; want still waiting", res, e.bookingStatus(id))
	}
	// Past expires_at but before the job has run, the approver cannot decide any more.
	e.setNow("2026-10-15", "10:00:01")
	if res := e.action(e.tuToken(), id, "approve", nil); res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
		t.Errorf("approving a request past expires_at: %d %s; want 409 INVALID_STATE", res.Code, res.errorCode())
	}
	if res := e.runJobs(r, "2026-10-15", "10:00:01"); res.Expired != 1 {
		t.Fatalf("one second after: %+v; want one expired", res)
	}
	if e.bookingStatus(id) != "KEDALUWARSA" {
		t.Errorf("status = %s; want KEDALUWARSA", e.bookingStatus(id))
	}
	if n := e.countRows("SELECT count(*) FROM booking_events WHERE booking_id = $1 AND from_status = 'MENUNGGU' AND to_status = 'KEDALUWARSA' AND actor_id IS NULL", id); n != 1 {
		t.Errorf("%d system events; want 1", n)
	}
	if e.violationPoints(mahasiswaA) != 0 {
		t.Error("expiry must not cost points")
	}
	if e.reminders(id, "BOOKING_EXPIRED") != 1 {
		t.Error("the student was not told")
	}

	// Slot and quota are free again: the same student files again, and another student takes the same slot.
	student = e.login(mahasiswaA).accessToken
	mustStatus(t, e.postBooking(student, e.freshRoom(), at(wed, "08:00"), at(wed, "10:00"), nil), http.StatusCreated)
	mustStatus(t, e.postBooking(e.login(mahasiswaB).accessToken, room, at(tue, "08:00"), at(tue, "10:00"), nil), http.StatusCreated)

	events := e.eventCount(id)
	if res := e.runJobs(r, "2026-10-15", "10:05:00"); res.Expired != 0 {
		t.Errorf("second run: %+v; want nothing", res)
	}
	if e.eventCount(id) != events || e.reminders(id, "BOOKING_EXPIRED") != 1 {
		t.Error("a second run repeated the effects")
	}
}

// A request that starts within the day expires at its start rather than a day later (BR-22).
func TestPendingRequestExpiresAtItsStartWhenSooner(t *testing.T) {
	e := newEnv(t)
	r := e.jobRunner()
	e.setNow(tue, "07:00:00")
	id := idOf(e.postBooking(e.login(mahasiswaA).accessToken, e.freshRoom(), at(tue, "08:00"), at(tue, "10:00"), nil))

	if res := e.runJobs(r, tue, "08:00:00"); res.Expired != 0 {
		t.Fatalf("at the start: %+v; want nothing", res)
	}
	if res := e.runJobs(r, tue, "08:00:01"); res.Expired != 1 || e.bookingStatus(id) != "KEDALUWARSA" {
		t.Fatalf("one second after the start: %+v, status %s; want expired", res, e.bookingStatus(id))
	}
}

// A multi-date request expires as one, and its owner hears about it once (BR-22, BR-27).
func TestGroupExpiresTogether(t *testing.T) {
	e := newEnv(t)
	r := e.jobRunner()
	tok := e.login(mahasiswaA).accessToken
	room := e.freshRoom()
	res := e.do("POST", "/api/v1/bookings", map[string]any{
		"room_id": room, "purpose": "Latihan rutin",
		"occurrences": []map[string]string{
			{"start_at": at(tue, "08:00"), "end_at": at(tue, "10:00")},
			{"start_at": at(wed, "08:00"), "end_at": at(wed, "10:00")},
			{"start_at": at("2026-10-22", "08:00"), "end_at": at("2026-10-22", "10:00")},
		},
	}, bearer(tok))
	mustStatus(t, res, http.StatusCreated)
	group := res.json()["group_id"].(string)

	if got := e.runJobs(r, "2026-10-15", "10:00:01"); got.Expired != 3 {
		t.Fatalf("%+v; want all three dates expired", got)
	}
	if n := e.countRows("SELECT count(*) FROM bookings WHERE group_id = $1 AND status = 'KEDALUWARSA'", group); n != 3 {
		t.Errorf("%d dates expired; want 3", n)
	}
	if n := e.notificationCount(e.userIDOf(mahasiswaA), "BOOKING_EXPIRED"); n != 1 {
		t.Errorf("%d BOOKING_EXPIRED notifications for the group; want 1", n)
	}
	if e.violationPoints(mahasiswaA) != 0 {
		t.Error("expiry must not cost points")
	}
}

// Reminders appear at the right time, once, and not for bookings they no longer concern (BR-28, #20).
func TestReminders(t *testing.T) {
	e := newEnv(t)
	r := e.jobRunner()

	notYet := e.approvedIn(e.freshRoom(), mahasiswaA, at(tue, "13:00"), at(tue, "15:00"))
	cancelled := e.approvedIn(e.freshRoom(), mahasiswaB, at(tue, "13:00"), at(tue, "15:00"))
	inUse := e.approvedIn(e.freshRoom(), orgID, at(tue, "13:00"), at(tue, "15:00"))
	code, qr := e.roomQR(e.roomOfBooking(inUse))

	// Before the window opens there is nothing.
	e.runJobs(r, tue, "12:44:59")
	if n := e.countRows("SELECT count(*) FROM notifications WHERE type LIKE 'REMINDER_%'"); n != 0 {
		t.Fatalf("%d reminders before the window opened; want none", n)
	}

	// The window opens at 12:45: one reminder each for the bookings still approved.
	mustStatus(t, e.do("POST", "/api/v1/bookings/"+cancelled+"/cancel", nil, bearer(e.login(mahasiswaB).accessToken)), http.StatusOK)
	e.runJobs(r, tue, "12:45:00")
	e.runJobs(r, tue, "12:45:30")
	e.runJobs(r, tue, "12:50:00")
	if e.reminders(notYet, "REMINDER_CHECKIN") != 1 || e.reminders(inUse, "REMINDER_CHECKIN") != 1 || e.reminders(cancelled, "REMINDER_CHECKIN") != 0 {
		t.Errorf("check-in reminders = %d, %d, %d; want 1, 1, 0 (the cancelled booking gets none)",
			e.reminders(notYet, "REMINDER_CHECKIN"), e.reminders(inUse, "REMINDER_CHECKIN"), e.reminders(cancelled, "REMINDER_CHECKIN"))
	}

	// The organisation checks in; the student does not.
	e.setNow(tue, "12:55:00")
	mustStatus(t, e.checkIn(e.orgToken(), code, qr), http.StatusOK)

	// Five minutes before the window closes (13:15): the last call goes to those who have not checked in.
	e.runJobs(r, tue, "13:09:59")
	if e.reminders(notYet, "REMINDER_CHECKIN_LAST") != 0 {
		t.Error("the last-call reminder came too early")
	}
	e.runJobs(r, tue, "13:10:00")
	e.runJobs(r, tue, "13:11:00")
	if e.reminders(notYet, "REMINDER_CHECKIN_LAST") != 1 || e.reminders(inUse, "REMINDER_CHECKIN_LAST") != 0 || e.reminders(cancelled, "REMINDER_CHECKIN_LAST") != 0 {
		t.Errorf("last-call reminders = %d, %d, %d; want 1, 0, 0", e.reminders(notYet, "REMINDER_CHECKIN_LAST"), e.reminders(inUse, "REMINDER_CHECKIN_LAST"), e.reminders(cancelled, "REMINDER_CHECKIN_LAST"))
	}

	// Ten minutes before the end: the check-out reminder, only for the booking in use.
	e.runJobs(r, tue, "14:49:59")
	if e.reminders(inUse, "REMINDER_CHECKOUT") != 0 {
		t.Error("the check-out reminder came too early")
	}
	e.runJobs(r, tue, "14:50:00")
	e.runJobs(r, tue, "14:51:00")
	if e.reminders(inUse, "REMINDER_CHECKOUT") != 1 || e.reminders(notYet, "REMINDER_CHECKOUT") != 0 {
		t.Errorf("check-out reminders = %d, %d; want 1, 0", e.reminders(inUse, "REMINDER_CHECKOUT"), e.reminders(notYet, "REMINDER_CHECKOUT"))
	}

	// The reminders reach the right person, with the booking as reference.
	var user, refType string
	if err := e.pool.QueryRow(t.Context(), "SELECT user_id::text, ref_type FROM notifications WHERE type = 'REMINDER_CHECKOUT' AND ref_id = $1", inUse).Scan(&user, &refType); err != nil {
		t.Fatal(err)
	}
	if user != e.userIDOf(orgID) || refType != "BOOKING" {
		t.Errorf("reminder went to %s about a %s; want the organisation and a BOOKING", user, refType)
	}
	total := e.countRows("SELECT count(*) FROM notifications WHERE type LIKE 'REMINDER_%'")
	e.runJobs(r, tue, "14:52:00")
	if e.countRows("SELECT count(*) FROM notifications WHERE type LIKE 'REMINDER_%'") != total {
		t.Error("another run created more reminders")
	}
}

// A booking checked out before the reminder time is not reminded to check out.
func TestNoCheckoutReminderAfterCheckout(t *testing.T) {
	e := newEnv(t)
	r := e.jobRunner()
	id := e.inUse(e.freshRoom(), mahasiswaA)
	e.setNow(tue, "09:00:00")
	mustStatus(t, e.checkOut(e.login(mahasiswaA).accessToken, id, map[string]string{"condition": "NORMAL"}, goodPhoto()), http.StatusOK)

	e.runJobs(r, tue, "09:55:00")
	if e.reminders(id, "REMINDER_CHECKOUT") != 0 {
		t.Error("a checked-out booking was reminded to check out")
	}
}

// Two instances running at the same time handle each booking once (BR-13, #10).
func TestJobsWithTwoInstances(t *testing.T) {
	e := newEnv(t)
	const perStudent = 4
	students := []string{mahasiswaA, mahasiswaB, "2021101003"}
	var ids []string
	for _, s := range students {
		for i := 0; i < perStudent; i++ {
			ids = append(ids, e.approvedIn(e.freshRoom(), s, at(tue, "08:00"), at(tue, "10:00")))
		}
	}
	e.setNow(tue, "08:30:00")

	a, b := e.jobRunner(), e.jobRunner()
	var wg sync.WaitGroup
	var total [2]int
	for i, r := range []*jobs.Runner{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := r.RunOnce(context.Background(), e.clock.Now())
			if err != nil {
				t.Errorf("instance %d: %v", i, err)
			}
			total[i] = res.NoShows
		}()
	}
	wg.Wait()

	// Whatever the split between the instances, every booking was handled exactly once.
	if total[0]+total[1] > len(ids) {
		t.Errorf("instances reported %d and %d; together more than the %d bookings", total[0], total[1], len(ids))
	}
	// A further run picks up anything an instance skipped because the other held its row.
	if res := e.runJobs(a, tue, "08:31:00"); total[0]+total[1]+res.NoShows != len(ids) {
		t.Errorf("handled %d + %d + %d; want %d in all", total[0], total[1], res.NoShows, len(ids))
	}
	for _, id := range ids {
		if e.bookingStatus(id) != "TIDAK_HADIR" {
			t.Fatalf("booking %s is %s", id, e.bookingStatus(id))
		}
		if e.eventCount(id) != 1 || e.reminders(id, "BOOKING_NO_SHOW") != 1 ||
			e.countRows("SELECT count(*) FROM violations WHERE booking_id = $1", id) != 1 {
			t.Errorf("booking %s has %d events, %d notices, %d violations; want 1 each", id, e.eventCount(id),
				e.reminders(id, "BOOKING_NO_SHOW"), e.countRows("SELECT count(*) FROM violations WHERE booking_id = $1", id))
		}
	}
	for _, s := range students {
		if e.violationPoints(s) != perStudent*2 {
			t.Errorf("%s has %d points; want %d", s, e.violationPoints(s), perStudent*2)
		}
		if n := e.notificationCount(e.userIDOf(s), "BOOKING_BLOCKED"); n != 1 {
			t.Errorf("%s got %d block notices; want 1", s, n)
		}
	}
}

// A row someone else is working on is left alone, not waited for and not touched (FOR UPDATE SKIP LOCKED).
func TestJobsSkipLockedRows(t *testing.T) {
	e := newEnv(t)
	r := e.jobRunner()
	held := e.approvedIn(e.freshRoom(), mahasiswaA, at(tue, "08:00"), at(tue, "10:00"))
	free := e.approvedIn(e.freshRoom(), mahasiswaB, at(tue, "08:00"), at(tue, "10:00"))
	e.setNow(tue, "08:30:00")

	// Another transaction (say a cancellation) holds the first booking.
	tx, err := e.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(t.Context(), "SELECT 1 FROM bookings WHERE id = $1 FOR UPDATE", held); err != nil {
		t.Fatal(err)
	}

	done := make(chan jobs.Result, 1)
	go func() {
		res, err := r.RunOnce(context.Background(), e.clock.Now())
		if err != nil {
			t.Error(err)
		}
		done <- res
	}()
	select {
	case res := <-done:
		if res.NoShows != 1 {
			t.Errorf("%+v; want only the free booking handled", res)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the run waited for a locked row instead of skipping it")
	}
	if e.bookingStatus(free) != "TIDAK_HADIR" {
		t.Errorf("free booking is %s; want TIDAK_HADIR", e.bookingStatus(free))
	}
	if err := tx.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	if e.bookingStatus(held) != "DISETUJUI" {
		t.Errorf("held booking is %s; want it untouched", e.bookingStatus(held))
	}
	if res := e.runJobs(r, tue, "08:31:00"); res.NoShows != 1 || e.bookingStatus(held) != "TIDAK_HADIR" {
		t.Errorf("next tick: %+v, status %s; want the held booking handled now", res, e.bookingStatus(held))
	}
}
