package app_test

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

const scheduleHeader = "room_code,title,day_of_week,start_time,end_time,semester_start,semester_end,skip_dates\n"

// schedCSV builds a schedule file from lines without the header.
func schedCSV(lines ...string) string { return scheduleHeader + strings.Join(lines, "\n") + "\n" }

// Four Mondays from 19 Oct 2026, all after the test clock (14 Oct 2026, a Wednesday).
func monday(room, title, start, end string) string {
	return room + "," + title + ",1," + start + "," + end + ",2026-10-19,2026-11-09,"
}

func (e *env) schedPreview(token, csv string) response {
	e.t.Helper()
	return e.multipartPost("/api/v1/imports/schedules?dry_run=true", token, nil, []byte(csv))
}

func (e *env) schedApply(token, csv, hash string) response {
	e.t.Helper()
	return e.multipartPost("/api/v1/imports/schedules", token, map[string]string{"plan_hash": hash}, []byte(csv))
}

func (e *env) schedPlanHash(token, csv string) string {
	e.t.Helper()
	res := e.schedPreview(token, csv)
	mustStatus(e.t, res, http.StatusOK)
	return res.json()["plan_hash"].(string)
}

func (e *env) academicCount(room string) int {
	return e.countRows(`SELECT count(*) FROM room_schedules WHERE room_id = $1 AND source = 'AKADEMIK'`, room)
}

func TestSchedulePreviewStoresNothingAndDescribesThePlan(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	a, b := e.insertRoom("IMP-A", "KELAS", 30), e.insertRoom("IMP-B", "LAB", 30)
	_ = a
	_ = b
	csv := schedCSV(monday("IMP-A", "Basis Data", "08:00", "10:00"), "IMP-B,Jaringan,3,13:00,15:30,2026-10-19,2026-11-09,2026-10-28")

	before := e.countRows("SELECT count(*) FROM room_schedules")
	res := e.schedPreview(tu, csv)
	mustStatus(t, res, http.StatusOK)
	body := res.json()
	// 4 Mondays, and 4 Wednesdays (21 Oct to 11 Nov is outside the range: 21, 28, Nov 4 = 3) minus the skipped 28 Oct.
	if body["rows"] != float64(2) || body["occurrences"] != float64(4+2) {
		t.Errorf("preview = %v, want 2 rows and 6 meetings", body)
	}
	if errs := body["errors"].([]any); len(errs) != 0 {
		t.Errorf("preview errors = %v", errs)
	}
	if cancel := body["bookings_to_cancel"].([]any); len(cancel) != 0 {
		t.Errorf("bookings_to_cancel = %v", cancel)
	}
	if e.countRows("SELECT count(*) FROM room_schedules") != before || e.countRows("SELECT count(*) FROM imports") != 0 {
		t.Error("the preview stored something")
	}
}

