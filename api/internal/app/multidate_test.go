package app_test

import (
	"net/http"
	"testing"

	"classlab/api/internal/config"
)

func occ(date, from, to string) map[string]string {
	return map[string]string{"start_at": at(date, from), "end_at": at(date, to)}
}

// postGroup files a multi-date request; the body carries occurrences instead of start_at and end_at.
func (e *env) postGroup(token, room string, dates []map[string]string, extra map[string]any, opts ...reqOpt) response {
	e.t.Helper()
	body := map[string]any{"room_id": room, "purpose": "Kuliah pengganti", "occurrences": dates}
	for k, v := range extra {
		body[k] = v
	}
	return e.do("POST", "/api/v1/bookings", body, append([]reqOpt{bearer(token)}, opts...)...)
}

func groupIDs(t *testing.T, res response) (string, []string) {
	t.Helper()
	body := res.json()
	var ids []string
	for _, b := range body["bookings"].([]any) {
		ids = append(ids, b.(map[string]any)["id"].(string))
	}
	return body["group_id"].(string), ids
}

func TestMultiDateRequestCreatesOneGroup(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("MD-LAB", "LAB", 30)
	student := e.login(mahasiswaA).accessToken

	res := e.postGroup(student, room, []map[string]string{occ(mon, "08:00", "10:00"), occ(tue, "08:00", "10:00"), occ(wed, "08:00", "10:00")}, nil)
	mustStatus(t, res, http.StatusCreated)
	groupID, ids := groupIDs(t, res)
	if len(ids) != 3 || res.json()["status"] != "MENUNGGU" || res.json()["approval_mode"] != "MANUAL" {
		t.Fatalf("group = %s", res.Body.String())
	}
	for i, b := range res.json()["bookings"].([]any) {
		if int(b.(map[string]any)["occurrence_no"].(float64)) != i+1 {
			t.Errorf("booking %d has occurrence_no %v", i, b.(map[string]any)["occurrence_no"])
		}
	}
	if n := e.countRows("SELECT count(*) FROM bookings WHERE group_id = $1", groupID); n != 3 {
		t.Errorf("rows in group = %d; want 3", n)
	}
	if n := e.notificationCount(e.userIDOf(kalabID), "BOOKING_CREATED"); n != 1 {
		t.Errorf("approver notifications = %d; one request is one notification", n)
	}
	for _, id := range ids {
		if n := e.eventCount(id); n != 1 {
			t.Errorf("booking %s has %d events; want 1", id, n)
		}
	}

	got := e.do("GET", "/api/v1/bookings/"+ids[1], nil, bearer(student))
	mustStatus(t, got, http.StatusOK)
	b := got.json()
	if b["group_id"] != groupID || b["occurrence_no"] != float64(2) || b["occurrence_count"] != float64(3) {
		t.Errorf("booking = group %v no %v count %v", b["group_id"], b["occurrence_no"], b["occurrence_count"])
	}
	list := e.do("GET", "/api/v1/bookings?group_id="+groupID, nil, bearer(student))
	mustStatus(t, list, http.StatusOK)
	if len(dataOf(list)) != 3 {
		t.Errorf("group_id filter returned %d; want 3", len(dataOf(list)))
	}
}

func TestMultiDateRequestIsAllOrNothing(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("MD-ATOM", "LAB", 30)
	e.insertSchedule(room, "Praktikum", at(tue, "09:00"), at(tue, "11:00"))
	student := e.login(mahasiswaA).accessToken

	res := e.postGroup(student, room, []map[string]string{occ(mon, "08:00", "10:00"), occ(tue, "08:00", "10:00"), occ(wed, "08:00", "10:00")}, nil)
	if res.Code != http.StatusConflict || res.errorCode() != "BOOKING_CONFLICT" {
		t.Fatalf("%d %s; want 409 BOOKING_CONFLICT", res.Code, res.Body.String())
	}
	details := res.json()["error"].(map[string]any)["details"].([]any)
	if len(details) != 1 || details[0].(map[string]any)["occurrence_no"] != float64(2) {
		t.Errorf("details = %v; want one conflict on date 2", details)
	}
	if n := e.countRows("SELECT count(*) FROM bookings"); n != 0 {
		t.Errorf("%d rows were written by a failed request", n)
	}
	if n := e.countRows("SELECT count(*) FROM booking_events"); n != 0 {
		t.Errorf("%d events were written by a failed request", n)
	}
	if n := e.notificationCount(e.userIDOf(kalabID), "BOOKING_CREATED"); n != 0 {
		t.Errorf("approvers were told about a request that does not exist")
	}
}

