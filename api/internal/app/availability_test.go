package app_test

import (
	"net/http"
	"net/url"
	"slices"
	"testing"
)

func availabilityPath(start, end string, extra string) string {
	return "/api/v1/availability?start_at=" + url.QueryEscape(start) + "&end_at=" + url.QueryEscape(end) + extra
}

// statusByCode flattens an availability response into room code -> status.
func statusByCode(res response) map[string]string {
	out := map[string]string{}
	for _, it := range dataOf(res) {
		m := it.(map[string]any)
		out[m["room"].(map[string]any)["code"].(string)] = m["status"].(string)
	}
	return out
}

func conflictsOf(res response, code string) []map[string]any {
	for _, it := range dataOf(res) {
		m := it.(map[string]any)
		if m["room"].(map[string]any)["code"] == code {
			out := []map[string]any{}
			for _, c := range m["conflicts"].([]any) {
				out = append(out, c.(map[string]any))
			}
			return out
		}
	}
	return nil
}

func TestAvailabilityReportsAllThreeStatuses(t *testing.T) {
	e := newEnv(t)
	tok := bearer(e.login(mahasiswaA).accessToken)
	window := func() response {
		return e.do("GET", availabilityPath(at(tue, "09:00"), at(tue, "11:00"), ""), nil, tok)
	}

	e.insertRoom("AV-FREE", "KELAS", 30)
	used := e.insertRoom("AV-USED", "KELAS", 30)
	e.insertSchedule(used, "Basis Data", at(tue, "08:00"), at(tue, "10:00"))
	asked := e.insertRoom("AV-ASKED", "KELAS", 30)
	e.insertBooking(asked, mahasiswaB, "MENUNGGU", "Latihan", at(tue, "10:00"), at(tue, "12:00"))
	approved := e.insertRoom("AV-APPR", "KELAS", 30)
	e.insertBooking(approved, mahasiswaB, "DISETUJUI", "Rapat", at(tue, "09:00"), at(tue, "10:00"))
	both := e.insertRoom("AV-BOTH", "KELAS", 30)
	e.insertBooking(both, mahasiswaB, "MENUNGGU", "Menunggu", at(tue, "09:00"), at(tue, "10:00"))
	e.insertSchedule(both, "Kuliah", at(tue, "10:00"), at(tue, "11:00"))
	dead := e.insertRoom("AV-DEAD", "KELAS", 30)
	e.insertBooking(dead, mahasiswaA, "DITOLAK", "Ditolak", at(tue, "09:00"), at(tue, "11:00"))
	e.insertBooking(dead, mahasiswaB, "DIBATALKAN", "Batal", at(tue, "09:00"), at(tue, "11:00"))

	res := window()
	mustStatus(t, res, http.StatusOK)
	got := statusByCode(res)

	want := map[string]string{
		"AV-FREE":  "TERSEDIA",
		"AV-USED":  "DIGUNAKAN", // academic schedule
		"AV-ASKED": "DIAJUKAN",  // pending booking only
		"AV-APPR":  "DIGUNAKAN", // approved booking
		"AV-BOTH":  "DIGUNAKAN", // DIGUNAKAN outranks DIAJUKAN
		"AV-DEAD":  "TERSEDIA",  // rejected and cancelled bookings hold nothing
	}
	for code, status := range want {
		if got[code] != status {
			t.Errorf("%s = %q; want %q", code, got[code], status)
		}
	}
	// Seeded rooms are free on Tuesday 09:00-11:00 except where the timetable says otherwise.
	if got["KLS-102"] != "DIGUNAKAN" || got["LAB-2"] != "TERSEDIA" {
		t.Errorf("seed timetable not reflected: KLS-102=%s (Matematika Diskrit 09-11), LAB-2=%s", got["KLS-102"], got["LAB-2"])
	}

	// conflicts list what is in the way, with the booking status exposed and times in WIB.
	both2 := conflictsOf(res, "AV-BOTH")
	if len(both2) != 2 || both2[0]["kind"] != "BOOKING" || both2[0]["status"] != "MENUNGGU" || both2[1]["kind"] != "SCHEDULE" {
		t.Errorf("AV-BOTH conflicts (sorted by start) = %v", both2)
	}
	if _, has := both2[1]["status"]; has {
		t.Error("SCHEDULE conflicts must not carry a status")
	}
	if c := conflictsOf(res, "AV-USED"); len(c) != 1 || c[0]["title"] != "Basis Data" || c[0]["start_at"] != at(tue, "08:00") {
		t.Errorf("AV-USED conflicts = %v", c)
	}
	if c := conflictsOf(res, "AV-FREE"); c == nil || len(c) != 0 {
		t.Errorf("a free room must have an empty conflicts array, got %v", c)
	}
}

