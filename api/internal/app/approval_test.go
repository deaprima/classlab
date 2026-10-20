package app_test

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"classlab/api/internal/config"
)

// addViolation gives a user one active violation point, on a booking made only to hang it on.
func (e *env) addViolation(identifier string) {
	e.t.Helper()
	b := e.insertBooking(e.insertRoom("VIO-"+identifier, "KELAS", 10), identifier, "SELESAI", "Fixture", at(mon, "08:00"), at(mon, "09:00"))
	if _, err := e.pool.Exec(e.t.Context(),
		`INSERT INTO violations (user_id, booking_id, kind, points, created_at) VALUES ($1, $2, 'TIDAK_HADIR', 1, '2026-10-13T00:00:00Z')`,
		e.userIDOf(identifier), b); err != nil {
		e.t.Fatal(err)
	}
}

type eventRow struct {
	kind, from, to, reason string
	actorNull              bool
}

// eventsOf reads a booking's events through the database, in a stable order.
func (e *env) eventsOf(bookingID string) []eventRow {
	e.t.Helper()
	rows, err := e.pool.Query(e.t.Context(),
		`SELECT kind::text, COALESCE(from_status::text,''), to_status::text, COALESCE(reason,''), actor_id IS NULL
		 FROM booking_events WHERE booking_id = $1 ORDER BY created_at, (from_status IS NOT NULL), id`, bookingID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []eventRow
	for rows.Next() {
		var r eventRow
		if err := rows.Scan(&r.kind, &r.from, &r.to, &r.reason, &r.actorNull); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func TestEligibleStudentOnAClassroomIsApprovedAtOnce(t *testing.T) {
	e := newEnv(t, withAutoApproval)
	room := e.insertRoom("AP-KLS", "KELAS", 40)
	student := e.login(mahasiswaA).accessToken

	res := e.postBooking(student, room, at(tue, "08:00"), at(tue, "10:00"), nil)
	mustStatus(t, res, http.StatusCreated)
	b := res.json()
	if b["status"] != "DISETUJUI" || b["approval_mode"] != "AUTO" || b["manual_reason"] != nil || b["decided_by"] != nil {
		t.Errorf("booking = status %v mode %v reason %v decided_by %v", b["status"], b["approval_mode"], b["manual_reason"], b["decided_by"])
	}
	if b["expires_at"] != nil {
		t.Errorf("expires_at = %v; an approved booking does not expire", b["expires_at"])
	}

	id := idOf(res)
	ev := e.eventsOf(id)
	if len(ev) != 2 || ev[0].to != "MENUNGGU" || ev[0].actorNull || ev[1].from != "MENUNGGU" || ev[1].to != "DISETUJUI" || !ev[1].actorNull || ev[1].reason != "AUTO_APPROVED" {
		t.Errorf("events = %+v; want MENUNGGU by the student then DISETUJUI by the system", ev)
	}
	if n := e.notificationCount(e.userIDOf(tuID), "BOOKING_CREATED"); n != 0 {
		t.Errorf("approvers got %d BOOKING_CREATED for a booking nobody has to decide", n)
	}
	// The slot is taken from the moment of creation.
	other := e.login(mahasiswaB).accessToken
	if res := e.postBooking(other, room, at(tue, "09:00"), at(tue, "11:00"), nil); res.Code != http.StatusConflict || res.errorCode() != "BOOKING_CONFLICT" {
		t.Errorf("overlapping request: %d %s; want 409 BOOKING_CONFLICT", res.Code, res.Body.String())
	}
}

func TestManualOutcomesCarryTheirReason(t *testing.T) {
	e := newEnv(t, withAutoApproval)
	kelas := e.insertRoom("AP-M-KLS", "KELAS", 40)
	lab := e.insertRoom("AP-M-LAB", "LAB", 30)
	ujian := e.insertRoom("AP-M-UJ", "RUANG_UJIAN", 30)
	student := e.login(mahasiswaA).accessToken
	studentB := e.login(mahasiswaB).accessToken
	org := e.orgToken()
	e.addViolation(mahasiswaB)
	letter := e.letterFor(org)

	cases := []struct {
		name, tok, room, reason string
		extra                   map[string]any
		approver                string
	}{
		{"student on a lab", student, lab, "ROOM_POLICY", nil, kalabID},
		{"student on an exam room", student, ujian, "ROOM_POLICY", nil, tuID},
		{"student with a point on a classroom", studentB, kelas, "VIOLATION_HISTORY", nil, tuID},
		{"organisation on a classroom", org, kelas, "ORGANIZATION", map[string]any{"letter_attachment_id": letter}, tuID},
	}
	for i, c := range cases {
		// each case on its own day so cases never collide
		day := []string{mon, tue, wed, "2026-10-22"}[i]
		res := e.postBooking(c.tok, c.room, at(day, "08:00"), at(day, "10:00"), c.extra)
		mustStatus(t, res, http.StatusCreated)
		b := res.json()
		if b["status"] != "MENUNGGU" || b["approval_mode"] != "MANUAL" || b["manual_reason"] != c.reason {
			t.Errorf("%s: status %v mode %v reason %v; want MENUNGGU MANUAL %s", c.name, b["status"], b["approval_mode"], b["manual_reason"], c.reason)
		}
		if b["expires_at"] == nil {
			t.Errorf("%s: a waiting request must have expires_at", c.name)
		}
		if n := e.notificationCount(e.userIDOf(c.approver), "BOOKING_CREATED"); n == 0 {
			t.Errorf("%s: approver %s was not notified", c.name, c.approver)
		}
	}
}

func TestExpiresAtIsTheEarlierOf24HoursAndTheStart(t *testing.T) {
	e := newEnv(t)
	lab := e.insertRoom("AP-EXP", "LAB", 30)
	student := e.login(mahasiswaA).accessToken

	res := e.postBooking(student, lab, at(tue, "08:00"), at(tue, "10:00"), nil)
	mustStatus(t, res, http.StatusCreated)
	got, _ := time.Parse(time.RFC3339, res.json()["expires_at"].(string))
	if want := startTime.Add(24 * time.Hour); !got.Equal(want) {
		t.Errorf("expires_at = %v; want %v (24h after creation)", got, want)
	}

	// A request for later today expires at its own start.
	e.clock.Set(time.Date(2026, 10, 19, 0, 30, 0, 0, time.UTC)) // 07:30 WIB Monday
	student = e.login(mahasiswaA).accessToken
	res = e.postBooking(student, e.insertRoom("AP-EXP2", "LAB", 30), at(mon, "09:00"), at(mon, "10:00"), nil)
	mustStatus(t, res, http.StatusCreated)
	got, _ = time.Parse(time.RFC3339, res.json()["expires_at"].(string))
	if want := time.Date(2026, 10, 19, 2, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("expires_at = %v; want the start %v", got, want)
	}
}

func TestOrganisationNeedsAValidOwnLetter(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("AP-LET", "KELAS", 40)
	org := e.orgToken()
	student := e.login(mahasiswaA).accessToken
	studentsLetter := e.letterFor(student)

	invalid := func(name string, res response) {
		t.Helper()
		if res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s; want 422", name, res.Code, res.Body.String())
			return
		}
		found := false
		for _, d := range res.json()["error"].(map[string]any)["details"].([]any) {
			if d.(map[string]any)["field"] == "letter_attachment_id" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no detail on letter_attachment_id: %s", name, res.Body.String())
		}
	}
	invalid("no letter", e.postBooking(org, room, at(tue, "08:00"), at(tue, "09:00"), nil))
	invalid("someone else's letter", e.postBooking(org, room, at(tue, "08:00"), at(tue, "09:00"), map[string]any{"letter_attachment_id": studentsLetter}))
	invalid("unknown letter", e.postBooking(org, room, at(tue, "08:00"), at(tue, "09:00"), map[string]any{"letter_attachment_id": "00000000-0000-0000-0000-000000000000"}))

	// A photo cannot stand in for a letter; make one through the database.
	var photo string
	if err := e.pool.QueryRow(t.Context(),
		`INSERT INTO attachments (uploaded_by, kind, storage_path, content_type, size_bytes) VALUES ($1, 'PHOTO', 'attachments/x.jpg', 'image/jpeg', 1) RETURNING id`,
		e.userIDOf(orgID)).Scan(&photo); err != nil {
		t.Fatal(err)
	}
	invalid("a photo", e.postBooking(org, room, at(tue, "08:00"), at(tue, "09:00"), map[string]any{"letter_attachment_id": photo}))

	purged := e.letterFor(org)
	if _, err := e.pool.Exec(t.Context(), "UPDATE attachments SET purged_at = now() WHERE id = $1", purged); err != nil {
		t.Fatal(err)
	}
	invalid("a purged letter", e.postBooking(org, room, at(tue, "08:00"), at(tue, "09:00"), map[string]any{"letter_attachment_id": purged}))

	good := e.letterFor(org)
	res := e.postBooking(org, room, at(tue, "08:00"), at(tue, "09:00"), map[string]any{"letter_attachment_id": good, "letter_number": "012/HIMA/X/2026"})
	mustStatus(t, res, http.StatusCreated)
	letter, _ := res.json()["letter"].(map[string]any)
	if letter["attachment_id"] != good || letter["letter_number"] != "012/HIMA/X/2026" {
		t.Errorf("letter = %v", letter)
	}
}

func TestRoomTypePolicyDecidesWhoMayRequest(t *testing.T) {
	e := newEnv(t)
	auditorium := e.insertRoom("AP-AUD", "AUDITORIUM", 200)
	sidang := e.insertRoom("AP-SID", "RUANG_SIDANG", 10)
	student := e.login(mahasiswaA).accessToken
	org := e.orgToken()

	if res := e.postBooking(student, auditorium, at(tue, "08:00"), at(tue, "10:00"), map[string]any{"participants": 50}); res.Code != http.StatusForbidden || res.errorCode() != "ROOM_TYPE_NOT_ALLOWED" {
		t.Errorf("student on auditorium: %d %s; want 403 ROOM_TYPE_NOT_ALLOWED", res.Code, res.Body.String())
	}
	if res := e.postBooking(org, sidang, at(tue, "08:00"), at(tue, "10:00"), map[string]any{"letter_attachment_id": e.letterFor(org)}); res.Code != http.StatusForbidden || res.errorCode() != "ROOM_TYPE_NOT_ALLOWED" {
		t.Errorf("organisation on sidang room: %d %s; want 403 ROOM_TYPE_NOT_ALLOWED", res.Code, res.Body.String())
	}
	// The allowed pair works, and the auditorium also wants a letter and a head count.
	letter := e.letterFor(org)
	if res := e.postBooking(org, auditorium, at(tue, "08:00"), at(tue, "10:00"), map[string]any{"letter_attachment_id": letter}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("auditorium without participants: %d; want 422", res.Code)
	}
	mustStatus(t, e.postBooking(org, auditorium, at(tue, "08:00"), at(tue, "10:00"), map[string]any{"letter_attachment_id": letter, "participants": 150}), http.StatusCreated)
}

func TestParticipantsCannotExceedTheCapacity(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("AP-CAP", "KELAS", 20)
	student := e.login(mahasiswaA).accessToken
	if res := e.postBooking(student, room, at(tue, "08:00"), at(tue, "09:00"), map[string]any{"participants": 21}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("21 in a room of 20: %d; want 422", res.Code)
	}
	mustStatus(t, e.postBooking(student, room, at(tue, "08:00"), at(tue, "09:00"), map[string]any{"participants": 20}), http.StatusCreated)
}

func TestOutsideOperatingHoursIsItsOwnError(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("AP-HRS", "KELAS", 20)
	student := e.login(mahasiswaA).accessToken
	for name, w := range map[string][2]string{"before opening": {"05:00", "06:30"}, "after closing": {"21:00", "22:00"}} {
		res := e.postBooking(student, room, at(tue, w[0]), at(tue, w[1]), nil)
		if res.Code != http.StatusUnprocessableEntity || res.errorCode() != "OUTSIDE_OPERATING_HOURS" {
			t.Errorf("%s: %d %s; want 422 OUTSIDE_OPERATING_HOURS", name, res.Code, res.Body.String())
		}
	}
}

func TestQuotaCountsLiveRequestsAndIsAConflictNotAValidationError(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Business.MaxActiveBookings = 2 })
	student := e.login(mahasiswaA).accessToken
	room := e.insertRoom("AP-Q", "LAB", 30)

	first := e.postBooking(student, room, at(mon, "08:00"), at(mon, "09:00"), nil)
	mustStatus(t, first, http.StatusCreated)
	mustStatus(t, e.postBooking(student, room, at(tue, "08:00"), at(tue, "09:00"), nil), http.StatusCreated)
	res := e.postBooking(student, room, at(wed, "08:00"), at(wed, "09:00"), nil)
	if res.Code != http.StatusConflict || res.errorCode() != "QUOTA_EXCEEDED" {
		t.Fatalf("third request: %d %s; want 409 QUOTA_EXCEEDED", res.Code, res.Body.String())
	}
	// Cancelling one frees the quota.
	mustStatus(t, e.action(student, idOf(first), "cancel", nil), http.StatusOK)
	mustStatus(t, e.postBooking(student, room, at(wed, "08:00"), at(wed, "09:00"), nil), http.StatusCreated)
}