func TestScheduleImportCreatesAcademicMeetings(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	room := e.insertRoom("IMP-C", "KELAS", 30)
	csv := schedCSV("IMP-C,Statistika,1,08:00,10:00,2026-10-19,2026-11-09,2026-10-26")

	res := e.schedApply(tu, csv, e.schedPlanHash(tu, csv))
	mustStatus(t, res, http.StatusOK)
	if body := res.json(); body["rows"] != float64(1) || body["occurrences"] != float64(3) || body["bookings_cancelled"] != float64(0) {
		t.Fatalf("result = %v", body)
	}
	importID := res.json()["import_id"].(string)

	// Three meetings (19 Oct, 2 Nov, 9 Nov; 26 Oct is skipped), tied to the import and the importer.
	rows, err := e.pool.Query(t.Context(),
		`SELECT to_char(start_at AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM-DD HH24:MI'), to_char(end_at AT TIME ZONE 'Asia/Jakarta', 'HH24:MI'), source::text, import_id::text, created_by::text
		 FROM room_schedules WHERE room_id = $1 ORDER BY start_at`, room)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var start, end, source, imp, by string
		if err := rows.Scan(&start, &end, &source, &imp, &by); err != nil {
			t.Fatal(err)
		}
		if source != "AKADEMIK" || imp != importID || by != e.userIDOf(tuID) {
			t.Errorf("row %s: source %s import %s by %s", start, source, imp, by)
		}
		got = append(got, start+"-"+end)
	}
	if want := "2026-10-19 08:00-10:00 2026-11-02 08:00-10:00 2026-11-09 08:00-10:00"; strings.Join(got, " ") != want {
		t.Errorf("meetings = %v", got)
	}

	if n := e.acctCountAudit("SCHEDULES_IMPORTED"); n != 1 {
		t.Errorf("SCHEDULES_IMPORTED rows = %d, want 1", n)
	}
	list := e.do("GET", "/api/v1/imports?kind=SCHEDULES", nil, bearer(tu))
	mustStatus(t, list, http.StatusOK)
	if items := dataOf(list); len(items) != 1 || items[0].(map[string]any)["status"] != "COMMITTED" || items[0].(map[string]any)["semester"] != nil {
		t.Errorf("history = %v", items)
	}
	// The same file again is refused outright.
	if again := e.schedApply(tu, csv, e.schedPlanHash(tu, csv)); again.Code != http.StatusConflict || again.errorCode() != "IMPORT_ALREADY_APPLIED" {
		t.Errorf("second apply: %d %s, want 409 IMPORT_ALREADY_APPLIED", again.Code, again.errorCode())
	}
}

func TestScheduleImportDisplacesPendingAndApprovedBookings(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	room := e.insertRoom("IMP-D", "KELAS", 30)
	pending := e.insertBooking(room, mahasiswaA, "MENUNGGU", "Belajar bersama", at(mon, "08:30"), at(mon, "09:30"))
	approved := e.insertBooking(room, mahasiswaB, "DISETUJUI", "Rapat", at(mon, "09:30"), at(mon, "10:30"))
	adjacent := e.insertBooking(room, mahasiswaA, "DISETUJUI", "Sesudah kuliah", at(mon, "10:30"), at(mon, "12:00")) // starts after the class ends
	csv := schedCSV("IMP-D,Kalkulus,1,08:00,10:00,2026-10-19,2026-10-19,")

	preview := e.schedPreview(tu, csv)
	mustStatus(t, preview, http.StatusOK)
	cancel := preview.json()["bookings_to_cancel"].([]any)
	if len(cancel) != 2 {
		t.Fatalf("bookings_to_cancel = %v, want the pending and the approved one but not the adjacent one", cancel)
	}
	if c := cancel[0].(map[string]any); c["room_code"] != "IMP-D" || c["conflicts_with"] != "Kalkulus" || c["booking_id"] == nil {
		t.Errorf("first cancel entry = %v", c)
	}
	if e.bookingStatus(pending) != "MENUNGGU" {
		t.Error("the preview cancelled a booking")
	}

	res := e.schedApply(tu, csv, preview.json()["plan_hash"].(string))
	mustStatus(t, res, http.StatusOK)
	if res.json()["bookings_cancelled"] != float64(2) {
		t.Errorf("bookings_cancelled = %v", res.json()["bookings_cancelled"])
	}

	for id, owner := range map[string]string{pending: mahasiswaA, approved: mahasiswaB} {
		if got := e.bookingStatus(id); got != "DIBATALKAN" {
			t.Errorf("booking %s is %s, want DIBATALKAN", id, got)
		}
		var actor, reason string
		err := e.pool.QueryRow(t.Context(), `SELECT actor_id::text, reason FROM booking_events WHERE booking_id = $1 AND to_status = 'DIBATALKAN'`, id).Scan(&actor, &reason)
		if err != nil || actor != e.userIDOf(tuID) || reason != "Jadwal akademik: Kalkulus" {
			t.Errorf("event for %s: actor %s reason %q (%v)", id, actor, reason, err)
		}
		if n := e.notificationCount(e.userIDOf(owner), "BOOKING_CANCELLED"); n != 1 {
			t.Errorf("%s has %d BOOKING_CANCELLED notifications, want 1", owner, n)
		}
	}
	if got := e.bookingStatus(adjacent); got != "DISETUJUI" {
		t.Errorf("the back-to-back booking became %s: [start, end) must not collide", got)
	}
	// The displaced requesters did nothing wrong: no violation points.
	if n := e.countRows("SELECT count(*) FROM violations"); n != 0 {
		t.Errorf("violations = %d, want 0", n)
	}
	// The academic meeting is a plain overlay in the room afterwards.
	if e.academicCount(room) != 1 {
		t.Errorf("academic meetings = %d, want 1", e.academicCount(room))
	}
}