func TestAvailabilityBoundariesAreHalfOpen(t *testing.T) {
	e := newEnv(t)
	tok := bearer(e.login(mahasiswaA).accessToken)
	room := e.insertRoom("HB-1", "KELAS", 30)
	e.insertSchedule(room, "Kuliah", at(tue, "08:00"), at(tue, "10:00"))

	for name, w := range map[string]struct {
		start, end, want string
	}{
		"starts exactly when it ends":      {"10:00", "12:00", "TERSEDIA"},
		"ends exactly when it starts":      {"06:00", "08:00", "TERSEDIA"},
		"one minute into the end":          {"09:59", "12:00", "DIGUNAKAN"},
		"one minute into the start":        {"06:00", "08:01", "DIGUNAKAN"},
		"inside":                           {"08:30", "09:00", "DIGUNAKAN"},
		"identical":                        {"08:00", "10:00", "DIGUNAKAN"},
		"wider on both sides":              {"07:00", "11:00", "DIGUNAKAN"},
		"pending booking boundary ignored": {"10:00", "11:00", "TERSEDIA"},
	} {
		res := e.do("GET", availabilityPath(at(tue, w.start), at(tue, w.end), "&type=KELAS&capacity_min=30"), nil, tok)
		mustStatus(t, res, http.StatusOK)
		if got := statusByCode(res)["HB-1"]; got != w.want {
			t.Errorf("%s (%s-%s): %q; want %q", name, w.start, w.end, got, w.want)
		}
	}

	// The same boundary for pending bookings.
	e.insertBooking(room, mahasiswaB, "MENUNGGU", "Pending", at(tue, "10:00"), at(tue, "11:00"))
	res := e.do("GET", availabilityPath(at(tue, "11:00"), at(tue, "12:00"), "&type=KELAS&capacity_min=30"), nil, tok)
	if got := statusByCode(res)["HB-1"]; got != "TERSEDIA" {
		t.Errorf("window starting where a pending booking ends: %q; want TERSEDIA", got)
	}
}

func TestAvailabilityFilters(t *testing.T) {
	e := newEnv(t)
	tok := bearer(e.login(mahasiswaA).accessToken)
	e.insertRoom("FL-INACTIVE", "KELAS", 30)
	e.insertRoom("FL-DELETED", "KELAS", 30)
	if _, err := e.pool.Exec(t.Context(), "UPDATE rooms SET is_active = false WHERE code = 'FL-INACTIVE'"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(t.Context(), "UPDATE rooms SET deleted_at = now(), is_active = false WHERE code = 'FL-DELETED'"); err != nil {
		t.Fatal(err)
	}

	list := func(extra string) []string {
		res := e.do("GET", availabilityPath(at(tue, "20:00"), at(tue, "21:00"), extra), nil, tok)
		mustStatus(t, res, http.StatusOK)
		var codes []string
		for code := range statusByCode(res) {
			codes = append(codes, code)
		}
		slices.Sort(codes)
		return codes
	}

	all := list("")
	if slices.Contains(all, "FL-INACTIVE") || slices.Contains(all, "FL-DELETED") {
		t.Errorf("inactive or deleted rooms are not bookable and must not be listed: %v", all)
	}
	if len(all) != 10 {
		t.Errorf("rooms = %v; want the 10 seeded ones", all)
	}
	if got := list("&type=LAB"); !slices.Equal(got, []string{"LAB-1", "LAB-2", "LAB-3"}) {
		t.Errorf("type=LAB: %v", got)
	}
	if got := list("&capacity_min=40"); !slices.Equal(got, []string{"AUD-1", "KLS-101", "KLS-102", "KLS-201", "LAB-2", "UJI-1", "UJI-2"}) {
		t.Errorf("capacity_min=40: %v", got)
	}
	if got := list("&type=LAB&capacity_min=40"); !slices.Equal(got, []string{"LAB-2"}) {
		t.Errorf("combined: %v", got)
	}
	if got := list("&capacity_min=500"); len(got) != 0 {
		t.Errorf("no room fits 500 people: %v", got)
	}
}

func TestAvailabilityRequestValidation(t *testing.T) {
	e := newEnv(t)
	tok := bearer(e.login(mahasiswaA).accessToken)

	cases := map[string]string{
		"missing start_at":   "/api/v1/availability?end_at=" + url.QueryEscape(at(tue, "10:00")),
		"missing end_at":     "/api/v1/availability?start_at=" + url.QueryEscape(at(tue, "10:00")),
		"no offset":          "/api/v1/availability?start_at=2026-10-20T08:00:00&end_at=2026-10-20T10:00:00",
		"not a timestamp":    "/api/v1/availability?start_at=besok&end_at=lusa",
		"end before start":   availabilityPath(at(tue, "10:00"), at(tue, "08:00"), ""),
		"empty range":        availabilityPath(at(tue, "10:00"), at(tue, "10:00"), ""),
		"range over 31 days": availabilityPath(at(tue, "10:00"), at("2026-12-01", "10:00"), ""),
		"unknown type":       availabilityPath(at(tue, "08:00"), at(tue, "10:00"), "&type=GUDANG"),
		"negative capacity":  availabilityPath(at(tue, "08:00"), at(tue, "10:00"), "&capacity_min=-1"),
		"capacity not int":   availabilityPath(at(tue, "08:00"), at(tue, "10:00"), "&capacity_min=banyak"),
	}
	for name, path := range cases {
		if res := e.do("GET", path, nil, tok); res.Code != http.StatusUnprocessableEntity || res.errorCode() != "VALIDATION_ERROR" {
			t.Errorf("%s: %d %s; want 422 VALIDATION_ERROR", name, res.Code, res.errorCode())
		}
	}
}

// The API examples write the offset as a bare "+07:00". In a query string "+" means space, so a
// client that does not escape it would otherwise get a puzzling validation error.
func TestAvailabilityAcceptsAnUnescapedPlusOffset(t *testing.T) {
	e := newEnv(t)
	tok := bearer(e.login(mahasiswaA).accessToken)

	raw := "/api/v1/availability?start_at=" + at(mon, "07:00") + "&end_at=" + at(mon, "09:30") + "&type=LAB"
	res := e.do("GET", raw, nil, tok)
	mustStatus(t, res, http.StatusOK)
	if got := statusByCode(res)["LAB-2"]; got != "DIGUNAKAN" {
		t.Errorf("LAB-2 = %q; want DIGUNAKAN (seeded Basis Data 07:00-09:30 on Monday)", got)
	}

	encoded := "/api/v1/availability?start_at=2026-10-19T07%3A00%3A00%2B07%3A00&end_at=2026-10-19T09%3A30%3A00%2B07%3A00"
	if got := statusByCode(e.do("GET", encoded, nil, tok))["LAB-2"]; got != "DIGUNAKAN" {
		t.Errorf("percent-encoded form: LAB-2 = %q", got)
	}

	// A different offset is the same instant: 00:00Z is 07:00 WIB.
	utc := "/api/v1/availability?start_at=2026-10-19T00:00:00Z&end_at=2026-10-19T02:30:00Z"
	if got := statusByCode(e.do("GET", utc, nil, tok))["LAB-2"]; got != "DIGUNAKAN" {
		t.Errorf("UTC form: LAB-2 = %q", got)
	}
}

func TestAvailabilityRequiresAuthentication(t *testing.T) {
	e := newEnv(t)
	if res := e.do("GET", availabilityPath(at(tue, "08:00"), at(tue, "10:00"), ""), nil); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: %d; want 401", res.Code)
	}
}

