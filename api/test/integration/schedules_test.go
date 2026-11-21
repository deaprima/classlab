package integration_test

import (
	"net/http"
	"slices"
	"sync"
	"testing"
)

func TestCreateSchedule(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("SCH-1", "LAB", 30)

	res := e.postSchedule(e.adminToken(), room, "  Basis Data  ", at(tue, "08:00"), at(tue, "10:00"), nil)
	mustStatus(t, res, http.StatusCreated)

	items := dataOf(res)
	if len(items) != 1 {
		t.Fatalf("got %d schedules; want 1", len(items))
	}
	s := items[0].(map[string]any)
	if s["title"] != "Basis Data" || s["source"] != "MANUAL" || s["room_id"] != room {
		t.Errorf("unexpected schedule: %v", s)
	}
	if s["start_at"] != at(tue, "08:00") || s["end_at"] != at(tue, "10:00") {
		t.Errorf("times should be echoed in WIB: %v - %v", s["start_at"], s["end_at"])
	}
	if s["created_by"] != e.userIDOf(adminID) {
		t.Errorf("created_by = %v; want the caller", s["created_by"])
	}

	res = e.postSchedule(e.adminToken(), room, "Kuliah", at(tue, "13:00"), at(tue, "15:00"), map[string]any{"source": "AKADEMIK"})
	mustStatus(t, res, http.StatusCreated)
	if dataOf(res)[0].(map[string]any)["source"] != "AKADEMIK" {
		t.Error("explicit source was ignored")
	}
}

func TestCreateScheduleValidation(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("VAL-1", "KELAS", 30)
	tok := e.adminToken()

	cases := []struct {
		name  string
		room  string
		title string
		start string
		end   string
		extra map[string]any
		field string
	}{
		{"blank title", room, " ", at(tue, "08:00"), at(tue, "09:00"), nil, "title"},
		{"end before start", room, "x", at(tue, "10:00"), at(tue, "09:00"), nil, "end_at"},
		{"zero length", room, "x", at(tue, "10:00"), at(tue, "10:00"), nil, "end_at"},
		{"crosses midnight", room, "x", at(tue, "22:00"), at(wed, "01:00"), nil, "end_at"},
		{"bad source", room, "x", at(tue, "08:00"), at(tue, "09:00"), map[string]any{"source": "LAINNYA"}, "source"},
		{"unknown room", "00000000-0000-4000-8000-000000000001", "x", at(tue, "08:00"), at(tue, "09:00"), nil, "room_id"},
		{"bad repeat freq", room, "x", at(tue, "08:00"), at(tue, "09:00"), map[string]any{"repeat": map[string]any{"freq": "DAILY", "until": "2026-11-01"}}, "repeat.freq"},
		{"repeat without until", room, "x", at(tue, "08:00"), at(tue, "09:00"), map[string]any{"repeat": map[string]any{"freq": "WEEKLY"}}, "repeat.until"},
		{"until not a date", room, "x", at(tue, "08:00"), at(tue, "09:00"), map[string]any{"repeat": map[string]any{"freq": "WEEKLY", "until": "besok"}}, "repeat.until"},
		{"until before start", room, "x", at(tue, "08:00"), at(tue, "09:00"), map[string]any{"repeat": map[string]any{"freq": "WEEKLY", "until": "2026-10-19"}}, "repeat.until"},
	}
	for _, tc := range cases {
		res := e.postSchedule(tok, tc.room, tc.title, tc.start, tc.end, tc.extra)
		if res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)[tc.field] {
			t.Errorf("%s: %d %s; want 422 naming %q", tc.name, res.Code, res.Body.String(), tc.field)
		}
	}

	if res := e.do("POST", "/api/v1/schedules", map[string]any{"room_id": room, "title": "x"}, bearer(tok)); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing times: %d; want 422", res.Code)
	}
	for name, body := range map[string]string{
		"timestamp without offset": `{"room_id":"` + room + `","title":"x","start_at":"2026-10-20T08:00:00","end_at":"2026-10-20T09:00:00"}`,
		"not json":                 `start=1`,
		"unknown field":            `{"room_id":"` + room + `","title":"x","bogus":1}`,
	} {
		if res := e.do("POST", "/api/v1/schedules", body, bearer(tok)); res.Code != http.StatusBadRequest {
			t.Errorf("%s: %d; want 400", name, res.Code)
		}
	}

	if n := e.countRows("SELECT count(*) FROM room_schedules WHERE room_id = $1", room); n != 0 {
		t.Errorf("%d schedule(s) were written despite validation errors", n)
	}
}