func TestBlockedRequesterCannotRequest(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("AP-BLK", "KELAS", 20)
	until := startTime.Add(48 * time.Hour)
	if _, err := e.pool.Exec(t.Context(), "UPDATE users SET booking_blocked_until = $2 WHERE id = $1", e.userIDOf(mahasiswaA), until); err != nil {
		t.Fatal(err)
	}
	student := e.login(mahasiswaA).accessToken
	res := e.postBooking(student, room, at(tue, "08:00"), at(tue, "09:00"), nil)
	if res.Code != http.StatusForbidden || res.errorCode() != "BOOKING_BLOCKED" {
		t.Fatalf("%d %s; want 403 BOOKING_BLOCKED", res.Code, res.Body.String())
	}
	details := res.json()["error"].(map[string]any)["details"].([]any)
	if len(details) != 1 || details[0].(map[string]any)["field"] != "blocked_until" {
		t.Errorf("details = %v", details)
	}
	// The block ends: requests work again.
	e.clock.Set(until.Add(time.Minute))
	student = e.login(mahasiswaA).accessToken
	mustStatus(t, e.postBooking(student, room, at("2026-10-22", "08:00"), at("2026-10-22", "09:00"), nil), http.StatusCreated)
}

func TestApproveAfterExpiryIsRefused(t *testing.T) {
	e := newEnv(t)
	lab := e.insertRoom("AP-EXPAPP", "LAB", 30)
	student := e.login(mahasiswaA).accessToken
	id := idOf(e.postBooking(student, lab, at(wed, "08:00"), at(wed, "09:00"), nil))
	e.clock.Add(25 * time.Hour) // the expiry job has not run, the request is stale anyway
	kalab := e.kalabToken()
	res := e.action(kalab, id, "approve", nil)
	if res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
		t.Errorf("approve after expiry: %d %s; want 409 INVALID_STATE", res.Code, res.Body.String())
	}
	if got := e.bookingStatus(id); got != "MENUNGGU" {
		t.Errorf("status = %s; a refused approval changes nothing", got)
	}
}