func TestScheduleImportTreatsOtherCollisionsAsLineErrors(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	running := e.insertRoom("IMP-E1", "KELAS", 30)
	manual := e.insertRoom("IMP-E2", "KELAS", 30)
	academic := e.insertRoom("IMP-E3", "KELAS", 30)
	clear := e.insertRoom("IMP-E4", "KELAS", 30)
	e.insertBooking(running, mahasiswaA, "BERLANGSUNG", "Sedang dipakai", at(mon, "08:30"), at(mon, "09:30"))
	e.insertSchedule(manual, "Rapat dosen", at(mon, "08:30"), at(mon, "09:30"))
	e.insertSchedule(academic, "Kuliah lama", at(mon, "08:30"), at(mon, "09:30"))
	e.pool.Exec(t.Context(), `UPDATE room_schedules SET source = 'AKADEMIK' WHERE room_id = $1`, academic)

	csv := schedCSV(
		"IMP-E1,A,1,08:00,10:00,2026-10-19,2026-10-19,", // line 2: a running booking
		"IMP-E2,B,1,08:00,10:00,2026-10-19,2026-10-19,", // line 3: a manual schedule
		"IMP-E3,C,1,08:00,10:00,2026-10-19,2026-10-19,", // line 4: another academic schedule
		"IMP-E4,D,1,08:00,10:00,2026-10-19,2026-10-19,", // line 5: fine
		"IMP-E4,E,1,09:00,11:00,2026-10-19,2026-10-19,", // line 6: overlaps line 5 inside the file
	)
	res := e.schedPreview(tu, csv)
	mustStatus(t, res, http.StatusOK)
	lines := map[int]string{}
	for _, item := range res.json()["errors"].([]any) {
		m := item.(map[string]any)
		lines[int(m["line"].(float64))] = m["message"].(string)
	}
	for _, line := range []int{2, 3, 4, 6} {
		if lines[line] == "" {
			t.Errorf("no error reported for line %d: %v", line, lines)
		}
	}
	if lines[5] != "" {
		t.Errorf("line 5 is fine but was flagged: %s", lines[5])
	}
	for line, want := range map[int]string{2: "berlangsung", 3: "manual", 4: "akademik", 6: "baris 5"} {
		if !strings.Contains(strings.ToLower(lines[line]), want) {
			t.Errorf("line %d message %q does not mention %q", line, lines[line], want)
		}
	}

	// Applying such a file stores nothing, not even the fine line.
	before := e.countRows("SELECT count(*) FROM room_schedules")
	apply := e.schedApply(tu, csv, res.json()["plan_hash"].(string))
	if apply.Code != http.StatusUnprocessableEntity || apply.errorCode() != "IMPORT_INVALID" {
		t.Fatalf("apply: %d %s, want 422 IMPORT_INVALID", apply.Code, apply.errorCode())
	}
	if e.countRows("SELECT count(*) FROM room_schedules") != before || e.academicCount(clear) != 0 || e.countRows("SELECT count(*) FROM imports") != 0 {
		t.Error("a rejected import left data behind")
	}
	if lines := importProblemLines(apply); !lines[2] || !lines[6] || lines[5] {
		t.Errorf("IMPORT_INVALID lines = %v", lines)
	}
}