func TestScheduleOverlapUsesHalfOpenIntervals(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("OVL-1", "KELAS", 30)
	tok := e.adminToken()
	mustStatus(t, e.postSchedule(tok, room, "Pagi", at(tue, "08:00"), at(tue, "10:00"), nil), http.StatusCreated)

	// BR-02: 08:00-10:00 and 10:00-12:00 do not collide, nor do 06:00-08:00 and 08:00-10:00.
	mustStatus(t, e.postSchedule(tok, room, "Sesudah", at(tue, "10:00"), at(tue, "12:00"), nil), http.StatusCreated)
	mustStatus(t, e.postSchedule(tok, room, "Sebelum", at(tue, "06:00"), at(tue, "08:00"), nil), http.StatusCreated)

	for name, w := range map[string][2]string{
		"one minute into the end":   {"09:59", "11:00"},
		"one minute into the start": {"07:00", "08:01"},
		"fully contained":           {"08:30", "09:30"},
		"fully containing":          {"07:00", "12:30"},
		"identical":                 {"08:00", "10:00"},
	} {
		res := e.postSchedule(tok, room, "Bentrok", at(tue, w[0]), at(tue, w[1]), nil)
		if res.Code != http.StatusConflict || res.errorCode() != "BOOKING_CONFLICT" {
			t.Errorf("%s: %d %s; want 409 BOOKING_CONFLICT", name, res.Code, res.errorCode())
		}
	}

	// The same slot in another room, or on another day, is fine.
	other := e.insertRoom("OVL-2", "KELAS", 30)
	mustStatus(t, e.postSchedule(tok, other, "Ruang lain", at(tue, "08:00"), at(tue, "10:00"), nil), http.StatusCreated)
	mustStatus(t, e.postSchedule(tok, room, "Hari lain", at(wed, "08:00"), at(wed, "10:00"), nil), http.StatusCreated)
}

func TestScheduleConflictDetailsDescribeWhatCollided(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("DET-1", "KELAS", 30)
	e.insertSchedule(room, "Basis Data", at(tue, "08:00"), at(tue, "10:00"))
	e.insertBooking(room, mahasiswaA, "MENUNGGU", "Latihan", at(tue, "10:00"), at(tue, "11:00"))
	e.insertBooking(room, mahasiswaB, "DISETUJUI", "Rapat", at(tue, "11:00"), at(tue, "12:00"))

	res := e.postSchedule(e.adminToken(), room, "Bentrok", at(tue, "09:00"), at(tue, "12:00"), nil)
	mustStatus(t, res, http.StatusConflict)

	if got, want := conflictKinds(res), []string{"SCHEDULE", "BOOKING:MENUNGGU", "BOOKING:DISETUJUI"}; !slices.Equal(got, want) {
		t.Errorf("conflicts = %v; want %v", got, want)
	}
	d := res.json()["error"].(map[string]any)["details"].([]any)[0].(map[string]any)
	if d["start_at"] != at(tue, "08:00") || d["end_at"] != at(tue, "10:00") || d["message"] == "" {
		t.Errorf("first detail should describe the colliding schedule: %v", d)
	}
}

func TestScheduleIgnoresInactiveBookings(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("INA-1", "KELAS", 30)
	e.insertBooking(room, mahasiswaA, "DITOLAK", "Ditolak", at(tue, "08:00"), at(tue, "10:00"))
	e.insertBooking(room, mahasiswaB, "DIBATALKAN", "Batal", at(tue, "08:00"), at(tue, "10:00"))

	mustStatus(t, e.postSchedule(e.adminToken(), room, "Kuliah", at(tue, "08:00"), at(tue, "10:00"), nil), http.StatusCreated)
}