func TestLateCancelOfAnApprovedBookingCostsAPointAndMayBlock(t *testing.T) {
	e := newEnv(t, withAutoApproval, func(c *config.Config) { c.Business.BlockThresholdPoints = 1 })
	room := e.insertRoom("AP-LATE", "KELAS", 20)
	student := e.login(mahasiswaA).accessToken
	id := idOf(e.postBooking(student, room, at(tue, "08:00"), at(tue, "10:00"), nil))
	if e.bookingStatus(id) != "DISETUJUI" {
		t.Fatal("fixture should be auto-approved")
	}

	e.clock.Set(time.Date(2026, 10, 20, 0, 30, 0, 0, time.UTC)) // 07:30 WIB, 30 minutes before the start
	student = e.login(mahasiswaA).accessToken
	mustStatus(t, e.action(student, id, "cancel", nil), http.StatusOK)

	if n := e.countRows("SELECT count(*) FROM violations WHERE booking_id = $1 AND kind = 'BATAL_TERLAMBAT' AND points = 1", id); n != 1 {
		t.Errorf("late-cancel violations = %d; want 1", n)
	}
	if n := e.countRows("SELECT count(*) FROM users WHERE id = $1 AND booking_blocked_until > $2", e.userIDOf(mahasiswaA), e.clock.Now()); n != 1 {
		t.Error("the threshold was reached but the student is not blocked")
	}
	if n := e.notificationCount(e.userIDOf(mahasiswaA), "BOOKING_BLOCKED"); n != 1 {
		t.Errorf("block notifications = %d; want 1", n)
	}
	res := e.postBooking(student, room, at(wed, "08:00"), at(wed, "09:00"), nil)
	if res.Code != http.StatusForbidden || res.errorCode() != "BOOKING_BLOCKED" {
		t.Errorf("after the block: %d %s; want 403 BOOKING_BLOCKED", res.Code, res.Body.String())
	}
}

