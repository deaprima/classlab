package app_test

import (
	"net/http"
	"slices"
	"testing"
)

func (e *env) newRoomBody(code string) map[string]any {
	return map[string]any{
		"org_unit_id": e.orgUnitID(),
		"code":        code,
		"name":        "Ruang " + code,
		"type":        "KELAS",
		"capacity":    30,
		"facilities":  []string{"Proyektor", "AC"},
	}
}

func TestCreateRoom(t *testing.T) {
	e := newEnv(t)
	admin := bearer(e.adminToken())

	body := e.newRoomBody("  kls-301 ")
	body["facilities"] = []string{"Proyektor", " proyektor ", "AC"}
	res := e.do("POST", "/api/v1/rooms", body, admin)
	mustStatus(t, res, http.StatusCreated)

	room := res.json()
	if room["code"] != "KLS-301" {
		t.Errorf("code = %v; want it trimmed and upper-cased", room["code"])
	}
	if f, _ := room["facilities"].([]any); len(f) != 2 {
		t.Errorf("facilities = %v; want duplicates dropped case-insensitively", room["facilities"])
	}
	if room["is_active"] != true || room["capacity"] != float64(30) || room["type"] != "KELAS" {
		t.Errorf("unexpected room: %v", room)
	}

	// Omitted facilities come back as an empty array, not null.
	body = e.newRoomBody("KLS-302")
	delete(body, "facilities")
	res = e.do("POST", "/api/v1/rooms", body, admin)
	mustStatus(t, res, http.StatusCreated)
	if f, ok := res.json()["facilities"].([]any); !ok || len(f) != 0 {
		t.Errorf("facilities = %v; want []", res.json()["facilities"])
	}
}

func TestCreateRoomValidation(t *testing.T) {
	e := newEnv(t)
	admin := bearer(e.adminToken())
	e.do("POST", "/api/v1/rooms", e.newRoomBody("DUP-1"), admin)

	cases := []struct {
		name   string
		mutate func(map[string]any)
		field  string
	}{
		{"blank code", func(b map[string]any) { b["code"] = "  " }, "code"},
		{"code with spaces", func(b map[string]any) { b["code"] = "A B" }, "code"},
		{"code too long", func(b map[string]any) { b["code"] = "A23456789012345678901234567890123" }, "code"},
		{"blank name", func(b map[string]any) { b["name"] = "" }, "name"},
		{"bad type", func(b map[string]any) { b["type"] = "GUDANG" }, "type"},
		{"zero capacity", func(b map[string]any) { b["capacity"] = 0 }, "capacity"},
		{"negative capacity", func(b map[string]any) { b["capacity"] = -5 }, "capacity"},
		{"absurd capacity", func(b map[string]any) { b["capacity"] = 1000000 }, "capacity"},
		{"blank facility", func(b map[string]any) { b["facilities"] = []string{"ok", " "} }, "facilities"},
		{"too many facilities", func(b map[string]any) {
			var f []string
			for i := 0; i < 31; i++ {
				f = append(f, "F"+string(rune('A'+i%26))+string(rune('a'+i/26)))
			}
			b["facilities"] = f
		}, "facilities"},
		{"missing org unit", func(b map[string]any) { delete(b, "org_unit_id") }, "org_unit_id"},
		{"unknown org unit", func(b map[string]any) { b["org_unit_id"] = "00000000-0000-4000-8000-000000000001" }, "org_unit_id"},
		{"duplicate code", func(b map[string]any) { b["code"] = "dup-1" }, "code"},
	}
	for _, tc := range cases {
		body := e.newRoomBody("NEW-1")
		tc.mutate(body)
		res := e.do("POST", "/api/v1/rooms", body, admin)
		if res.Code != http.StatusUnprocessableEntity || !fieldsOf(res)[tc.field] {
			t.Errorf("%s: %d %s; want 422 naming %q", tc.name, res.Code, res.Body.String(), tc.field)
		}
	}

	if res := e.do("POST", "/api/v1/rooms", `{"code":`, admin); res.Code != http.StatusBadRequest {
		t.Errorf("malformed JSON: %d; want 400", res.Code)
	}
	if res := e.do("POST", "/api/v1/rooms", `{"code":"X","bogus":1}`, admin); res.Code != http.StatusBadRequest {
		t.Errorf("unknown field: %d; want 400", res.Code)
	}
}