func TestRepeatingScheduleExpandsWeekly(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("REP-1", "LAB", 30)

	res := e.postSchedule(e.adminToken(), room, "Jarkom", at(tue, "13:00"), at(tue, "15:00"),
		map[string]any{"source": "AKADEMIK", "repeat": map[string]any{"freq": "WEEKLY", "until": "2026-11-10"}})
	mustStatus(t, res, http.StatusCreated)

	var starts []string
	for _, it := range dataOf(res) {
		s := it.(map[string]any)
		starts = append(starts, s["start_at"].(string))
		if s["source"] != "AKADEMIK" || s["title"] != "Jarkom" {
			t.Errorf("unexpected session: %v", s)
		}
	}
	// 20 Oct, 27 Oct, 3 Nov, 10 Nov: `until` is inclusive.
	want := []string{at("2026-10-20", "13:00"), at("2026-10-27", "13:00"), at("2026-11-03", "13:00"), at("2026-11-10", "13:00")}
	if !slices.Equal(starts, want) {
		t.Errorf("sessions = %v; want %v", starts, want)
	}
	if n := e.countRows("SELECT count(*) FROM room_schedules WHERE room_id = $1", room); n != 4 {
		t.Errorf("rows in database = %d; want 4", n)
	}

	// until on the very first day gives a single session; the day before the next week excludes it.
	single := e.postSchedule(e.adminToken(), room, "Satu", at(wed, "08:00"), at(wed, "09:00"),
		map[string]any{"repeat": map[string]any{"freq": "WEEKLY", "until": "2026-10-27"}})
	mustStatus(t, single, http.StatusCreated)
	if n := len(dataOf(single)); n != 1 {
		t.Errorf("until one day before the next week produced %d sessions; want 1", n)
	}
}

func TestRepeatingScheduleIsAllOrNothing(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("ATM-1", "LAB", 30)
	// Collides only with the third occurrence (3 Nov).
	e.insertSchedule(room, "Ujian", at("2026-11-03", "14:00"), at("2026-11-03", "16:00"))
	// Fourth occurrence (10 Nov) collides with an approved booking.
	e.insertBooking(room, mahasiswaA, "DISETUJUI", "Seminar", at("2026-11-10", "13:30"), at("2026-11-10", "14:30"))

	res := e.postSchedule(e.adminToken(), room, "Jarkom", at(tue, "13:00"), at(tue, "15:00"),
		map[string]any{"repeat": map[string]any{"freq": "WEEKLY", "until": "2026-11-10"}})
	mustStatus(t, res, http.StatusConflict)

	if got, want := conflictKinds(res), []string{"SCHEDULE", "BOOKING:DISETUJUI"}; !slices.Equal(got, want) {
		t.Errorf("conflicts = %v; want every collision listed: %v", got, want)
	}
	// Only the pre-existing row: the non-conflicting first two sessions must not have been kept.
	if n := e.countRows("SELECT count(*) FROM room_schedules WHERE room_id = $1", room); n != 1 {
		t.Errorf("rows in database = %d; want 1 (nothing from the failed request)", n)
	}
}

func TestRepeatingScheduleLimit(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("LIM-1", "KELAS", 30)
	tok := e.adminToken()

	// 26 weekly sessions from 20 Oct end on 13 Apr 2027.
	res := e.postSchedule(tok, room, "Semester", at(tue, "08:00"), at(tue, "09:00"),
		map[string]any{"repeat": map[string]any{"freq": "WEEKLY", "until": "2027-04-13"}})
	mustStatus(t, res, http.StatusCreated)
	if n := len(dataOf(res)); n != 26 {
		t.Fatalf("sessions = %d; want 26", n)
	}

	// One week further would be a 27th session.
	other := e.insertRoom("LIM-2", "KELAS", 30)
	res = e.postSchedule(tok, other, "Terlalu panjang", at(tue, "08:00"), at(tue, "09:00"),
		map[string]any{"repeat": map[string]any{"freq": "WEEKLY", "until": "2027-04-20"}})
	if res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)["repeat.until"] {
		t.Errorf("27 sessions: %d %s; want 422 naming repeat.until", res.Code, res.Body.String())
	}
	if n := e.countRows("SELECT count(*) FROM room_schedules WHERE room_id = $1", other); n != 0 {
		t.Errorf("%d rows written for a rejected request", n)
	}
}

func TestParallelSchedulesForOneSlotYieldExactlyOne(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("PAR-1", "KELAS", 30)
	tok := e.adminToken()

	const n = 12
	var wg sync.WaitGroup
	codes := make(chan int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- e.postSchedule(tok, room, "Rebutan", at(tue, "08:00"), at(tue, "10:00"), nil).Code
		}()
	}
	wg.Wait()
	close(codes)

	created, conflicts := 0, 0
	for c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflicts++
		default:
			t.Errorf("unexpected status %d", c)
		}
	}
	if created != 1 || conflicts != n-1 {
		t.Errorf("%d created, %d conflicts; want 1 and %d", created, conflicts, n-1)
	}
	if rows := e.countRows("SELECT count(*) FROM room_schedules WHERE room_id = $1", room); rows != 1 {
		t.Errorf("rows in database = %d; want 1", rows)
	}
}