func TestScheduleImportReportsRowProblems(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	e.insertRoom("IMP-F1", "KELAS", 30)
	off := e.insertRoom("IMP-F2", "KELAS", 30)
	e.pool.Exec(t.Context(), `UPDATE rooms SET is_active = false WHERE id = $1`, off)

	csv := schedCSV(
		"IMP-F1,Baik,1,08:00,10:00,2026-10-19,2026-11-09,",
		"NOPE-1,Ruang gaib,1,08:00,10:00,2026-10-19,2026-11-09,",
		"IMP-F2,Ruang mati,1,08:00,10:00,2026-10-19,2026-11-09,",
		"IMP-F1,Salah jam,1,10:00,08:00,2026-10-19,2026-11-09,",
		"IMP-F1,Salah hari,9,08:00,10:00,2026-10-19,2026-11-09,",
		"IMP-F1,Terlalu panjang,1,08:00,10:00,2026-01-05,2026-12-28,",
	)
	res := e.schedPreview(tu, csv)
	mustStatus(t, res, http.StatusOK)
	reported := map[int]string{}
	for _, item := range res.json()["errors"].([]any) {
		m := item.(map[string]any)
		reported[int(m["line"].(float64))] = m["column"].(string)
	}
	for line, col := range map[int]string{3: "room_code", 4: "room_code", 5: "end_time", 6: "day_of_week", 7: "semester_end"} {
		if reported[line] != col {
			t.Errorf("line %d: column %q, want %q (all: %v)", line, reported[line], col, reported)
		}
	}
	if _, bad := reported[2]; bad {
		t.Errorf("the valid line 2 was flagged: %v", reported)
	}
}

func TestScheduleImportPlanChangesAreCaughtBeforeAnythingIsWritten(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	room := e.insertRoom("IMP-G", "KELAS", 30)
	csv := schedCSV("IMP-G,Fisika,1,08:00,10:00,2026-10-19,2026-10-19,")
	hash := e.schedPlanHash(tu, csv)

	if res := e.schedApply(tu, csv, strings.Repeat("0", 64)); res.Code != http.StatusConflict || res.errorCode() != "IMPORT_PLAN_CHANGED" {
		t.Errorf("made-up hash: %d %s, want 409 IMPORT_PLAN_CHANGED", res.Code, res.errorCode())
	}
	if res := e.multipartPost("/api/v1/imports/schedules", tu, nil, []byte(csv)); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("no plan_hash: %d, want 422", res.Code)
	}

	// A request lands in the slot after the preview: the displaced-bookings part of the plan is now different.
	late := e.insertBooking(room, mahasiswaA, "MENUNGGU", "Datang belakangan", at(mon, "09:00"), at(mon, "10:00"))
	res := e.schedApply(tu, csv, hash)
	if res.Code != http.StatusConflict || res.errorCode() != "IMPORT_PLAN_CHANGED" {
		t.Fatalf("stale plan: %d %s, want 409 IMPORT_PLAN_CHANGED", res.Code, res.errorCode())
	}
	if e.bookingStatus(late) != "MENUNGGU" || e.academicCount(room) != 0 {
		t.Error("a stale apply changed something")
	}
	// A new preview sees the booking and the apply then displaces it.
	fresh := e.schedPreview(tu, csv)
	if n := len(fresh.json()["bookings_to_cancel"].([]any)); n != 1 {
		t.Fatalf("fresh preview lists %d bookings, want 1", n)
	}
	mustStatus(t, e.schedApply(tu, csv, fresh.json()["plan_hash"].(string)), http.StatusOK)
	if e.bookingStatus(late) != "DIBATALKAN" {
		t.Error("the late booking was not displaced by the fresh plan")
	}
}