func TestListRooms(t *testing.T) {
	e := newEnv(t) // seed: 6 rooms, 3 KELAS (40, 40, 60) and 3 LAB (30, 40, 35)
	tok := bearer(e.login(mahasiswaA).accessToken)
	get := func(query string) ([]any, map[string]any, int) {
		res := e.do("GET", "/api/v1/rooms"+query, nil, tok)
		if res.Code != http.StatusOK {
			return nil, nil, res.Code
		}
		meta, _ := res.json()["meta"].(map[string]any)
		return dataOf(res), meta, res.Code
	}
	codes := func(items []any) []string {
		var out []string
		for _, it := range items {
			out = append(out, it.(map[string]any)["code"].(string))
		}
		return out
	}

	items, meta, _ := get("")
	if len(items) != 6 || meta["total"] != float64(6) {
		t.Fatalf("default listing: %d items, meta %v", len(items), meta)
	}
	if want := []string{"KLS-101", "KLS-102", "KLS-201", "LAB-1", "LAB-2", "LAB-3"}; !slices.Equal(codes(items), want) {
		t.Errorf("order = %v; want by code %v", codes(items), want)
	}

	if items, _, _ = get("?type=LAB"); len(items) != 3 {
		t.Errorf("type=LAB: %d; want 3", len(items))
	}
	if items, _, _ = get("?capacity_min=40"); len(items) != 4 {
		t.Errorf("capacity_min=40: %d; want 4", len(items))
	}
	if items, _, _ = get("?type=LAB&capacity_min=40"); len(items) != 1 {
		t.Errorf("combined filter: %d; want 1 (LAB-2)", len(items))
	}
	if items, _, _ = get("?q=jaringan"); len(items) != 1 {
		t.Errorf("q matches name case-insensitively: %d; want 1", len(items))
	}
	if items, _, _ = get("?q=lab-"); len(items) != 3 {
		t.Errorf("q matches code: %d; want 3", len(items))
	}
	if items, _, _ = get("?q=%25"); len(items) != 0 {
		t.Errorf("q=%% matched %d rooms; wildcards must be literal", len(items))
	}
	if items, meta, _ = get("?per_page=4&page=2"); len(items) != 2 || meta["total"] != float64(6) {
		t.Errorf("page 2 of per_page=4: %d items, meta %v", len(items), meta)
	}

	e.do("PATCH", "/api/v1/rooms/"+e.roomIDByCode("LAB-3"), map[string]any{"is_active": false}, bearer(e.adminToken()))
	if items, _, _ = get("?is_active=false"); len(items) != 1 {
		t.Errorf("is_active=false: %d; want 1", len(items))
	}
	if items, _, _ = get(""); len(items) != 6 {
		t.Errorf("inactive rooms stay listed by default: %d; want 6", len(items))
	}

	for _, bad := range []string{"?type=X", "?capacity_min=-1", "?capacity_min=a", "?is_active=maybe", "?page=0", "?per_page=500"} {
		if _, _, code := get(bad); code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d; want 422", bad, code)
		}
	}
}

