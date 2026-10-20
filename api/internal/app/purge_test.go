package app_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"classlab/api/internal/jobs"
)

func (e *env) purged(attachmentID string) bool {
	e.t.Helper()
	var at *time.Time
	if err := e.pool.QueryRow(e.t.Context(), "SELECT purged_at FROM attachments WHERE id = $1", attachmentID).Scan(&at); err != nil {
		e.t.Fatal(err)
	}
	return at != nil
}

func (e *env) fileOf(attachmentID string) string {
	e.t.Helper()
	var path string
	if err := e.pool.QueryRow(e.t.Context(), "SELECT storage_path FROM attachments WHERE id = $1", attachmentID).Scan(&path); err != nil {
		e.t.Fatal(err)
	}
	return filepath.Join(e.cfg.UploadDir, filepath.FromSlash(path))
}

func (e *env) purgeAt(r *jobs.Runner, now time.Time) int {
	e.t.Helper()
	n, err := r.PurgeFiles(e.t.Context(), now)
	if err != nil {
		e.t.Fatalf("PurgeFiles: %v", err)
	}
	return n
}

// A letter nothing refers to is deleted once it is older than 24 hours; one a booking uses stays (BR-13 c, BR-19).
func TestOrphanLettersArePurged(t *testing.T) {
	e := newEnv(t)
	r := e.jobRunner()
	t0 := e.clock.Now()

	student := e.login(mahasiswaA).accessToken
	orphan := e.letterFor(student)
	used := e.letterFor(student)
	mustStatus(t, e.postBooking(student, e.freshRoom(), at(tue, "08:00"), at(tue, "10:00"), map[string]any{"letter_attachment_id": used}), http.StatusCreated)
	file := e.fileOf(orphan)

	if n := e.purgeAt(r, t0.Add(24*time.Hour)); n != 0 {
		t.Fatalf("at exactly 24 hours: %d purged; want 0", n)
	}
	if n := e.purgeAt(r, t0.Add(24*time.Hour+time.Second)); n != 1 {
		t.Fatalf("a second later: %d purged; want only the orphan", n)
	}
	if !e.purged(orphan) || e.purged(used) {
		t.Errorf("purged: orphan %v, used %v; want true and false", e.purged(orphan), e.purged(used))
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("the orphan's file is still on disk (%v)", err)
	}
	e.setNow("2026-10-15", "12:00:00")
	owner := e.login(mahasiswaA).accessToken
	if res := e.getAttachment(owner, orphan); res.Code != http.StatusGone || res.errorCode() != "GONE" {
		t.Errorf("opening the purged letter: %d %s; want 410 GONE", res.Code, res.errorCode())
	}
	if res := e.getAttachment(owner, used); res.Code != http.StatusOK {
		t.Errorf("opening the letter in use: %d; want 200", res.Code)
	}
	if n := e.purgeAt(r, e.clock.Now()); n != 0 {
		t.Errorf("a second purge removed %d; want 0", n)
	}
}

// A letter a booking refers to is kept for LETTER_RETENTION_DAYS, and beyond that while the booking is still live.
func TestLetterRetention(t *testing.T) {
	e := newEnv(t)
	r := e.jobRunner()
	t0 := e.clock.Now()
	org := e.orgToken()
	letter := e.letterFor(org)
	id := idOf(e.postBooking(org, e.freshRoom(), at(tue, "08:00"), at(tue, "10:00"), map[string]any{"letter_attachment_id": letter}))
	retention := 365 * 24 * time.Hour

	if n := e.purgeAt(r, t0.Add(retention+time.Second)); n != 0 || e.purged(letter) {
		t.Fatalf("a live booking still needs the letter: %d purged", n)
	}
	if _, err := e.pool.Exec(t.Context(), "UPDATE bookings SET status = 'DIBATALKAN' WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	if n := e.purgeAt(r, t0.Add(retention)); n != 0 {
		t.Fatalf("at exactly the retention: %d purged; want 0", n)
	}
	if n := e.purgeAt(r, t0.Add(retention+time.Second)); n != 1 || !e.purged(letter) {
		t.Fatalf("past the retention with the booking over: %d purged; want the letter", n)
	}
	e.setNow("2027-10-15", "12:00:00")
	if res := e.getAttachment(e.login(orgID).accessToken, letter); res.Code != http.StatusGone {
		t.Errorf("opening the purged letter: %d; want 410", res.Code)
	}
	if res := e.getAttachment(e.tuToken(), letter); res.Code != http.StatusGone {
		t.Errorf("the approver opening it: %d; want 410", res.Code)
	}
}