func TestScheduleRollbackRemovesOnlyUpcomingMeetings(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	room := e.insertRoom("IMP-H", "KELAS", 30)
	displaced := e.insertBooking(room, mahasiswaA, "MENUNGGU", "Akan terdesak", at(mon, "09:00"), at(mon, "10:00"))
	csv := schedCSV(monday("IMP-H", "Algoritma", "08:00", "10:00"))

	applied := e.schedApply(tu, csv, e.schedPlanHash(tu, csv))
	mustStatus(t, applied, http.StatusOK)
	importID := applied.json()["import_id"].(string)
	if e.academicCount(room) != 4 {
		t.Fatalf("meetings after the import = %d, want 4", e.academicCount(room))
	}

	// A week later the first meeting (19 Oct) is behind us.
	e.clock.Add(7 * 24 * time.Hour)
	tu = e.tuToken() // the access token from before the jump has expired
	res := e.do("POST", "/api/v1/imports/"+importID+"/rollback", nil, bearer(tu))
	mustStatus(t, res, http.StatusOK)
	if res.json()["status"] != "ROLLED_BACK" || res.json()["deleted_occurrences"] != float64(3) {
		t.Errorf("rollback = %v, want 3 deleted", res.json())
	}
	if e.academicCount(room) != 1 {
		t.Errorf("meetings after the rollback = %d, want only the one that already took place", e.academicCount(room))
	}
	if e.bookingStatus(displaced) != "DIBATALKAN" {
		t.Error("rolling back must not bring a displaced booking back")
	}
	if n := e.acctCountAudit("IMPORT_ROLLED_BACK"); n != 1 {
		t.Errorf("IMPORT_ROLLED_BACK rows = %d, want 1", n)
	}
	var status string
	var rolledBackBy *string
	if err := e.pool.QueryRow(t.Context(), `SELECT status, rolled_back_by::text FROM imports WHERE id = $1`, importID).Scan(&status, &rolledBackBy); err != nil || status != "ROLLED_BACK" || rolledBackBy == nil {
		t.Errorf("import row: %s %v (%v)", status, rolledBackBy, err)
	}

	if again := e.do("POST", "/api/v1/imports/"+importID+"/rollback", nil, bearer(tu)); again.Code != http.StatusConflict || again.errorCode() != "INVALID_STATE" {
		t.Errorf("second rollback: %d %s, want 409 INVALID_STATE", again.Code, again.errorCode())
	}

	// Replacing a semester: after a rollback the same file can be imported again, and the meeting that
	// stayed behind does not get in its way because a re-import only creates what is still to come.
	again := e.schedApply(tu, csv, e.schedPlanHash(tu, csv))
	mustStatus(t, again, http.StatusOK)
	if again.json()["occurrences"] != float64(3) || e.academicCount(room) != 1+3 {
		t.Errorf("re-import = %v, meetings %d, want 3 new on top of the 1 that stayed", again.json(), e.academicCount(room))
	}
}

func TestScheduleRollbackIsLimitedToScheduleImports(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	// A student import cannot be rolled back (BR-23 applies to schedules).
	csv := "identifier,name\n23400001,Mahasiswa Baru\n"
	hash := e.importPreview(tu, csv, importSemester).json()["plan_hash"].(string)
	imp := e.importApply(tu, csv, importSemester, hash).json()["import_id"].(string)

	if res := e.do("POST", "/api/v1/imports/"+imp+"/rollback", nil, bearer(tu)); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("rolling back a user import: %d, want 422", res.Code)
	}
	if res := e.do("POST", "/api/v1/imports/00000000-0000-0000-0000-000000000000/rollback", nil, bearer(tu)); res.Code != http.StatusNotFound {
		t.Errorf("unknown import: %d, want 404", res.Code)
	}
	if res := e.do("POST", "/api/v1/imports/not-a-uuid/rollback", nil, bearer(tu)); res.Code != http.StatusNotFound {
		t.Errorf("malformed id: %d, want 404", res.Code)
	}
	for _, ident := range []string{mahasiswaA, obID, kalabID, orgID} {
		if res := e.do("POST", "/api/v1/imports/"+imp+"/rollback", nil, bearer(e.login(ident).accessToken)); res.Code != http.StatusForbidden {
			t.Errorf("%s rolling back: %d, want 403", ident, res.Code)
		}
	}
}

