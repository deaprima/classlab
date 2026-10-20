package app_test

import (
	"net/http"
	"slices"
	"strconv"
	"testing"

	"classlab/api/internal/config"
)

// The expected status for one caller. Keys are the role names below plus "anonymous"; a value of skip
// leaves that cell unchecked (used only where the contract has not settled).
type expect map[string]int

const skip = -1

var allRoles = []struct {
	name       string
	identifier string // "" for the anonymous caller
}{
	{"anonymous", ""},
	{"MAHASISWA", mahasiswaA},
	{"ORGANISASI", orgID},
	{"TU", tuID},
	{"KALAB", kalabID},
	{"OB", obID},
	{"ADMIN", adminID},
}

// only: the listed roles get ok, every other signed-in role 403, and an anonymous caller 401.
func only(ok int, roles ...string) expect {
	x := expect{}
	for _, r := range allRoles {
		switch {
		case r.name == "anonymous":
			x[r.name] = http.StatusUnauthorized
		case slices.Contains(roles, r.name):
			x[r.name] = ok
		default:
			x[r.name] = http.StatusForbidden
		}
	}
	return x
}

// everyone: any signed-in role gets code, an anonymous caller 401.
func everyone(code int) expect {
	x := expect{}
	for _, r := range allRoles {
		x[r.name] = code
	}
	x["anonymous"] = http.StatusUnauthorized
	return x
}

// with returns a copy where the given roles get code instead.
func (x expect) with(code int, roles ...string) expect {
	out := expect{}
	for k, v := range x {
		out[k] = v
	}
	for _, r := range roles {
		out[r] = code
	}
	return out
}

// roomTypeRules is the initial policy of brief BR-20, written out independently of the database so that a
// change in the seed data of room_type_policies shows up as a failing test.
var roomTypeRules = []struct {
	typ        string
	approvers  []string // TU / KALAB / ADMIN, as in approver_roles
	requesters []string
}{
	{"KELAS", []string{"TU"}, []string{"MAHASISWA", "ORGANISASI"}},
	{"LAB", []string{"KALAB"}, []string{"MAHASISWA", "ORGANISASI"}},
	{"AUDITORIUM", []string{"TU", "ADMIN"}, []string{"ORGANISASI"}},
	{"RUANG_UJIAN", []string{"TU", "ADMIN"}, []string{"MAHASISWA", "ORGANISASI"}},
	{"RUANG_SIDANG", []string{"TU", "ADMIN"}, []string{"MAHASISWA"}},
}

// roomWriters are the roles that may create, change and delete rooms and manual schedules of a type: its
// approvers, plus ADMIN for every type (BR-10).
func roomWriters(approvers []string) []string {
	if slices.Contains(approvers, "ADMIN") {
		return approvers
	}
	return append(slices.Clone(approvers), "ADMIN")
}

// freshRoomOfType inserts an empty room of the given type, so a case controls exactly what occupies it.
func (e *env) freshRoomOfType(typ string) string {
	roomSeq++
	return e.insertRoom("BK-"+strconv.Itoa(roomSeq), typ, 40)
}

// pendingBookingIn files a MENUNGGU booking of the student in a fresh room of the type, bypassing the
// requester rules so that every combination of caller and room type can be tried.
func (e *env) pendingBookingIn(typ, student string) string {
	e.t.Helper()
	return e.insertBooking(e.freshRoomOfType(typ), student, "MENUNGGU", "Fixture", at(tue, "08:00"), at(tue, "10:00"))
}

func (e *env) approvedBookingIn(typ, student string) string {
	e.t.Helper()
	return e.insertBooking(e.freshRoomOfType(typ), student, "DISETUJUI", "Fixture", at(tue, "08:00"), at(tue, "10:00"))
}

// reportIn has the student report damage in a fresh room of the type.
func (e *env) reportIn(typ string, photo *photoPart) string {
	e.t.Helper()
	res := e.postReport(e.login(mahasiswaA).accessToken, e.freshRoomOfType(typ), "Rusak", photo)
	mustStatus(e.t, res, http.StatusCreated)
	return idOf(res)
}