func TestEarlyCancelOfAnApprovedBookingIsFree(t *testing.T) {
	e := newEnv(t, withAutoApproval)
	room := e.insertRoom("AP-EARLY", "KELAS", 20)
	student := e.login(mahasiswaA).accessToken
	id := idOf(e.postBooking(student, room, at(tue, "08:00"), at(tue, "10:00"), nil))
	mustStatus(t, e.action(student, id, "cancel", nil), http.StatusOK)
	if n := e.countRows("SELECT count(*) FROM violations"); n != 0 {
		t.Errorf("%d violations for a cancellation days ahead", n)
	}
	// After the start nobody can cancel their own booking any more.
	id = idOf(e.postBooking(student, room, at(tue, "08:00"), at(tue, "10:00"), nil))
	e.clock.Set(time.Date(2026, 10, 20, 2, 0, 0, 0, time.UTC)) // 09:00 WIB, in progress
	student = e.login(mahasiswaA).accessToken
	if res := e.action(student, id, "cancel", nil); res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
		t.Errorf("cancel after start: %d %s; want 409 INVALID_STATE", res.Code, res.Body.String())
	}
}

func TestManagerCancelsOnlyApprovedBookingsOfTheirTypeWithAReason(t *testing.T) {
	e := newEnv(t, withAutoApproval)
	kelas := e.insertRoom("AP-MC", "KELAS", 20)
	student := e.login(mahasiswaA).accessToken
	id := idOf(e.postBooking(student, kelas, at(tue, "08:00"), at(tue, "10:00"), nil))

	tu, kalab := e.tuToken(), e.kalabToken()
	if res := e.action(kalab, id, "cancel", map[string]string{"reason": "Bukan wewenang saya"}); res.Code != http.StatusForbidden {
		t.Errorf("KALAB on a classroom: %d; want 403", res.Code)
	}
	if res := e.action(tu, id, "cancel", nil); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("TU without reason: %d; want 422", res.Code)
	}
	mustStatus(t, e.action(tu, id, "cancel", map[string]string{"reason": "Ruangan dipakai ujian"}), http.StatusOK)
	if n := e.notificationCount(e.userIDOf(mahasiswaA), "BOOKING_CANCELLED"); n != 1 {
		t.Errorf("owner notifications = %d; want 1", n)
	}
	if n := e.countRows("SELECT count(*) FROM violations"); n != 0 {
		t.Error("a manager's cancellation must not cost the student a point")
	}
}