func TestMultiDateValidation(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("MD-VAL", "LAB", 30)
	student := e.login(mahasiswaA).accessToken

	overlap := e.postGroup(student, room, []map[string]string{occ(mon, "08:00", "10:00"), occ(mon, "09:00", "11:00")}, nil)
	if overlap.Code != http.StatusUnprocessableEntity {
		t.Fatalf("overlapping dates: %d; want 422", overlap.Code)
	}
	details := overlap.json()["error"].(map[string]any)["details"].([]any)
	if len(details) != 1 || details[0].(map[string]any)["occurrence_no"] != float64(2) {
		t.Errorf("details = %v; want the second date named", details)
	}

	var many []map[string]string
	for i := 0; i < 15; i++ { // MAX_OCCURRENCES is 14
		many = append(many, occ("2026-10-2"+string(rune('0'+i%10)), "08:00", "09:00"))
	}
	if res := e.postGroup(student, room, many, nil); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("15 dates: %d; want 422", res.Code)
	}
	if res := e.postGroup(student, room, nil, nil); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("no dates: %d; want 422", res.Code)
	}
	if res := e.postGroup(student, room, []map[string]string{occ(mon, "08:00", "10:00")}, map[string]any{"start_at": at(tue, "08:00"), "end_at": at(tue, "09:00")}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("both forms: %d; want 422", res.Code)
	}
	// A bad date among good ones names which one.
	bad := e.postGroup(student, room, []map[string]string{occ(mon, "08:00", "10:00"), occ(tue, "05:00", "06:00")}, nil)
	if bad.Code != http.StatusUnprocessableEntity || bad.errorCode() != "OUTSIDE_OPERATING_HOURS" {
		t.Errorf("one date outside hours: %d %s", bad.Code, bad.Body.String())
	}
	if n := e.countRows("SELECT count(*) FROM bookings"); n != 0 {
		t.Errorf("%d rows after only failed requests", n)
	}
}

func TestApprovingOneDateApprovesTheGroup(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("MD-APP", "LAB", 30)
	student := e.login(mahasiswaA).accessToken
	res := e.postGroup(student, room, []map[string]string{occ(mon, "08:00", "10:00"), occ(tue, "08:00", "10:00")}, nil)
	mustStatus(t, res, http.StatusCreated)
	_, ids := groupIDs(t, res)

	kalab := e.kalabToken()
	mustStatus(t, e.action(kalab, ids[1], "approve", nil), http.StatusOK)
	for _, id := range ids {
		if got := e.bookingStatus(id); got != "DISETUJUI" {
			t.Errorf("%s = %s; want the whole group approved", id, got)
		}
		if ev := e.eventsOf(id); len(ev) != 2 || ev[1].to != "DISETUJUI" || ev[1].actorNull {
			t.Errorf("events of %s = %+v", id, ev)
		}
	}
	if n := e.notificationCount(e.userIDOf(mahasiswaA), "BOOKING_APPROVED"); n != 1 {
		t.Errorf("student notifications = %d; one decision is one notification", n)
	}
}

func TestApprovingAGroupFailsAsAWholeWhenOneDateNowCollides(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("MD-COLL", "LAB", 30)
	student := e.login(mahasiswaA).accessToken
	res := e.postGroup(student, room, []map[string]string{occ(mon, "08:00", "10:00"), occ(tue, "08:00", "10:00")}, nil)
	mustStatus(t, res, http.StatusCreated)
	_, ids := groupIDs(t, res)
	// A schedule lands on the second date after the request was made.
	e.insertSchedule(room, "Dosen tamu", at(tue, "09:00"), at(tue, "10:00"))

	r := e.action(e.kalabToken(), ids[0], "approve", nil)
	if r.Code != http.StatusConflict || r.errorCode() != "BOOKING_CONFLICT" {
		t.Fatalf("%d %s; want 409 BOOKING_CONFLICT", r.Code, r.Body.String())
	}
	for _, id := range ids {
		if got := e.bookingStatus(id); got != "MENUNGGU" {
			t.Errorf("%s = %s; no date may be approved when one cannot be", id, got)
		}
	}
}

func TestRejectingOneDateRejectsTheGroup(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("MD-REJ", "LAB", 30)
	student := e.login(mahasiswaA).accessToken
	res := e.postGroup(student, room, []map[string]string{occ(mon, "08:00", "10:00"), occ(tue, "08:00", "10:00")}, nil)
	_, ids := groupIDs(t, res)

	mustStatus(t, e.action(e.kalabToken(), ids[0], "reject", map[string]string{"reason": "Lab dipakai penelitian"}), http.StatusOK)
	for _, id := range ids {
		if got := e.bookingStatus(id); got != "DITOLAK" {
			t.Errorf("%s = %s; want DITOLAK", id, got)
		}
	}
	if n := e.notificationCount(e.userIDOf(mahasiswaA), "BOOKING_REJECTED"); n != 1 {
		t.Errorf("notifications = %d; want 1", n)
	}
}