func (e *env) roomIDByCode(code string) string {
	e.t.Helper()
	var id string
	if err := e.pool.QueryRow(e.t.Context(), "SELECT id FROM rooms WHERE code = $1 AND deleted_at IS NULL", code).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func TestGetRoom(t *testing.T) {
	e := newEnv(t)
	tok := bearer(e.login(obID).accessToken)

	res := e.do("GET", "/api/v1/rooms/"+e.roomIDByCode("LAB-1"), nil, tok)
	mustStatus(t, res, http.StatusOK)
	if r := res.json(); r["code"] != "LAB-1" || r["capacity"] != float64(30) || r["type"] != "LAB" {
		t.Errorf("unexpected room: %v", r)
	}

	for name, path := range map[string]string{
		"unknown id":   "/api/v1/rooms/00000000-0000-4000-8000-000000000001",
		"malformed id": "/api/v1/rooms/not-a-uuid",
	} {
		if res := e.do("GET", path, nil, tok); res.Code != http.StatusNotFound || res.errorCode() != "NOT_FOUND" {
			t.Errorf("%s: %d %s; want 404 NOT_FOUND", name, res.Code, res.errorCode())
		}
	}
}

func TestUpdateRoom(t *testing.T) {
	e := newEnv(t)
	admin := bearer(e.adminToken())
	id := e.roomIDByCode("KLS-101")
	path := "/api/v1/rooms/" + id

	res := e.do("PATCH", path, map[string]any{"name": "  Ruang Baru ", "capacity": 55, "type": "LAB", "facilities": []string{"Papan Tulis"}}, admin)
	mustStatus(t, res, http.StatusOK)
	r := res.json()
	if r["name"] != "Ruang Baru" || r["capacity"] != float64(55) || r["type"] != "LAB" || r["code"] != "KLS-101" {
		t.Errorf("unexpected room: %v", r)
	}
	if f, _ := r["facilities"].([]any); len(f) != 1 {
		t.Errorf("facilities = %v", r["facilities"])
	}

	// Fields not sent stay as they were; an explicit empty list clears facilities.
	res = e.do("PATCH", path, map[string]any{"is_active": false}, admin)
	mustStatus(t, res, http.StatusOK)
	if res.json()["capacity"] != float64(55) || res.json()["is_active"] != false {
		t.Errorf("partial update changed other fields: %v", res.json())
	}
	res = e.do("PATCH", path, map[string]any{"facilities": []string{}}, admin)
	mustStatus(t, res, http.StatusOK)
	if f, ok := res.json()["facilities"].([]any); !ok || len(f) != 0 {
		t.Errorf("facilities = %v; want []", res.json()["facilities"])
	}
}

func TestUpdateRoomErrors(t *testing.T) {
	e := newEnv(t)
	admin := bearer(e.adminToken())
	path := "/api/v1/rooms/" + e.roomIDByCode("KLS-101")

	cases := []struct {
		name string
		path string
		body any
		want int
	}{
		{"code is immutable", path, map[string]any{"code": "NEW"}, http.StatusBadRequest},
		{"empty patch", path, map[string]any{}, http.StatusUnprocessableEntity},
		{"bad type", path, map[string]any{"type": "X"}, http.StatusUnprocessableEntity},
		{"zero capacity", path, map[string]any{"capacity": 0}, http.StatusUnprocessableEntity},
		{"blank name", path, map[string]any{"name": " "}, http.StatusUnprocessableEntity},
		{"unknown id", "/api/v1/rooms/00000000-0000-4000-8000-000000000001", map[string]any{"name": "x"}, http.StatusNotFound},
		{"malformed id", "/api/v1/rooms/zzz", map[string]any{"name": "x"}, http.StatusNotFound},
	}
	for _, tc := range cases {
		if res := e.do("PATCH", tc.path, tc.body, admin); res.Code != tc.want {
			t.Errorf("%s: %d; want %d (%s)", tc.name, res.Code, tc.want, res.Body.String())
		}
	}
}

func TestDeleteRoomIsSoftAndFreesTheCode(t *testing.T) {
	e := newEnv(t)
	admin := bearer(e.adminToken())

	res := e.do("POST", "/api/v1/rooms", e.newRoomBody("TMP-1"), admin)
	id := res.json()["id"].(string)

	mustStatus(t, e.do("DELETE", "/api/v1/rooms/"+id, nil, admin), http.StatusNoContent)

	// Gone from every read path...
	if r := e.do("GET", "/api/v1/rooms/"+id, nil, admin); r.Code != http.StatusNotFound {
		t.Errorf("get after delete: %d; want 404", r.Code)
	}
	if r := e.do("GET", "/api/v1/rooms?q=TMP-1", nil, admin); len(dataOf(r)) != 0 {
		t.Error("deleted room still listed")
	}
	if r := e.do("DELETE", "/api/v1/rooms/"+id, nil, admin); r.Code != http.StatusNotFound {
		t.Errorf("second delete: %d; want 404", r.Code)
	}
	if r := e.do("PATCH", "/api/v1/rooms/"+id, map[string]any{"name": "x"}, admin); r.Code != http.StatusNotFound {
		t.Errorf("patch after delete: %d; want 404", r.Code)
	}

	// ...but the row stays as history, flagged and inactive.
	if n := e.countRows("SELECT count(*) FROM rooms WHERE id = $1 AND deleted_at IS NOT NULL AND NOT is_active", id); n != 1 {
		t.Errorf("row was not soft-deleted (matching rows: %d)", n)
	}

	// The code is free again.
	mustStatus(t, e.do("POST", "/api/v1/rooms", e.newRoomBody("TMP-1"), admin), http.StatusCreated)
}

func TestDeleteRoomRefusedWithLiveBookings(t *testing.T) {
	e := newEnv(t)
	admin := bearer(e.adminToken())

	// BR-07: pending and approved bookings that have not ended block deletion.
	for _, status := range []string{"MENUNGGU", "DISETUJUI"} {
		room := e.insertRoom("LIVE-"+status, "KELAS", 20)
		e.insertBooking(room, mahasiswaA, status, "Kerja kelompok", at(tue, "08:00"), at(tue, "10:00"))

		res := e.do("DELETE", "/api/v1/rooms/"+room, nil, admin)
		if res.Code != http.StatusConflict || res.errorCode() != "ROOM_HAS_ACTIVE_BOOKINGS" {
			t.Errorf("%s booking: %d %s; want 409 ROOM_HAS_ACTIVE_BOOKINGS", status, res.Code, res.errorCode())
		}
		if n := e.countRows("SELECT count(*) FROM rooms WHERE id = $1 AND deleted_at IS NULL", room); n != 1 {
			t.Errorf("%s booking: room was deleted despite the 409", status)
		}
	}

	// Finished, rejected and cancelled bookings do not block it.
	room := e.insertRoom("DONE-1", "KELAS", 20)
	e.insertBooking(room, mahasiswaA, "DISETUJUI", "Sudah lewat", at("2026-10-01", "08:00"), at("2026-10-01", "10:00"))
	e.insertBooking(room, mahasiswaA, "DITOLAK", "Ditolak", at(tue, "08:00"), at(tue, "10:00"))
	e.insertBooking(room, mahasiswaB, "DIBATALKAN", "Batal", at(tue, "08:00"), at(tue, "10:00"))
	mustStatus(t, e.do("DELETE", "/api/v1/rooms/"+room, nil, admin), http.StatusNoContent)
}

func TestDeleteRoomKeepsItsSchedulesAsHistory(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("HIST-1", "KELAS", 20)
	e.insertSchedule(room, "Kuliah lama", at(tue, "08:00"), at(tue, "10:00"))

	mustStatus(t, e.do("DELETE", "/api/v1/rooms/"+room, nil, bearer(e.adminToken())), http.StatusNoContent)
	if n := e.countRows("SELECT count(*) FROM room_schedules WHERE room_id = $1", room); n != 1 {
		t.Errorf("schedules after soft delete: %d; want 1", n)
	}
}