// TestOneWinnerAmongParallelRequests fires many requesters at one slot at once (the room row lock decides)
// for an automatically approved room and for a manual one.
func TestOneWinnerAmongParallelRequests(t *testing.T) {
	for name, typ := range map[string]string{"auto-approved classroom": "KELAS", "manual lab": "LAB"} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, withAutoApproval)
			room := e.insertRoom("AP-RACE", typ, 30)
			const n = 20
			tokens := make([]string, n)
			for i := range tokens {
				id := fmt.Sprintf("99000%02d", i)
				e.insertUser(id, "Penguji "+id, "MAHASISWA")
				tokens[i] = e.login(id).accessToken
			}

			var wins, conflicts, other atomic.Int32
			var wg sync.WaitGroup
			start := make(chan struct{})
			for _, tok := range tokens {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					res := e.doUnchecked("POST", "/api/v1/bookings", bookingBody(room, at(tue, "08:00"), at(tue, "10:00"), nil), bearer(tok))
					switch {
					case res.Code == http.StatusCreated:
						wins.Add(1)
					case res.Code == http.StatusConflict && res.errorCode() == "BOOKING_CONFLICT":
						conflicts.Add(1)
					default:
						other.Add(1)
					}
				}()
			}
			close(start)
			wg.Wait()

			if wins.Load() != 1 || conflicts.Load() != n-1 || other.Load() != 0 {
				t.Errorf("wins %d, conflicts %d, other %d; want 1, %d, 0", wins.Load(), conflicts.Load(), other.Load(), n-1)
			}
			if got := e.countRows("SELECT count(*) FROM bookings WHERE room_id = $1", room); got != 1 {
				t.Errorf("%d booking rows; want 1", got)
			}
		})
	}
}