func TestAGroupCountsOnceAgainstTheQuota(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Business.MaxActiveBookings = 2 })
	room := e.insertRoom("MD-Q", "LAB", 30)
	student := e.login(mahasiswaA).accessToken

	mustStatus(t, e.postGroup(student, room, []map[string]string{occ(mon, "08:00", "09:00"), occ(tue, "08:00", "09:00"), occ(wed, "08:00", "09:00")}, nil), http.StatusCreated)
	mustStatus(t, e.postBooking(student, room, at(mon, "10:00"), at(mon, "11:00"), nil), http.StatusCreated)
	if res := e.postBooking(student, room, at(mon, "12:00"), at(mon, "13:00"), nil); res.Code != http.StatusConflict || res.errorCode() != "QUOTA_EXCEEDED" {
		t.Errorf("third request: %d %s; want 409 QUOTA_EXCEEDED (a group is one)", res.Code, res.Body.String())
	}
}

func TestCancelScopeOccurrenceAndGroup(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("MD-CAN", "LAB", 30)
	student := e.login(mahasiswaA).accessToken
	res := e.postGroup(student, room, []map[string]string{occ(mon, "08:00", "09:00"), occ(tue, "08:00", "09:00"), occ(wed, "08:00", "09:00")}, nil)
	_, ids := groupIDs(t, res)

	if r := e.action(student, ids[0], "cancel", map[string]string{"scope": "EVERYTHING"}); r.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown scope: %d; want 422", r.Code)
	}
	// Default scope is one date.
	mustStatus(t, e.action(student, ids[0], "cancel", nil), http.StatusOK)
	want := []string{"DIBATALKAN", "MENUNGGU", "MENUNGGU"}
	for i, id := range ids {
		if got := e.bookingStatus(id); got != want[i] {
			t.Errorf("date %d = %s; want %s", i+1, got, want[i])
		}
	}
	// GROUP cancels what is still cancellable and leaves the cancelled one alone.
	mustStatus(t, e.action(student, ids[1], "cancel", map[string]string{"scope": "GROUP"}), http.StatusOK)
	for i, id := range ids {
		if got := e.bookingStatus(id); got != "DIBATALKAN" {
			t.Errorf("date %d = %s; want DIBATALKAN", i+1, got)
		}
	}
	if n := e.eventCount(ids[0]); n != 2 {
		t.Errorf("the first date has %d events; it was cancelled once", n)
	}
	// Nothing is left to cancel.
	if r := e.action(student, ids[2], "cancel", map[string]string{"scope": "GROUP"}); r.Code != http.StatusConflict {
		t.Errorf("cancelling a finished group: %d; want 409", r.Code)
	}
}

func TestIdempotencyKeyReplaysAGroupAndRejectsADifferentRequest(t *testing.T) {
	e := newEnv(t)
	room := e.insertRoom("MD-IDEM", "LAB", 30)
	student := e.login(mahasiswaA).accessToken
	dates := []map[string]string{occ(mon, "08:00", "09:00"), occ(tue, "08:00", "09:00")}
	key := func(k string) reqOpt { return func(r *http.Request) { r.Header.Set("Idempotency-Key", k) } }

	first := e.postGroup(student, room, dates, nil, key("grp-1"))
	mustStatus(t, first, http.StatusCreated)
	second := e.postGroup(student, room, dates, nil, key("grp-1"))
	mustStatus(t, second, http.StatusCreated)
	if second.Header().Get("Idempotency-Replayed") != "true" {
		t.Error("the retry was not marked as a replay")
	}
	g1, ids1 := groupIDs(t, first)
	g2, ids2 := groupIDs(t, second)
	if g1 != g2 || ids1[0] != ids2[0] || ids1[1] != ids2[1] {
		t.Errorf("replay returned a different group: %s vs %s", g1, g2)
	}
	if n := e.countRows("SELECT count(*) FROM bookings"); n != 2 {
		t.Errorf("%d rows after a retry; want 2", n)
	}
	different := e.postGroup(student, room, dates[:1], nil, key("grp-1"))
	if different.Code != http.StatusUnprocessableEntity {
		t.Errorf("same key, other request: %d; want 422", different.Code)
	}
}