func timelinePath(room, from, to string) string {
	return "/api/v1/rooms/" + room + "/schedule?from=" + url.QueryEscape(from) + "&to=" + url.QueryEscape(to)
}

func TestRoomTimelineOverlaysApprovedBookingsOnSchedules(t *testing.T) {
	e := newEnv(t)
	tok := bearer(e.login(mahasiswaA).accessToken)
	room := e.insertRoom("TL-1", "LAB", 30)

	e.insertSchedule(room, "Basis Data", at(tue, "08:00"), at(tue, "10:00"))
	e.insertSchedule(room, "Jaringan", at(wed, "13:00"), at(wed, "15:00"))
	e.insertSchedule(room, "Di luar rentang", at("2026-11-20", "08:00"), at("2026-11-20", "10:00"))
	e.insertBooking(room, mahasiswaB, "DISETUJUI", "Rapat himpunan", at(tue, "10:00"), at(tue, "12:00"))
	e.insertBooking(room, mahasiswaB, "MENUNGGU", "Belum disetujui", at(tue, "13:00"), at(tue, "14:00"))
	e.insertBooking(room, mahasiswaA, "DITOLAK", "Ditolak", at(tue, "15:00"), at(tue, "16:00"))
	e.insertBooking(room, mahasiswaA, "DIBATALKAN", "Batal", at(tue, "16:00"), at(tue, "17:00"))

	res := e.do("GET", timelinePath(room, at(mon, "00:00"), at("2026-10-26", "00:00")), nil, tok)
	mustStatus(t, res, http.StatusOK)

	type item struct{ kind, title, start string }
	var got []item
	for _, it := range dataOf(res) {
		m := it.(map[string]any)
		got = append(got, item{m["kind"].(string), m["title"].(string), m["start_at"].(string)})
	}
	want := []item{
		{"SCHEDULE", "Basis Data", at(tue, "08:00")},
		{"BOOKING", "Rapat himpunan", at(tue, "10:00")},
		{"SCHEDULE", "Jaringan", at(wed, "13:00")},
	}
	if !slices.Equal(got, want) {
		t.Errorf("timeline = %v\nwant       %v", got, want)
	}

	// BR-05: an approved booking is an overlay; the academic schedule table is untouched.
	if n := e.countRows("SELECT count(*) FROM room_schedules WHERE room_id = $1", room); n != 3 {
		t.Errorf("room_schedules rows = %d; want the 3 inserted ones", n)
	}
}