func TestScheduleImportAccess(t *testing.T) {
	e := newEnv(t)
	e.insertRoom("IMP-I", "KELAS", 30)
	csv := schedCSV(monday("IMP-I", "Akses", "08:00", "10:00"))

	for _, ident := range []string{mahasiswaA, obID, kalabID, orgID} {
		tok := e.login(ident).accessToken
		if res := e.schedPreview(tok, csv); res.Code != http.StatusForbidden {
			t.Errorf("%s previewing: %d, want 403", ident, res.Code)
		}
	}
	if res := e.schedPreview("", csv); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d, want 401", res.Code)
	}
	// TU and administrators import for every room type.
	admin := e.adminToken()
	mustStatus(t, e.schedApply(admin, csv, e.schedPlanHash(admin, csv)), http.StatusOK)
}

func TestImportingWhileBookingsArriveNeverLeavesAnOverlap(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()
	room := e.insertRoom("IMP-J", "KELAS", 30)
	csv := schedCSV("IMP-J,Praktikum,1,08:00,10:00,2026-10-19,2026-10-19,")
	hash := e.schedPlanHash(tu, csv)

	students := []string{mahasiswaA, mahasiswaB, "2021101003", "2021101004", "2021101005"}
	tokens := make([]string, len(students))
	for i, s := range students {
		tokens[i] = e.login(s).accessToken
	}

	// The import and requests for the same slot race. Whatever the order, the room ends up with either the
	// meeting or a booking in that slot, never both (brief 4.4).
	var wg sync.WaitGroup
	statuses := make([]int, len(tokens))
	var importStatus int
	wg.Add(1 + len(tokens))
	go func() {
		defer wg.Done()
		importStatus = e.schedApply(tu, csv, hash).Code
	}()
	for i := range tokens {
		go func() {
			defer wg.Done()
			statuses[i] = e.postBooking(tokens[i], room, at(mon, "08:00"), at(mon, "10:00"), nil).Code
		}()
	}
	wg.Wait()

	if importStatus != http.StatusOK && importStatus != http.StatusConflict {
		t.Errorf("import status %d, want 200 or 409", importStatus)
	}
	live := e.countRows(`SELECT count(*) FROM bookings WHERE room_id = $1 AND status IN ('MENUNGGU','DISETUJUI','BERLANGSUNG')`, room)
	meetings := e.academicCount(room)
	if live > 0 && meetings > 0 {
		t.Errorf("room holds %d live bookings and %d academic meetings in the same slot", live, meetings)
	}
	if overlaps := e.countRows(`SELECT count(*) FROM room_schedules s JOIN bookings b ON b.room_id = s.room_id
		WHERE s.room_id = $1 AND b.status IN ('MENUNGGU','DISETUJUI','BERLANGSUNG') AND tstzrange(s.start_at, s.end_at, '[)') && tstzrange(b.start_at, b.end_at, '[)')`, room); overlaps != 0 {
		t.Errorf("%d overlapping schedule/booking pairs", overlaps)
	}
	if importStatus == http.StatusOK && live != 0 {
		t.Errorf("the import succeeded yet %d live bookings remain in its slot", live)
	}
	created := 0
	for _, s := range statuses {
		if s == http.StatusCreated {
			created++
		}
	}
	if importStatus == http.StatusConflict && created != 1 {
		t.Errorf("import lost the race but %d bookings were created, want exactly 1", created)
	}
}

func TestSampleFilesPassThePreview(t *testing.T) {
	e := newEnv(t)
	tu := e.tuToken()

	schedule, err := os.ReadFile("../../testdata/samples/schedule.csv")
	if err != nil {
		t.Fatal(err)
	}
	res := e.schedPreview(tu, string(schedule))
	mustStatus(t, res, http.StatusOK)
	if errs := res.json()["errors"].([]any); len(errs) != 0 || res.json()["rows"] != float64(3) || res.json()["occurrences"].(float64) < 3 {
		t.Errorf("schedule.csv preview = %v", res.json())
	}

	users, err := os.ReadFile("../../testdata/samples/users.csv")
	if err != nil {
		t.Fatal(err)
	}
	res = e.importPreview(tu, string(users), importSemester)
	mustStatus(t, res, http.StatusOK)
	if errs := res.json()["errors"].([]any); len(errs) != 0 || res.json()["to_create"] != float64(3) {
		t.Errorf("users.csv preview = %v", res.json())
	}
}