// TestRBACMatrix pins the expected status for every endpoint x role. Authority over rooms, schedules,
// bookings and reports depends on the room type (BR-10, BR-20), so those endpoints are tried once per type.
// Each case is evaluated with an unauthenticated caller and with one account per role.
func TestRBACMatrix(t *testing.T) {
	seq := 0

	type endpoint struct {
		name   string
		method string
		path   func(e *env) string
		body   func(e *env) any
		// bodyAs builds the body for the caller (identifier "" is anonymous) when it depends on who sends it.
		bodyAs func(e *env, identifier string) any
		// call replaces the JSON request for endpoints that take multipart data; token is "" when anonymous.
		call func(e *env, token string) response
		want expect
	}
	endpoints := []endpoint{
		{
			name: "GET /auth/me", method: "GET",
			path: func(*env) string { return "/api/v1/auth/me" },
			want: everyone(http.StatusOK),
		},
		{
			name: "POST /auth/logout", method: "POST",
			path: func(*env) string { return "/api/v1/auth/logout" },
			want: everyone(http.StatusOK),
		},
		// Users: ADMIN manages everyone, TU only students (see TestTUManagesOnlyStudentAccounts), creation is ADMIN only.
		{
			name: "GET /users", method: "GET",
			path: func(*env) string { return "/api/v1/users" },
			want: only(http.StatusOK, "TU", "ADMIN"),
		},
		{
			name: "POST /users", method: "POST",
			path: func(*env) string { return "/api/v1/users" },
			body: func(e *env) any { seq++; return e.newUserBody("rbac" + strconv.Itoa(seq)) },
			want: only(http.StatusCreated, "ADMIN"),
		},
		{
			name: "PATCH /users/{id}", method: "PATCH",
			// Targets a seed student that is not one of the callers, so self-protection rules do not interfere.
			path: func(e *env) string { return "/api/v1/users/" + e.userIDOf("2021101005") },
			body: func(*env) any { return map[string]any{"name": "Diubah RBAC"} },
			want: only(http.StatusOK, "TU", "ADMIN"),
		},
		{
			name: "GET /rooms", method: "GET",
			path: func(*env) string { return "/api/v1/rooms" },
			want: everyone(http.StatusOK),
		},
		{
			name: "GET /rooms/{id}", method: "GET",
			path: func(e *env) string { return "/api/v1/rooms/" + e.roomIDByCode("LAB-1") },
			want: everyone(http.StatusOK),
		},
		{
			name: "GET /rooms/{id}/schedule", method: "GET",
			path: func(e *env) string {
				return timelinePath(e.roomIDByCode("LAB-1"), at(mon, "00:00"), at(tue, "00:00"))
			},
			want: everyone(http.StatusOK),
		},
		{
			name: "GET /availability", method: "GET",
			path: func(*env) string { return availabilityPath(at(tue, "08:00"), at(tue, "10:00"), "") },
			want: everyone(http.StatusOK),
		},
		{
			name: "GET /notifications", method: "GET",
			path: func(*env) string { return "/api/v1/notifications" },
			want: everyone(http.StatusOK),
		},
		{
			name: "POST /notifications/read-all", method: "POST",
			path: func(*env) string { return "/api/v1/notifications/read-all" },
			want: everyone(http.StatusOK),
		},
		// GET /notifications/stream never ends for a valid caller, so its 401 is checked in notifications_test.go.
		// POST /reports takes multipart, which this table cannot send; reports_test.go files one as every role.
		{
			// Everyone may list; the list is scoped to what the role may see (see TestReportVisibilityFollowsRoomType).
			name: "GET /reports", method: "GET",
			path: func(*env) string { return "/api/v1/reports" },
			want: everyone(http.StatusOK),
		},
		{
			name: "GET /bookings", method: "GET",
			path: func(*env) string { return "/api/v1/bookings" },
			// OB does not see bookings at all; the other roles see the part they are entitled to.
			want: only(http.StatusOK, "MAHASISWA", "ORGANISASI", "TU", "KALAB", "ADMIN"),
		},
		{
			// Letters are uploaded by requesters only (BR-19).
			name: "POST /attachments", method: "POST",
			path: func(*env) string { return "/api/v1/attachments" },
			call: func(e *env, token string) response {
				return e.uploadLetter(token, "LETTER", pdfBytes())
			},
			want: only(http.StatusCreated, "MAHASISWA", "ORGANISASI"),
		},
		{
			name: "GET /attachments", method: "GET",
			path: func(*env) string { return "/api/v1/attachments?kind=LETTER" },
			want: only(http.StatusOK, "MAHASISWA", "ORGANISASI"),
		},
		{
			// A letter that is not attached to any booking is private to its uploader and ADMIN. The cells where TU or
			// KALAB open the letter of a booking of a type they approve are not in this matrix (they need a booking with
			// a letter per type); they are covered by the attachment tests.
			name: "GET /attachments/{id} (unattached letter)", method: "GET",
			path: func(e *env) string {
				return "/api/v1/attachments/" + e.letterFor(e.login(mahasiswaA).accessToken)
			},
			want: everyone(http.StatusNotFound).with(http.StatusOK, "MAHASISWA", "ADMIN"),
		},

		// Check-in is for requesters (C1: organisations too); nobody here has a booking, so those that pass the role
		// gate meet NO_ELIGIBLE_BOOKING. The booking-side cells are in checkin_test.go.
		{
			name: "POST /check-in", method: "POST",
			path: func(*env) string { return "/api/v1/check-in" },
			body: func(e *env) any {
				code, qr := e.roomQR(e.roomIDByCode("LAB-1"))
				return map[string]string{"room_code": code, "qr_token": qr}
			},
			want: only(http.StatusConflict, "MAHASISWA", "ORGANISASI"),
		},
		{
			name: "GET /checkouts", method: "GET",
			path: func(*env) string { return "/api/v1/checkouts" },
			want: only(http.StatusOK, "OB", "TU", "KALAB", "ADMIN"),
		},
		{
			name: "GET /users/{id}/violations", method: "GET",
			path: func(e *env) string { return "/api/v1/users/" + e.userIDOf(mahasiswaA) + "/violations" },
			want: only(http.StatusOK, "TU", "KALAB", "ADMIN"),
		},
		{
			name: "POST /violations/{id}/annul", method: "POST",
			path: func(e *env) string { return "/api/v1/violations/" + e.violationOf(mahasiswaA) + "/annul" },
			body: func(*env) any { return map[string]any{"reason": "Salah catat oleh sistem"} },
			want: only(http.StatusOK, "ADMIN"),
		},
	}

	for _, rt := range roomTypeRules {
		typ, approvers, requesters := rt.typ, rt.approvers, rt.requesters
		writers := roomWriters(approvers)

		endpoints = append(endpoints,
			endpoint{
				name: "POST /rooms [" + typ + "]", method: "POST",
				path: func(*env) string { return "/api/v1/rooms" },
				body: func(e *env) any {
					seq++
					b := e.newRoomBody("RBAC-" + strconv.Itoa(seq))
					b["type"] = typ
					return b
				},
				want: only(http.StatusCreated, writers...),
			},
			endpoint{
				name: "PATCH /rooms/{id} [" + typ + "]", method: "PATCH",
				path: func(e *env) string { return "/api/v1/rooms/" + e.freshRoomOfType(typ) },
				body: func(*env) any { return map[string]any{"capacity": 36} },
				want: only(http.StatusOK, writers...),
			},
			endpoint{
				name: "DELETE /rooms/{id} [" + typ + "]", method: "DELETE",
				path: func(e *env) string { return "/api/v1/rooms/" + e.freshRoomOfType(typ) },
				want: only(http.StatusNoContent, writers...),
			},
			endpoint{
				name: "POST /schedules [" + typ + "]", method: "POST",
				path: func(*env) string { return "/api/v1/schedules" },
				body: func(e *env) any {
					return map[string]any{"room_id": e.freshRoomOfType(typ), "title": "RBAC", "start_at": at(tue, "08:00"), "end_at": at(tue, "09:00")}
				},
				want: only(http.StatusCreated, writers...),
			},
			endpoint{
				name: "PATCH /schedules/{id} [" + typ + "]", method: "PATCH",
				path: func(e *env) string {
					return "/api/v1/schedules/" + e.insertSchedule(e.freshRoomOfType(typ), "Awal", at(tue, "08:00"), at(tue, "09:00"))
				},
				body: func(*env) any { return map[string]any{"title": "Diubah"} },
				want: only(http.StatusOK, writers...),
			},
			endpoint{
				name: "DELETE /schedules/{id} [" + typ + "]", method: "DELETE",
				path: func(e *env) string {
					return "/api/v1/schedules/" + e.insertSchedule(e.freshRoomOfType(typ), "Hapus", at(tue, "08:00"), at(tue, "09:00"))
				},
				want: only(http.StatusNoContent, writers...),
			},

			// Bookings. Only the requester roles of the type may file one; a requester who is not on the list gets
			// ROOM_TYPE_NOT_ALLOWED, any other role the plain 403. Nobody else files bookings.
			endpoint{
				name: "POST /bookings [" + typ + "]", method: "POST",
				path: func(*env) string { return "/api/v1/bookings" },
				// A letter is mandatory for ORGANISASI and AUDITORIUM, and AUDITORIUM needs a head count, so every
				// requester sends both; the letter belongs to the sender.
				bodyAs: func(e *env, identifier string) any {
					extra := map[string]any{"participants": 10}
					if identifier == mahasiswaA || identifier == orgID {
						extra["letter_attachment_id"] = e.letterFor(e.login(identifier).accessToken)
					}
					return bookingBody(e.freshRoomOfType(typ), at(tue, "08:00"), at(tue, "09:00"), extra)
				},
				want: only(http.StatusCreated, requesters...),
			},

			// The booking belongs to the student (MAHASISWA), so another requester (ORGANISASI) gets 404; managers see it
			// only when they approve the room's type, except ADMIN who reads everything; OB never reads bookings.
			endpoint{
				name: "GET /bookings/{id} [" + typ + "]", method: "GET",
				path: func(e *env) string { return "/api/v1/bookings/" + e.pendingBookingIn(typ, mahasiswaA) },
				want: bookingReadable(approvers),
			},
			endpoint{
				name: "GET /bookings/{id}/events [" + typ + "]", method: "GET",
				path: func(e *env) string { return "/api/v1/bookings/" + e.pendingBookingIn(typ, mahasiswaA) + "/events" },
				want: bookingReadable(approvers),
			},
			endpoint{
				name: "POST /bookings/{id}/approve [" + typ + "]", method: "POST",
				path: func(e *env) string { return "/api/v1/bookings/" + e.pendingBookingIn(typ, mahasiswaA) + "/approve" },
				want: only(http.StatusOK, approvers...),
			},
			endpoint{
				name: "POST /bookings/{id}/reject [" + typ + "]", method: "POST",
				path: func(e *env) string { return "/api/v1/bookings/" + e.pendingBookingIn(typ, mahasiswaA) + "/reject" },
				body: func(*env) any { return map[string]any{"reason": "Dipakai ujian"} },
				want: only(http.StatusOK, approvers...),
			},
			endpoint{
				// The booking is pending and belongs to the student, so the owner may cancel it. Approvers of the type pass
				// the authority gate and then get 409 INVALID_STATE (they cancel only approved bookings). Other managers
				// are stopped by the room type (403) and a requester who is not the owner by ownership (404).
				name: "POST /bookings/{id}/cancel (pending) [" + typ + "]", method: "POST",
				path: func(e *env) string { return "/api/v1/bookings/" + e.pendingBookingIn(typ, mahasiswaA) + "/cancel" },
				body: func(*env) any { return map[string]any{"reason": "Tidak jadi dipakai"} },
				want: only(http.StatusConflict, approvers...).with(http.StatusOK, "MAHASISWA").with(http.StatusNotFound, "ORGANISASI"),
			},
			endpoint{
				// An approved booking is cancelled by an approver of the type with a reason (BR-04, BR-10), and by its owner
				// while it has not started (BR-14; it starts days after the test clock, so no violation is recorded).
				name: "POST /bookings/{id}/cancel (approved) [" + typ + "]", method: "POST",
				path: func(e *env) string { return "/api/v1/bookings/" + e.approvedBookingIn(typ, mahasiswaA) + "/cancel" },
				body: func(*env) any { return map[string]any{"reason": "Ruang diperbaiki"} },
				want: only(http.StatusOK, approvers...).with(http.StatusOK, "MAHASISWA").with(http.StatusNotFound, "ORGANISASI"),
			},

			// Damage reports follow the same rule: the reporter (MAHASISWA) and the approvers of the room's type see
			// it, ADMIN reads everything, nobody else does (404). Only the approvers change its status.
			endpoint{
				name: "GET /reports/{id} [" + typ + "]", method: "GET",
				path: func(e *env) string { return "/api/v1/reports/" + e.reportIn(typ, nil) },
				want: reportReadable(approvers),
			},
			// The photo of a report is opened through /attachments (the per-report route is gone): the uploader, ADMIN, OB
			// (damage photos) and the approvers of the type; nobody else (404).
			endpoint{
				name: "GET /attachments/{id} (report photo) [" + typ + "]", method: "GET",
				path: func(e *env) string { return "/api/v1/attachments/" + e.photoOfReportIn(typ) },
				want: everyone(http.StatusOK).with(http.StatusNotFound, "ORGANISASI", "TU", "KALAB").with(http.StatusOK, approvers...),
			},
			// The QR is a secret of the room: the managers of its type and ADMIN read and rotate it (BR-10, BR-11).
			endpoint{
				name: "GET /rooms/{id}/qr [" + typ + "]", method: "GET",
				path: func(e *env) string { return "/api/v1/rooms/" + e.freshRoomOfType(typ) + "/qr" },
				want: only(http.StatusOK, writers...),
			},
			endpoint{
				name: "POST /rooms/{id}/qr/rotate [" + typ + "]", method: "POST",
				path: func(e *env) string { return "/api/v1/rooms/" + e.freshRoomOfType(typ) + "/qr/rotate" },
				want: only(http.StatusOK, writers...),
			},
			// Only the owner of a booking in use checks out; another requester gets 404, every other role 403.
			endpoint{
				name: "POST /bookings/{id}/check-out [" + typ + "]", method: "POST",
				call: func(e *env, token string) response {
					id := e.insertBooking(e.freshRoomOfType(typ), mahasiswaA, "BERLANGSUNG", "Fixture", at(tue, "08:00"), at(tue, "10:00"))
					return e.checkOut(token, id, map[string]string{"condition": "NORMAL"}, goodPhoto())
				},
				want: only(http.StatusOK, "MAHASISWA").with(http.StatusNotFound, "ORGANISASI"),
			},
			endpoint{
				name: "PATCH /reports/{id}/status [" + typ + "]", method: "PATCH",
				path: func(e *env) string { return "/api/v1/reports/" + e.reportIn(typ, nil) + "/status" },
				body: func(*env) any { return map[string]any{"status": "DITERIMA"} },
				want: only(http.StatusOK, approvers...),
			},
		)
		if typ != "AUDITORIUM" {
			endpoints = append(endpoints, endpoint{
				name: "PATCH /bookings/{id} [" + typ + "]", method: "PATCH",
				path: func(e *env) string { return "/api/v1/bookings/" + e.pendingBookingIn(typ, mahasiswaA) },
				body: func(*env) any { return map[string]any{"notes": "Butuh proyektor"} },
				want: only(http.StatusOK, "MAHASISWA").with(http.StatusNotFound, "ORGANISASI"),
			})
		}
	}

	// The fixtures file many bookings for the same student; the quota is not what this test is about.
	e := newEnv(t, func(c *config.Config) { c.Business.MaxActiveBookings = 100000 })
	for _, ep := range endpoints {
		for _, role := range allRoles {
			want := ep.want[role.name]
			if want == skip {
				continue
			}
			t.Run(ep.name+" as "+role.name, func(t *testing.T) {
				var opts []reqOpt
				if role.identifier != "" {
					opts = append(opts, bearer(e.login(role.identifier).accessToken))
				}
				var res response
				if ep.call != nil {
					token := ""
					if role.identifier != "" {
						token = e.login(role.identifier).accessToken
					}
					res = ep.call(e, token)
				} else {
					var body any
					switch {
					case ep.bodyAs != nil:
						body = ep.bodyAs(e, role.identifier)
					case ep.body != nil:
						body = ep.body(e)
					}
					res = e.do(ep.method, ep.path(e), body, opts...)
				}
				if res.Code != want {
					t.Errorf("status = %d; want %d (body %s)", res.Code, want, res.Body.String())
				}
				if res.Code >= 400 && res.errorCode() == "" {
					t.Errorf("error response is not the standard envelope: %s", res.Body.String())
				}
			})
		}
	}
}

