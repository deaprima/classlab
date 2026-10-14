package app_test

import (
	"net/http"
	"strconv"
	"testing"
)

// TestRBACMatrix pins the expected status for every endpoint x role. Later phases add rows here.
// Each case is evaluated with an unauthenticated caller and with one account per role.
func TestRBACMatrix(t *testing.T) {
	type expect struct {
		anon, mahasiswa, tuKalab, ob, kaprodi int
	}
	seq := 0

	endpoints := []struct {
		name   string
		method string
		path   func(e *env) string
		body   func(e *env) any
		want   expect
	}{
		{
			name: "GET /auth/me", method: "GET",
			path: func(*env) string { return "/api/v1/auth/me" },
			want: expect{401, 200, 200, 200, 200},
		},
		{
			name: "POST /auth/logout", method: "POST",
			path: func(*env) string { return "/api/v1/auth/logout" },
			want: expect{401, 200, 200, 200, 200},
		},
		{
			name: "GET /users", method: "GET",
			path: func(*env) string { return "/api/v1/users" },
			want: expect{401, 403, 403, 403, 200},
		},
		{
			name: "POST /users", method: "POST",
			path: func(*env) string { return "/api/v1/users" },
			body: func(e *env) any { seq++; return e.newUserBody("rbac" + strconv.Itoa(seq)) },
			want: expect{401, 403, 403, 403, 201},
		},
		{
			name: "PATCH /users/{id}", method: "PATCH",
			// Targets a seed student that is not one of the callers, so self-protection rules do not interfere.
			path: func(e *env) string { return "/api/v1/users/" + e.userIDOf("2021101005") },
			body: func(*env) any { return map[string]any{"name": "Diubah RBAC"} },
			want: expect{401, 403, 403, 403, 200},
		},
		{
			name: "GET /rooms", method: "GET",
			path: func(*env) string { return "/api/v1/rooms" },
			want: expect{401, 200, 200, 200, 200},
		},
		{
			name: "GET /rooms/{id}", method: "GET",
			path: func(e *env) string { return "/api/v1/rooms/" + e.roomIDByCode("LAB-1") },
			want: expect{401, 200, 200, 200, 200},
		},
		{
			name: "POST /rooms", method: "POST",
			path: func(*env) string { return "/api/v1/rooms" },
			body: func(e *env) any { seq++; return e.newRoomBody("RBAC-" + strconv.Itoa(seq)) },
			want: expect{401, 403, 201, 403, 201},
		},
		{
			name: "PATCH /rooms/{id}", method: "PATCH",
			path: func(e *env) string { return "/api/v1/rooms/" + e.roomIDByCode("LAB-3") },
			body: func(*env) any { return map[string]any{"capacity": 36} },
			want: expect{401, 403, 200, 403, 200},
		},
		{
			name: "DELETE /rooms/{id}", method: "DELETE",
			path: func(e *env) string {
				seq++
				return "/api/v1/rooms/" + e.insertRoom("DEL-"+strconv.Itoa(seq), "KELAS", 10)
			},
			want: expect{401, 403, 204, 403, 204},
		},
		{
			name: "GET /rooms/{id}/schedule", method: "GET",
			path: func(e *env) string {
				return timelinePath(e.roomIDByCode("LAB-1"), at(mon, "00:00"), at(tue, "00:00"))
			},
			want: expect{401, 200, 200, 200, 200},
		},
		{
			name: "GET /availability", method: "GET",
			path: func(*env) string { return availabilityPath(at(tue, "08:00"), at(tue, "10:00"), "") },
			want: expect{401, 200, 200, 200, 200},
		},
		{
			name: "POST /schedules", method: "POST",
			path: func(*env) string { return "/api/v1/schedules" },
			body: func(e *env) any {
				seq++
				room := e.insertRoom("SCH-"+strconv.Itoa(seq), "KELAS", 10)
				return map[string]any{"room_id": room, "title": "RBAC", "start_at": at(tue, "08:00"), "end_at": at(tue, "09:00")}
			},
			want: expect{401, 403, 201, 403, 201},
		},
		{
			name: "PATCH /schedules/{id}", method: "PATCH",
			path: func(e *env) string {
				seq++
				room := e.insertRoom("PSC-"+strconv.Itoa(seq), "KELAS", 10)
				return "/api/v1/schedules/" + e.insertSchedule(room, "Awal", at(tue, "08:00"), at(tue, "09:00"))
			},
			body: func(*env) any { return map[string]any{"title": "Diubah"} },
			want: expect{401, 403, 200, 403, 200},
		},
		{
			name: "DELETE /schedules/{id}", method: "DELETE",
			path: func(e *env) string {
				seq++
				room := e.insertRoom("DSC-"+strconv.Itoa(seq), "KELAS", 10)
				return "/api/v1/schedules/" + e.insertSchedule(room, "Hapus", at(tue, "08:00"), at(tue, "09:00"))
			},
			want: expect{401, 403, 204, 403, 204},
		},
		{
			name: "POST /bookings", method: "POST",
			path: func(*env) string { return "/api/v1/bookings" },
			body: func(e *env) any { return bookingBody(e.freshRoom(), at(tue, "08:00"), at(tue, "09:00"), nil) },
			want: expect{401, 201, 403, 403, 403},
		},
		{
			name: "GET /bookings", method: "GET",
			path: func(*env) string { return "/api/v1/bookings" },
			want: expect{401, 200, 200, 403, 200},
		},
		{
			name: "GET /bookings/{id}", method: "GET",
			path: func(e *env) string { return "/api/v1/bookings/" + e.pendingBookingOf(mahasiswaA) },
			want: expect{401, 200, 200, 403, 200},
		},
		{
			name: "GET /bookings/{id}/events", method: "GET",
			path: func(e *env) string { return "/api/v1/bookings/" + e.pendingBookingOf(mahasiswaA) + "/events" },
			want: expect{401, 200, 200, 403, 200},
		},
		{
			name: "POST /bookings/{id}/approve", method: "POST",
			path: func(e *env) string { return "/api/v1/bookings/" + e.pendingBookingOf(mahasiswaA) + "/approve" },
			want: expect{401, 403, 200, 403, 403},
		},
		{
			name: "POST /bookings/{id}/reject", method: "POST",
			path: func(e *env) string { return "/api/v1/bookings/" + e.pendingBookingOf(mahasiswaA) + "/reject" },
			body: func(*env) any { return map[string]any{"reason": "Dipakai ujian"} },
			want: expect{401, 403, 200, 403, 403},
		},
		{
			// The booking is pending and belongs to the student, so the owner may cancel it while TU/Kalab
			// passes the role gate but gets 409 INVALID_STATE: approvers cancel only approved bookings.
			name: "POST /bookings/{id}/cancel", method: "POST",
			path: func(e *env) string { return "/api/v1/bookings/" + e.pendingBookingOf(mahasiswaA) + "/cancel" },
			body: func(*env) any { return map[string]any{"reason": "Tidak jadi dipakai"} },
			want: expect{401, 200, 409, 403, 403},
		},
		{
			name: "GET /notifications", method: "GET",
			path: func(*env) string { return "/api/v1/notifications" },
			want: expect{401, 200, 200, 200, 200},
		},
		{
			name: "POST /notifications/read-all", method: "POST",
			path: func(*env) string { return "/api/v1/notifications/read-all" },
			want: expect{401, 200, 200, 200, 200},
		},
		// GET /notifications/stream never ends for a valid caller, so its 401 is checked in notifications_test.go.
		// POST /reports takes multipart, which this table cannot send; reports_test.go files one as every role.
		{
			name: "GET /reports", method: "GET",
			path: func(*env) string { return "/api/v1/reports" },
			want: expect{401, 200, 200, 200, 200},
		},
		{
			// The report belongs to the student, so the OB, who may only read their own, gets 404.
			name: "GET /reports/{id}", method: "GET",
			path: func(e *env) string { return "/api/v1/reports/" + e.reportAt("BARU", mahasiswaA) },
			want: expect{401, 200, 200, 404, 200},
		},
		{
			name: "GET /reports/{id}/photo", method: "GET",
			path: func(e *env) string {
				res := e.postReport(e.login(mahasiswaA).accessToken, e.freshRoom(), "Rusak", &photoPart{"a.png", "image/png", pngBytes(64)})
				return "/api/v1/reports/" + idOf(res) + "/photo"
			},
			want: expect{401, 200, 200, 404, 200},
		},
		{
			name: "PATCH /reports/{id}/status", method: "PATCH",
			path: func(e *env) string { return "/api/v1/reports/" + e.reportAt("BARU", mahasiswaA) + "/status" },
			body: func(*env) any { return map[string]any{"status": "DITERIMA"} },
			want: expect{401, 403, 200, 403, 403},
		},
	}

	roles := []struct {
		name       string
		identifier string
		pick       func(expect) int
	}{
		{"anonymous", "", func(x expect) int { return x.anon }},
		{"MAHASISWA", mahasiswaA, func(x expect) int { return x.mahasiswa }},
		{"TU_KALAB", tuKalabID, func(x expect) int { return x.tuKalab }},
		{"OB", obID, func(x expect) int { return x.ob }},
		{"KAPRODI", kaprodiID, func(x expect) int { return x.kaprodi }},
	}

	e := newEnv(t)
	for _, ep := range endpoints {
		for _, role := range roles {
			t.Run(ep.name+" as "+role.name, func(t *testing.T) {
				var opts []reqOpt
				if role.identifier != "" {
					opts = append(opts, bearer(e.login(role.identifier).accessToken))
				}
				var body any
				if ep.body != nil {
					body = ep.body(e)
				}

				res := e.do(ep.method, ep.path(e), body, opts...)
				if want := role.pick(ep.want); res.Code != want {
					t.Errorf("status = %d; want %d (body %s)", res.Code, want, res.Body.String())
				}
				if res.Code >= 400 && res.errorCode() == "" {
					t.Errorf("error response is not the standard envelope: %s", res.Body.String())
				}
			})
		}
	}
}

func (e *env) userIDOf(identifier string) string {
	e.t.Helper()
	var id string
	if err := e.pool.QueryRow(e.t.Context(), "SELECT id FROM users WHERE identifier = $1", identifier).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

func TestForbiddenResponseShape(t *testing.T) {
	e := newEnv(t)
	res := e.do("GET", "/api/v1/users", nil, bearer(e.login(mahasiswaA).accessToken))
	if res.Code != http.StatusForbidden || res.errorCode() != "FORBIDDEN" {
		t.Fatalf("got %d %s; want 403 FORBIDDEN", res.Code, res.errorCode())
	}
}