// Check-out photos go after PHOTO_RETENTION_DAYS, except while a report that is not SELESAI still shows them (#12).
func TestPhotoRetention(t *testing.T) {
	e := newEnv(t)
	r := e.jobRunner()
	retention := 60 * 24 * time.Hour

	normal := e.inUse(e.freshRoom(), mahasiswaA)
	problem := e.inUse(e.freshRoom(), mahasiswaB)
	e.setNow(tue, "09:00:00")
	n := e.checkOut(e.login(mahasiswaA).accessToken, normal, map[string]string{"condition": "NORMAL"}, goodPhoto())
	mustStatus(t, n, http.StatusOK)
	p := e.checkOut(e.login(mahasiswaB).accessToken, problem, map[string]string{"condition": "ADA_MASALAH", "note": "Kursi patah"}, goodPhoto())
	mustStatus(t, p, http.StatusOK)
	normalPhoto := n.json()["booking"].(map[string]any)["checkout_attachment_id"].(string)
	problemPhoto := p.json()["booking"].(map[string]any)["checkout_attachment_id"].(string)
	report := p.json()["report_id"].(string)
	taken := e.clock.Now()

	if got := e.purgeAt(r, taken.Add(retention)); got != 0 {
		t.Fatalf("at exactly the retention: %d purged; want 0", got)
	}
	file := e.fileOf(normalPhoto)
	if got := e.purgeAt(r, taken.Add(retention+time.Second)); got != 1 {
		t.Fatalf("a second later: %d purged; want only the photo without a report", got)
	}
	if !e.purged(normalPhoto) || e.purged(problemPhoto) {
		t.Errorf("purged: normal %v, problem %v; want true and false", e.purged(normalPhoto), e.purged(problemPhoto))
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("the purged photo is still on disk (%v)", err)
	}

	// An open report keeps its photo however old it is.
	if got := e.purgeAt(r, taken.Add(400*24*time.Hour)); got != 0 || e.purged(problemPhoto) {
		t.Fatalf("a photo of an open report was purged (%d)", got)
	}
	later := taken.Add(401 * 24 * time.Hour)
	e.clock.Set(later)
	tu := e.tuToken()
	for _, step := range []string{"DITERIMA", "DIPROSES"} {
		mustStatus(t, e.patchReport(tu, report, map[string]string{"status": step}), http.StatusOK)
	}
	if got := e.purgeAt(r, later); got != 0 {
		t.Fatalf("a report still in progress lost its photo (%d)", got)
	}
	mustStatus(t, e.patchReport(tu, report, map[string]string{"status": "SELESAI"}), http.StatusOK)
	if got := e.purgeAt(r, later); got != 1 || !e.purged(problemPhoto) {
		t.Fatalf("after the report is finished: %d purged; want the photo", got)
	}

	// Opening a purged photo is 410 for those who may see it, and still 404 for those who may not.
	if res := e.getAttachment(e.obToken(), normalPhoto); res.Code != http.StatusGone {
		t.Errorf("OB opening a purged photo: %d; want 410", res.Code)
	}
	if res := e.getAttachment(e.login(mahasiswaA).accessToken, normalPhoto); res.Code != http.StatusGone {
		t.Errorf("owner opening a purged photo: %d; want 410", res.Code)
	}
	if res := e.getAttachment(e.login(mahasiswaB).accessToken, normalPhoto); res.Code != http.StatusNotFound {
		t.Errorf("another student: %d; want 404", res.Code)
	}
}

// The daily purge runs with the other jobs, at most once a day.
func TestPurgeRunsDailyWithTheJobs(t *testing.T) {
	e := newEnv(t)
	r := e.jobRunner()
	t0 := e.clock.Now()

	first := e.letterFor(e.login(mahasiswaA).accessToken)
	e.clock.Set(t0.Add(time.Hour))
	second := e.letterFor(e.login(mahasiswaA).accessToken)

	run := func(at time.Time) int {
		t.Helper()
		e.clock.Set(at)
		res, err := r.RunOnce(t.Context(), at)
		if err != nil {
			t.Fatal(err)
		}
		return res.PurgedFiles
	}
	if got := run(t0.Add(25 * time.Hour)); got != 1 || !e.purged(first) || e.purged(second) {
		t.Fatalf("first run: %d purged, first %v, second %v; want only the first letter", got, e.purged(first), e.purged(second))
	}
	if got := run(t0.Add(26*time.Hour + time.Second)); got != 0 || e.purged(second) {
		t.Errorf("an hour later: %d purged; the purge is not due again yet", got)
	}
	if got := run(t0.Add(49 * time.Hour)); got != 1 || !e.purged(second) {
		t.Errorf("a day later: %d purged; want the second letter", got)
	}
}
