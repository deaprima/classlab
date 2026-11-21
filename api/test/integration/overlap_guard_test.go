package app_test

import (
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// isExclusionViolation reports whether the database refused a write with SQLSTATE 23P01.
func isExclusionViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23P01"
}

// A booking and a schedule live in different tables, so no exclusion constraint sees both. The triggers of
// migration 000009 are the database's own guard: even a write that skips the services cannot put a live
// booking on top of a lecture, or a lecture on top of a live booking.
func TestDatabaseRefusesBookingOverSchedule(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("GRD-1", "KELAS", 30)
	e.insertSchedule(room, "Kalkulus", at(tue, "08:00"), at(tue, "10:00"))
	insert := `INSERT INTO bookings (room_id, requester_id, start_at, end_at, purpose, status, approval_mode, manual_reason)
	           VALUES ($1, $2, $3, $4, 'Rapat', $5, 'MANUAL', 'ROOM_POLICY')`

	for _, status := range []string{"MENUNGGU", "DISETUJUI", "BERLANGSUNG"} {
		_, err := e.pool.Exec(t.Context(), insert, room, e.userIDOf(mahasiswaA), at(tue, "09:00"), at(tue, "11:00"), status)
		if !isExclusionViolation(err) {
			t.Errorf("%s booking over a schedule: err = %v; want 23P01", status, err)
		}
	}
	// A booking that no longer holds its slot may sit anywhere, and back-to-back is not an overlap.
	if _, err := e.pool.Exec(t.Context(), insert, room, e.userIDOf(mahasiswaA), at(tue, "09:00"), at(tue, "11:00"), "DIBATALKAN"); err != nil {
		t.Errorf("cancelled booking over a schedule: %v", err)
	}
	moved := e.insertBooking(room, mahasiswaA, "MENUNGGU", "Sesudahnya", at(tue, "10:00"), at(tue, "11:00"))

	// Moving a live booking into the lecture is refused too.
	_, err := e.pool.Exec(t.Context(), "UPDATE bookings SET start_at = $2 WHERE id = $1", moved, at(tue, "09:30"))
	if !isExclusionViolation(err) {
		t.Errorf("moving a booking into a schedule: err = %v; want 23P01", err)
	}
	// So is reviving a cancelled one whose window is taken.
	var cancelled string
	if err := e.pool.QueryRow(t.Context(), "SELECT id FROM bookings WHERE room_id = $1 AND status = 'DIBATALKAN'", room).Scan(&cancelled); err != nil {
		t.Fatal(err)
	}
	_, err = e.pool.Exec(t.Context(), "UPDATE bookings SET status = 'MENUNGGU' WHERE id = $1", cancelled)
	if !isExclusionViolation(err) {
		t.Errorf("reviving a booking over a schedule: err = %v; want 23P01", err)
	}
}

func TestDatabaseRefusesScheduleOverLiveBooking(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("GRD-2", "KELAS", 30)
	e.insertBooking(room, mahasiswaA, "DISETUJUI", "Seminar", at(tue, "08:00"), at(tue, "10:00"))
	e.insertBooking(room, mahasiswaB, "DITOLAK", "Ditolak", at(tue, "13:00"), at(tue, "15:00"))
	insert := `INSERT INTO room_schedules (room_id, title, start_at, end_at, created_by) VALUES ($1, $2, $3, $4, $5)`

	_, err := e.pool.Exec(t.Context(), insert, room, "Bentrok", at(tue, "09:00"), at(tue, "11:00"), e.userIDOf(tuID))
	if !isExclusionViolation(err) {
		t.Errorf("schedule over an approved booking: err = %v; want 23P01", err)
	}
	// A rejected booking holds nothing.
	e.insertSchedule(room, "Bebas", at(tue, "13:00"), at(tue, "15:00"))
	free := e.insertSchedule(room, "Pagi", at(tue, "06:00"), at(tue, "08:00"))
	_, err = e.pool.Exec(t.Context(), "UPDATE room_schedules SET end_at = $2 WHERE id = $1", free, at(tue, "08:30"))
	if !isExclusionViolation(err) {
		t.Errorf("stretching a schedule into a booking: err = %v; want 23P01", err)
	}
}

// Many students request a slot while an administrator puts a lecture on the very same slot, all at once.
// Whatever the interleaving, exactly one of them gets it, everyone else gets a 409, and the database never
// holds a booking and a schedule that overlap.
func TestBookingsAndScheduleRaceForOneSlot(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("GRD-3", "KELAS", 30)
	students := []string{mahasiswaA, mahasiswaB, "2021101003", "2021101004", "2021101005"}
	tokens := make([]string, len(students))
	for i, s := range students {
		tokens[i] = e.login(s).accessToken
	}
	admin := e.adminToken()

	const n = 20
	results := make([]response, n+1)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = e.postBooking(tokens[i%len(tokens)], room, at(tue, "08:00"), at(tue, "10:00"), nil)
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		results[n] = e.postSchedule(admin, room, "Kuliah Pengganti", at(tue, "09:00"), at(tue, "11:00"), nil)
	}()
	close(start)
	wg.Wait()

	won := 0
	for i, res := range results {
		switch res.Code {
		case http.StatusCreated:
			won++
		case http.StatusConflict:
		default:
			t.Errorf("request %d: unexpected response %d: %s", i, res.Code, res.Body.String())
		}
	}
	if won != 1 {
		t.Errorf("%d requests won the slot; want exactly 1", won)
	}
	overlaps := e.countRows(`SELECT count(*) FROM room_schedules s JOIN bookings b ON b.room_id = s.room_id
		WHERE s.room_id = $1 AND b.status IN ('MENUNGGU','DISETUJUI','BERLANGSUNG')
		  AND tstzrange(s.start_at, s.end_at, '[)') && tstzrange(b.start_at, b.end_at, '[)')`, room)
	if overlaps != 0 {
		t.Errorf("%d booking/schedule overlaps stored; want 0", overlaps)
	}
}