// bookingReadable is who may read a MAHASISWA's booking in a room whose type is approved by approvers:
// the owner and ADMIN always, TU/KALAB when they approve the type (404 otherwise), another requester never.
func bookingReadable(approvers []string) expect {
	x := everyone(http.StatusOK).
		with(http.StatusNotFound, "ORGANISASI", "TU", "KALAB").
		with(http.StatusForbidden, "OB")
	return x.with(http.StatusOK, approvers...)
}

// reportReadable is the same for damage reports, except that OB (who may report) sees only their own: 404.
func reportReadable(approvers []string) expect {
	x := everyone(http.StatusOK).
		with(http.StatusNotFound, "ORGANISASI", "TU", "KALAB", "OB")
	return x.with(http.StatusOK, approvers...)
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

// A requester who is not on the type's requester_roles gets 403 ROOM_TYPE_NOT_ALLOWED and nothing is
// written: no booking, no event, no notification to the approvers (BR-17, BR-20).
func TestBookingByARoleOutsideTheTypesRequesterRoles(t *testing.T) {
	e := newEnv(t)
	tokens := map[string]string{"MAHASISWA": e.login(mahasiswaA).accessToken, "ORGANISASI": e.orgToken()}

	for _, rt := range roomTypeRules {
		room := e.freshRoomOfType(rt.typ)
		for role, tok := range tokens {
			events, notes := e.countRows("SELECT count(*) FROM booking_events"), e.countRows("SELECT count(*) FROM notifications")
			// Organisations and auditorium requests need a letter, and an auditorium a head count (BR-18, BR-19).
			res := e.postBooking(tok, room, at(tue, "08:00"), at(tue, "09:00"), map[string]any{"participants": 10, "letter_attachment_id": e.letterFor(tok)})
			if slices.Contains(rt.requesters, role) {
				if res.Code != http.StatusCreated {
					t.Errorf("%s booking a %s room: %d %s; want 201", role, rt.typ, res.Code, res.Body.String())
				}
				// Free the slot so the next role is judged on its own role, not on a clash.
				mustStatus(t, e.action(tok, idOf(res), "cancel", nil), http.StatusOK)
				continue
			}
			if res.Code != http.StatusForbidden || res.errorCode() != "ROOM_TYPE_NOT_ALLOWED" {
				t.Errorf("%s booking a %s room: %d %s; want 403 ROOM_TYPE_NOT_ALLOWED", role, rt.typ, res.Code, res.Body.String())
			}
			if ev, nt := e.countRows("SELECT count(*) FROM booking_events"), e.countRows("SELECT count(*) FROM notifications"); ev != events || nt != notes {
				t.Errorf("%s booking a %s room left traces: events %d -> %d, notifications %d -> %d", role, rt.typ, events, ev, notes, nt)
			}
		}
		if n := e.countRows("SELECT count(*) FROM bookings WHERE room_id = $1 AND status = 'MENUNGGU'", room); n != 0 {
			t.Errorf("%s: %d pending booking(s) left behind", rt.typ, n)
		}
	}
}

// Within a type the first decision wins, and a decision by someone outside the type's approvers is refused
// without touching the booking, whatever its state (BR-10, agent.md section 7.3).
func TestOnlyTheTypesApproversDecideBookings(t *testing.T) {
	e := newEnv(t)
	tokens := map[string]string{"TU": e.tuToken(), "KALAB": e.kalabToken(), "ADMIN": e.adminToken()}

	for _, rt := range roomTypeRules {
		for role, tok := range tokens {
			may := slices.Contains(rt.approvers, role)

			id := e.pendingBookingIn(rt.typ, mahasiswaA)
			res := e.action(tok, id, "approve", nil)
			switch {
			case may && res.Code != http.StatusOK:
				t.Errorf("%s approving a %s booking: %d %s; want 200", role, rt.typ, res.Code, res.Body.String())
			case !may && (res.Code != http.StatusForbidden || res.errorCode() != "FORBIDDEN"):
				t.Errorf("%s approving a %s booking: %d %s; want 403 FORBIDDEN", role, rt.typ, res.Code, res.Body.String())
			}
			if want := map[bool]string{true: "DISETUJUI", false: "MENUNGGU"}[may]; e.bookingStatus(id) != want {
				t.Errorf("%s approving a %s booking left it %s; want %s", role, rt.typ, e.bookingStatus(id), want)
			}

			if may {
				// A second approver of the same type arrives too late: INVALID_STATE, not a second decision.
				for other, otherTok := range tokens {
					if other != role && slices.Contains(rt.approvers, other) {
						if res := e.action(otherTok, id, "reject", map[string]string{"reason": "Terlambat memutuskan"}); res.Code != http.StatusConflict || res.errorCode() != "INVALID_STATE" {
							t.Errorf("%s rejecting what %s approved: %d %s; want 409 INVALID_STATE", other, role, res.Code, res.Body.String())
						}
					}
				}
			}

			id = e.approvedBookingIn(rt.typ, mahasiswaA)
			res = e.action(tok, id, "cancel", map[string]string{"reason": "Ruang diperbaiki"})
			switch {
			case may && res.Code != http.StatusOK:
				t.Errorf("%s cancelling a %s booking: %d %s; want 200", role, rt.typ, res.Code, res.Body.String())
			case !may && res.Code != http.StatusForbidden:
				t.Errorf("%s cancelling a %s booking: %d %s; want 403", role, rt.typ, res.Code, res.Body.String())
			}
			if want := map[bool]string{true: "DIBATALKAN", false: "DISETUJUI"}[may]; e.bookingStatus(id) != want {
				t.Errorf("%s cancelling a %s booking left it %s; want %s", role, rt.typ, e.bookingStatus(id), want)
			}
		}
	}
}

// Listings are scoped by room type too: a TU or KALAB sees only the bookings and reports of the types they
// approve, ADMIN sees all, and a requester only their own (BR-10).
func TestBookingAndReportListsFollowRoomTypeAuthority(t *testing.T) {
	e := newEnv(t)
	student := e.login(mahasiswaA).accessToken

	bookings := map[string]string{} // type -> id
	reports := map[string]string{}
	for _, rt := range roomTypeRules {
		bookings[rt.typ] = e.pendingBookingIn(rt.typ, mahasiswaA)
		res := e.postReport(student, e.freshRoomOfType(rt.typ), "Rusak di "+rt.typ, nil)
		mustStatus(t, res, http.StatusCreated)
		reports[rt.typ] = idOf(res)
	}

	ids := func(path, tok string) []string {
		res := e.do("GET", path+"?per_page=100", nil, bearer(tok))
		mustStatus(t, res, http.StatusOK)
		var out []string
		for _, it := range dataOf(res) {
			out = append(out, it.(map[string]any)["id"].(string))
		}
		slices.Sort(out)
		return out
	}
	// expected lists the ids a role should see: those of the types it approves, everything for ADMIN, and the
	// student's own for MAHASISWA.
	expected := func(byType map[string]string, role string) []string {
		var out []string
		for _, rt := range roomTypeRules {
			if role == "ADMIN" || role == "MAHASISWA" || slices.Contains(rt.approvers, role) {
				out = append(out, byType[rt.typ])
			}
		}
		slices.Sort(out)
		return out
	}

	for role, tok := range map[string]string{"TU": e.tuToken(), "KALAB": e.kalabToken(), "ADMIN": e.adminToken(), "MAHASISWA": student} {
		if got, want := ids("/api/v1/bookings", tok), expected(bookings, role); !slices.Equal(got, want) {
			t.Errorf("%s sees bookings %v; want %v", role, got, want)
		}
		if got, want := ids("/api/v1/reports", tok), expected(reports, role); !slices.Equal(got, want) {
			t.Errorf("%s sees reports %v; want %v", role, got, want)
		}
	}
	// Other requesters and the OB see none of the student's bookings or reports.
	for role, tok := range map[string]string{"ORGANISASI": e.orgToken(), "OB": e.obToken()} {
		if role == "ORGANISASI" {
			if got := ids("/api/v1/bookings", tok); len(got) != 0 {
				t.Errorf("%s sees someone else's bookings: %v", role, got)
			}
		}
		if got := ids("/api/v1/reports", tok); len(got) != 0 {
			t.Errorf("%s sees someone else's reports: %v", role, got)
		}
	}
}

// TU may manage student accounts only; the role stays with ADMIN (BR-24, agent.md section 7.3).
func TestTUManagesOnlyStudentAccounts(t *testing.T) {
	e := newEnv(t)
	tu := bearer(e.tuToken())
	student, kalab, admin := e.userIDOf("2021101005"), e.userIDOf(kalabID), e.userIDOf(adminID)

	// Listing is limited to students: asking for another role is refused instead of returning an empty page.
	res := e.do("GET", "/api/v1/users?per_page=100", nil, tu)
	mustStatus(t, res, http.StatusOK)
	for _, it := range dataOf(res) {
		if role := it.(map[string]any)["role"]; role != "MAHASISWA" {
			t.Errorf("TU listed an account with role %v", role)
		}
	}
	if got := len(dataOf(res)); got != 5 {
		t.Errorf("TU sees %d accounts; want the 5 students", got)
	}
	for _, role := range []string{"ADMIN", "KALAB", "ORGANISASI", "OB", "TU"} {
		if r := e.do("GET", "/api/v1/users?role="+role, nil, tu); r.Code != http.StatusForbidden {
			t.Errorf("TU listing role %s: %d; want 403", role, r.Code)
		}
	}
	if r := e.do("GET", "/api/v1/users?role=MAHASISWA", nil, tu); r.Code != http.StatusOK {
		t.Errorf("TU listing students: %d; want 200", r.Code)
	}

	// A student's name, email and activation may change; nothing about the role or the contact person.
	mustStatus(t, e.do("PATCH", "/api/v1/users/"+student, map[string]any{"name": "Nama Baru", "email": "baru@kampus.test", "is_active": false}, tu), http.StatusOK)
	mustStatus(t, e.do("PATCH", "/api/v1/users/"+student, map[string]any{"is_active": true}, tu), http.StatusOK)
	for name, body := range map[string]map[string]any{
		"role":                {"role": "TU"},
		"contact person":      {"pic_name": "X"},
		"contact phone":       {"pic_phone": "081234567890"},
		"clear booking block": {"clear_booking_block": true},
	} {
		if r := e.do("PATCH", "/api/v1/users/"+student, body, tu); r.Code != http.StatusForbidden {
			t.Errorf("TU changing the %s of a student: %d; want 403", name, r.Code)
		}
	}
	if role := e.countRows("SELECT count(*) FROM users WHERE id = $1 AND role = 'MAHASISWA'", student); role != 1 {
		t.Error("a refused request changed the student's role")
	}

	// Staff accounts are out of reach, even for harmless fields.
	for name, id := range map[string]string{"KALAB": kalab, "ADMIN": admin} {
		if r := e.do("PATCH", "/api/v1/users/"+id, map[string]any{"name": "Dicoba TU"}, tu); r.Code != http.StatusForbidden {
			t.Errorf("TU renaming a %s account: %d; want 403", name, r.Code)
		}
	}
	if n := e.countRows("SELECT count(*) FROM users WHERE name = 'Dicoba TU'"); n != 0 {
		t.Errorf("%d staff account(s) were renamed by a TU", n)
	}

	// Creating accounts is for ADMIN alone, students included.
	if r := e.do("POST", "/api/v1/users", e.newUserBody("2022202999"), tu); r.Code != http.StatusForbidden {
		t.Errorf("TU creating a student: %d; want 403", r.Code)
	}
	if n := e.countRows("SELECT count(*) FROM users WHERE identifier = '2022202999'"); n != 0 {
		t.Error("a refused creation still wrote a row")
	}
}

// photoOfReportIn files a report with a photo in a fresh room of the type and returns the photo's attachment id.
func (e *env) photoOfReportIn(typ string) string {
	e.t.Helper()
	res := e.postReport(e.login(mahasiswaA).accessToken, e.freshRoomOfType(typ), "Rusak", &photoPart{"a.png", "image/png", pngBytes(64)})
	mustStatus(e.t, res, http.StatusCreated)
	id, _ := res.json()["attachment_id"].(string)
	return id
}

// violationOf records a no-show violation for the student (on a booking inserted for the purpose) and returns its id.
func (e *env) violationOf(student string) string {
	e.t.Helper()
	booking := e.insertBooking(e.freshRoom(), student, "TIDAK_HADIR", "Fixture", at(tue, "08:00"), at(tue, "10:00"))
	var id string
	if err := e.pool.QueryRow(e.t.Context(),
		`INSERT INTO violations (user_id, booking_id, kind, points, created_at) VALUES ($1, $2, 'TIDAK_HADIR', 2, $3) RETURNING id`,
		e.userIDOf(student), booking, e.clock.Now()).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}