func TestExclusionConstraintBacksUpTheApplicationCheck(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("EXC-1", "KELAS", 30)
	e.insertSchedule(room, "Pertama", at(tue, "08:00"), at(tue, "10:00"))

	_, err := e.pool.Exec(t.Context(),
		`INSERT INTO room_schedules (room_id, title, start_at, end_at, created_by) VALUES ($1, 'Kedua', $2, $3, $4)`,
		room, at(tue, "09:00"), at(tue, "11:00"), e.userIDOf(tuID))
	if err == nil {
		t.Fatal("database accepted two overlapping schedules for one room")
	}
	// Adjacent ranges stay legal at the database level too.
	e.insertSchedule(room, "Sambungan", at(tue, "10:00"), at(tue, "11:00"))
}

func TestUpdateSchedule(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("UPD-1", "KELAS", 30)
	tok := bearer(e.adminToken())
	id := e.insertSchedule(room, "Awal", at(tue, "08:00"), at(tue, "10:00"))
	e.insertSchedule(room, "Berikutnya", at(tue, "12:00"), at(tue, "14:00"))
	path := "/api/v1/schedules/" + id

	// Title only.
	res := e.do("PATCH", path, map[string]any{"title": " Judul Baru "}, tok)
	mustStatus(t, res, http.StatusOK)
	if s := res.json(); s["title"] != "Judul Baru" || s["start_at"] != at(tue, "08:00") {
		t.Errorf("unexpected schedule: %v", s)
	}

	// A schedule may move into time it already occupies: it must not conflict with itself.
	res = e.do("PATCH", path, map[string]any{"start_at": at(tue, "09:00"), "end_at": at(tue, "11:00")}, tok)
	mustStatus(t, res, http.StatusOK)

	// Only one bound sent: the other keeps its stored value.
	res = e.do("PATCH", path, map[string]any{"end_at": at(tue, "12:00")}, tok)
	mustStatus(t, res, http.StatusOK)
	if res.json()["start_at"] != at(tue, "09:00") || res.json()["end_at"] != at(tue, "12:00") {
		t.Errorf("unexpected range: %v", res.json())
	}

	// Touching the next schedule at 12:00 is fine; one minute more is not.
	res = e.do("PATCH", path, map[string]any{"end_at": at(tue, "12:01")}, tok)
	if res.Code != http.StatusConflict || res.errorCode() != "BOOKING_CONFLICT" {
		t.Errorf("overlap on update: %d %s; want 409 BOOKING_CONFLICT", res.Code, res.errorCode())
	}
	if got := e.countRows("SELECT count(*) FROM room_schedules WHERE id = $1 AND end_at = $2", id, at(tue, "12:00")); got != 1 {
		t.Error("a rejected update changed the row")
	}

	// Moving onto a live booking conflicts as well.
	e.insertBooking(room, mahasiswaA, "MENUNGGU", "Latihan", at(tue, "15:00"), at(tue, "16:00"))
	res = e.do("PATCH", path, map[string]any{"start_at": at(tue, "15:30"), "end_at": at(tue, "16:30")}, tok)
	mustStatus(t, res, http.StatusConflict)
	if got := conflictKinds(res); !slices.Equal(got, []string{"BOOKING:MENUNGGU"}) {
		t.Errorf("conflicts = %v", got)
	}
}

func TestUpdateScheduleErrors(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("UPE-1", "KELAS", 30)
	tok := bearer(e.adminToken())
	id := e.insertSchedule(room, "Awal", at(tue, "08:00"), at(tue, "10:00"))
	path := "/api/v1/schedules/" + id

	cases := []struct {
		name string
		path string
		body any
		want int
	}{
		{"empty patch", path, map[string]any{}, http.StatusUnprocessableEntity},
		{"blank title", path, map[string]any{"title": " "}, http.StatusUnprocessableEntity},
		{"end before start", path, map[string]any{"end_at": at(tue, "07:00")}, http.StatusUnprocessableEntity},
		{"crosses midnight", path, map[string]any{"end_at": at(wed, "01:00")}, http.StatusUnprocessableEntity},
		{"room is not editable", path, map[string]any{"room_id": room}, http.StatusBadRequest},
		{"unknown id", "/api/v1/schedules/00000000-0000-4000-8000-000000000001", map[string]any{"title": "x"}, http.StatusNotFound},
		{"malformed id", "/api/v1/schedules/zzz", map[string]any{"title": "x"}, http.StatusNotFound},
	}
	for _, tc := range cases {
		if res := e.do("PATCH", tc.path, tc.body, tok); res.Code != tc.want {
			t.Errorf("%s: %d; want %d (%s)", tc.name, res.Code, tc.want, res.Body.String())
		}
	}
}