func TestDetailsAcceptsOnlyAnEmptyObjectForNow(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("AP-DET", "KELAS", 20)
	student := e.login(mahasiswaA).accessToken
	if res := e.postBooking(student, room, at(tue, "08:00"), at(tue, "09:00"), map[string]any{"details": map[string]any{"dosen_pendamping": "Dr. X"}}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown details key: %d; want 422", res.Code)
	}
	res := e.postBooking(student, room, at(tue, "08:00"), at(tue, "09:00"), map[string]any{"details": map[string]any{}})
	mustStatus(t, res, http.StatusCreated)
	if d, ok := res.json()["details"].(map[string]any); !ok || len(d) != 0 {
		t.Errorf("details = %v; want {}", res.json()["details"])
	}
}

func TestRequesterSummaryIsForApproversOnly(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("AP-SUM", "KELAS", 20)
	e.addViolation(mahasiswaA)
	student := e.login(mahasiswaA).accessToken
	id := idOf(e.postBooking(student, room, at(tue, "08:00"), at(tue, "09:00"), nil))

	own := e.do("GET", "/api/v1/bookings/"+id, nil, bearer(student))
	mustStatus(t, own, http.StatusOK)
	if _, has := own.json()["requester_summary"]; has {
		t.Error("the requester gets their own summary; it is for approvers")
	}
	seen := e.do("GET", "/api/v1/bookings/"+id, nil, bearer(e.tuToken()))
	mustStatus(t, seen, http.StatusOK)
	sum, _ := seen.json()["requester_summary"].(map[string]any)
	if sum == nil || sum["active_points"] != float64(1) {
		t.Errorf("requester_summary = %v; want 1 active point", sum)
	}
}