func TestRoomTimelineBoundariesAndEmpty(t *testing.T) {
	e := newEnv(t)
	tok := bearer(e.login(mahasiswaA).accessToken)
	room := e.insertRoom("TL-2", "KELAS", 30)
	e.insertSchedule(room, "Kuliah", at(tue, "08:00"), at(tue, "10:00"))

	for name, w := range map[string]struct {
		from, to string
		n        int
	}{
		"window ends as it starts":     {"06:00", "08:00", 0},
		"window starts as it ends":     {"10:00", "12:00", 0},
		"window overlaps by a minute":  {"09:59", "12:00", 1},
		"window inside the schedule":   {"08:30", "09:00", 1},
		"window covering the schedule": {"00:00", "23:59", 1},
	} {
		res := e.do("GET", timelinePath(room, at(tue, w.from), at(tue, w.to)), nil, tok)
		mustStatus(t, res, http.StatusOK)
		if n := len(dataOf(res)); n != w.n {
			t.Errorf("%s: %d items; want %d", name, n, w.n)
		}
	}

	// An empty timeline is an empty array, not null.
	res := e.do("GET", timelinePath(room, at(wed, "00:00"), at(wed, "23:00")), nil, tok)
	if d, ok := res.json()["data"].([]any); !ok || len(d) != 0 {
		t.Errorf("data = %v; want []", res.json()["data"])
	}
}

func TestRoomTimelineErrors(t *testing.T) {
	e := newEnv(t)
	tok := bearer(e.login(mahasiswaA).accessToken)
	room := e.insertRoom("TL-3", "KELAS", 30)
	from, to := at(tue, "00:00"), at(wed, "00:00")

	if res := e.do("GET", timelinePath("00000000-0000-4000-8000-000000000001", from, to), nil, tok); res.Code != http.StatusNotFound {
		t.Errorf("unknown room: %d; want 404", res.Code)
	}
	if res := e.do("GET", timelinePath("not-a-uuid", from, to), nil, tok); res.Code != http.StatusNotFound {
		t.Errorf("malformed id: %d; want 404", res.Code)
	}
	for name, path := range map[string]string{
		"missing from":  "/api/v1/rooms/" + room + "/schedule?to=" + url.QueryEscape(to),
		"missing to":    "/api/v1/rooms/" + room + "/schedule?from=" + url.QueryEscape(from),
		"from after to": timelinePath(room, to, from),
		"over 31 days":  timelinePath(room, from, at("2026-12-31", "00:00")),
	} {
		if res := e.do("GET", path, nil, tok); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d; want 422", name, res.Code)
		}
	}

	// Deleted rooms have no timeline; inactive ones still do.
	if _, err := e.pool.Exec(t.Context(), "UPDATE rooms SET is_active = false WHERE id = $1", room); err != nil {
		t.Fatal(err)
	}
	mustStatus(t, e.do("GET", timelinePath(room, from, to), nil, tok), http.StatusOK)
	mustStatus(t, e.do("DELETE", "/api/v1/rooms/"+room, nil, bearer(e.adminToken())), http.StatusNoContent)
	if res := e.do("GET", timelinePath(room, from, to), nil, tok); res.Code != http.StatusNotFound {
		t.Errorf("deleted room: %d; want 404", res.Code)
	}
}

// Every room type is a valid availability filter, not just the original KELAS and LAB.
func TestAvailabilityFiltersByEveryRoomType(t *testing.T) {
	e := newEnv(t)
	tok := bearer(e.login(mahasiswaA).accessToken)
	for typ, want := range map[string][]string{
		"KELAS":        {"KLS-101", "KLS-102", "KLS-201"},
		"LAB":          {"LAB-1", "LAB-2", "LAB-3"},
		"AUDITORIUM":   {"AUD-1"},
		"RUANG_UJIAN":  {"UJI-1", "UJI-2"},
		"RUANG_SIDANG": {"SDG-1"},
	} {
		res := e.do("GET", availabilityPath(at(tue, "20:00"), at(tue, "21:00"), "&type="+typ), nil, tok)
		if res.Code != http.StatusOK {
			t.Errorf("type=%s: %d %s; want 200", typ, res.Code, res.Body.String())
			continue
		}
		var got []string
		for code := range statusByCode(res) {
			got = append(got, code)
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("type=%s: %v; want %v", typ, got, want)
		}
	}
}