func TestDeleteSchedule(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("DEL-1", "KELAS", 30)
	tok := bearer(e.adminToken())
	id := e.insertSchedule(room, "Hapus aku", at(tue, "08:00"), at(tue, "10:00"))

	mustStatus(t, e.do("DELETE", "/api/v1/schedules/"+id, nil, tok), http.StatusNoContent)
	if res := e.do("DELETE", "/api/v1/schedules/"+id, nil, tok); res.Code != http.StatusNotFound {
		t.Errorf("second delete: %d; want 404", res.Code)
	}
	if res := e.do("DELETE", "/api/v1/schedules/nope", nil, tok); res.Code != http.StatusNotFound {
		t.Errorf("malformed id: %d; want 404", res.Code)
	}
	// The slot is usable again.
	mustStatus(t, e.postSchedule(e.adminToken(), room, "Pengganti", at(tue, "08:00"), at(tue, "10:00"), nil), http.StatusCreated)
}

// Manual schedules follow the same authority as rooms (BR-10): the approvers of the room's type and
// ADMIN write them; everyone else is refused, and a refused request leaves the schedule untouched.
func TestScheduleWritesFollowRoomTypeAuthority(t *testing.T) {
	e := newEnv(t)
	tokens := map[string]string{
		"TU": e.tuToken(), "KALAB": e.kalabToken(), "ADMIN": e.adminToken(),
		"OB": e.obToken(), "MAHASISWA": e.login(mahasiswaA).accessToken, "ORGANISASI": e.orgToken(),
	}
	allowed := map[string][]string{
		"KELAS": {"TU", "ADMIN"}, "LAB": {"KALAB", "ADMIN"},
		"AUDITORIUM": {"TU", "ADMIN"}, "RUANG_UJIAN": {"TU", "ADMIN"}, "RUANG_SIDANG": {"TU", "ADMIN"},
	}

	n := 0
	for typ, who := range allowed {
		for role, tok := range tokens {
			n++
			may := slices.Contains(who, role)
			wantCreate, wantChange, wantDelete := http.StatusForbidden, http.StatusForbidden, http.StatusForbidden
			if may {
				wantCreate, wantChange, wantDelete = http.StatusCreated, http.StatusOK, http.StatusNoContent
			}

			room := e.insertRoom("SA-"+typ[:3]+string(rune('A'+n%26))+string(rune('A'+n/26)), typ, 20)
			if res := e.postSchedule(tok, room, "Kuliah", at(tue, "08:00"), at(tue, "09:00"), nil); res.Code != wantCreate {
				t.Errorf("%s creating a schedule in a %s room: %d; want %d (%s)", role, typ, res.Code, wantCreate, res.Body.String())
			}
			if may {
				if got := e.countRows("SELECT count(*) FROM room_schedules WHERE room_id = $1", room); got != 1 {
					t.Errorf("%s in %s: %d schedules written; want 1", role, typ, got)
				}
			} else if got := e.countRows("SELECT count(*) FROM room_schedules WHERE room_id = $1", room); got != 0 {
				t.Errorf("%s in %s: refused request still wrote %d schedule(s)", role, typ, got)
			}

			existing := e.insertSchedule(room, "Awal", at(tue, "13:00"), at(tue, "14:00"))
			if res := e.do("PATCH", "/api/v1/schedules/"+existing, map[string]any{"title": "Diubah"}, bearer(tok)); res.Code != wantChange {
				t.Errorf("%s updating a schedule in a %s room: %d; want %d (%s)", role, typ, res.Code, wantChange, res.Body.String())
			}
			if res := e.do("DELETE", "/api/v1/schedules/"+existing, nil, bearer(tok)); res.Code != wantDelete {
				t.Errorf("%s deleting a schedule in a %s room: %d; want %d (%s)", role, typ, res.Code, wantDelete, res.Body.String())
			}
		}
	}
}
