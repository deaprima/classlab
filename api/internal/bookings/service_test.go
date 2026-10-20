package bookings

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/roomtypes"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func testService() *Service { return &Service{} }

// testPolicy is a small, tight policy so every limit is easy to hit: open 07:00 to 21:00, durations of
// one to four hours, bookable up to seven days ahead.
func testPolicy() roomtypes.Policy {
	return roomtypes.Policy{
		Type: "KELAS", OpenMinute: 7 * 60, CloseMinute: 21 * 60,
		MinDuration: time.Hour, MaxDuration: 4 * time.Hour, MaxAdvanceDays: 7,
	}
}

func wib(day, h, m int) time.Time { return time.Date(2026, 10, day, h, m, 0, 0, clock.WIB) }

func fieldsOf(details []httpx.ErrorDetail) []string {
	var out []string
	for _, d := range details {
		out = append(out, d.Field)
	}
	slices.Sort(out)
	return out
}

// outcome flattens what validatePolicyWindow returns into a code and the fields it names.
func outcome(err error) (code string, fields []string) {
	if err == nil {
		return "", nil
	}
	var ce *httpx.CustomDomainError
	if !errors.As(err, &ce) {
		return "unexpected: " + err.Error(), nil
	}
	return ce.Code, fieldsOf(ce.Details)
}

func TestValidateWindowChecksOnlyWhatDoesNotDependOnTheRoom(t *testing.T) {
	now := wib(14, 10, 0)

	cases := []struct {
		name       string
		start, end time.Time
		want       []string // fields that must be reported, empty for a valid window
	}{
		{"valid", wib(15, 8, 0), wib(15, 10, 0), nil},
		{"a short slot is fine here; the type decides durations", wib(15, 8, 0), wib(15, 8, 30), nil},
		{"outside any hours is fine here; the room decides hours", wib(15, 3, 0), wib(15, 5, 0), nil},
		{"starting right now is not the past", now, now.Add(time.Hour), nil},
		{"one slot ago", wib(14, 9, 30), wib(14, 11, 30), []string{"start_at"}},
		{"odd seconds", wib(15, 8, 0).Add(5 * time.Second), wib(15, 10, 0), []string{"start_at"}},
		{"both ends off the grid", wib(15, 8, 10), wib(15, 10, 20), []string{"end_at", "start_at"}},
		{"reversed", wib(15, 10, 0), wib(15, 8, 0), []string{"end_at"}},
		{"crosses midnight", wib(15, 20, 0), wib(16, 0, 0), []string{"end_at"}},
		{"nothing given", time.Time{}, time.Time{}, []string{"end_at", "start_at"}},
	}
	for _, tc := range cases {
		got := fieldsOf(validateWindow(tc.start, tc.end, now))
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: fields %v; want %v", tc.name, got, tc.want)
		}
	}
}

func TestValidatePolicyWindowFollowsTheTypesLimits(t *testing.T) {
	now := wib(14, 10, 0)
	none := pgtype.Time{}

	cases := []struct {
		name       string
		start, end time.Time
		code       string
		fields     []string
	}{
		{"valid", wib(15, 8, 0), wib(15, 10, 0), "", nil},
		{"exactly the minimum", wib(15, 8, 0), wib(15, 9, 0), "", nil},
		{"exactly the maximum", wib(15, 8, 0), wib(15, 12, 0), "", nil},
		{"below the minimum", wib(15, 8, 0), wib(15, 8, 30), "VALIDATION_ERROR", []string{"end_at"}},
		{"above the maximum", wib(15, 8, 0), wib(15, 12, 30), "VALIDATION_ERROR", []string{"end_at"}},
		{"opens at the opening minute", wib(15, 7, 0), wib(15, 9, 0), "", nil},
		{"closes at the closing minute", wib(15, 19, 0), wib(15, 21, 0), "", nil},
		{"starts a slot before opening", wib(15, 6, 30), wib(15, 8, 30), "OUTSIDE_OPERATING_HOURS", []string{"close_time", "open_time"}},
		{"ends a slot after closing", wib(15, 19, 30), wib(15, 21, 30), "OUTSIDE_OPERATING_HOURS", []string{"close_time", "open_time"}},
		{"on the last allowed day", wib(21, 8, 0), wib(21, 10, 0), "", nil},
		{"one day too far", wib(22, 8, 0), wib(22, 10, 0), "VALIDATION_ERROR", []string{"start_at"}},
	}
	for _, tc := range cases {
		code, fields := outcome(validatePolicyWindow(testPolicy(), none, none, tc.start, tc.end, now))
		if code != tc.code || !slices.Equal(fields, tc.fields) {
			t.Errorf("%s: %q %v; want %q %v", tc.name, code, fields, tc.code, tc.fields)
		}
	}
}

func TestValidatePolicyWindowPrefersTheRoomsOwnHours(t *testing.T) {
	now := wib(14, 10, 0)
	hours := func(open, closing int) (pgtype.Time, pgtype.Time) {
		return roomtypes.PGTime(open), roomtypes.PGTime(closing)
	}

	// A room may open longer than its type's default window (BR-29).
	open, closing := hours(6*60, 22*60)
	if code, _ := outcome(validatePolicyWindow(testPolicy(), open, closing, wib(15, 6, 0), wib(15, 8, 0), now)); code != "" {
		t.Errorf("room opening at 06:00: %q, want allowed", code)
	}
	// And a tighter room window beats the type's wider one.
	open, closing = hours(9*60, 17*60)
	if code, _ := outcome(validatePolicyWindow(testPolicy(), open, closing, wib(15, 8, 0), wib(15, 10, 0), now)); code != "OUTSIDE_OPERATING_HOURS" {
		t.Errorf("08:00 in a room that opens at 09:00: %q, want OUTSIDE_OPERATING_HOURS", code)
	}
	if code, _ := outcome(validatePolicyWindow(testPolicy(), open, closing, wib(15, 15, 0), wib(15, 18, 0), now)); code != "OUTSIDE_OPERATING_HOURS" {
		t.Errorf("ending at 18:00 in a room that closes at 17:00: %q, want OUTSIDE_OPERATING_HOURS", code)
	}
	if code, _ := outcome(validatePolicyWindow(testPolicy(), open, closing, wib(15, 9, 0), wib(15, 17, 0), now)); code == "OUTSIDE_OPERATING_HOURS" {
		t.Error("a booking exactly filling the room's window was refused")
	}

	// The error names the window that applied, so the client can show it.
	err := validatePolicyWindow(testPolicy(), open, closing, wib(15, 8, 0), wib(15, 10, 0), now)
	var ce *httpx.CustomDomainError
	if !errors.As(err, &ce) || len(ce.Details) != 2 || ce.Details[0].Message != "09:00" || ce.Details[1].Message != "17:00" {
		t.Errorf("details = %+v, want the room's 09:00-17:00 window", ce)
	}
	if ce != nil && ce.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", ce.StatusCode)
	}
}

func TestValidatePolicyWindowUsesWIBForTheCalendarDay(t *testing.T) {
	// 20:00 UTC on the 14th is 03:00 WIB on the 15th: "today" for the 7-day limit is the WIB date.
	now := time.Date(2026, 10, 14, 20, 0, 0, 0, time.UTC)
	none := pgtype.Time{}
	if err := validatePolicyWindow(testPolicy(), none, none, wib(22, 8, 0), wib(22, 10, 0), now); err != nil {
		t.Errorf("22nd is 7 days after the WIB date 15th, got %v", err)
	}
	if err := validatePolicyWindow(testPolicy(), none, none, wib(23, 8, 0), wib(23, 10, 0), now); err == nil {
		t.Error("23rd is 8 days after the WIB date and must be refused")
	}
}

func TestMapWriteError(t *testing.T) {
	s := testService()

	status, env := httpx.MapError(s.mapWriteError(fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23P01"}), "creating booking"), "req_1")
	if status != http.StatusConflict || env.Error.Code != "BOOKING_CONFLICT" {
		t.Errorf("exclusion violation: %d %s; want 409 BOOKING_CONFLICT", status, env.Error.Code)
	}

	status, env = httpx.MapError(s.mapWriteError(&pgconn.PgError{Code: "23505"}, "creating booking"), "req_1")
	if status != http.StatusUnprocessableEntity || env.Error.Details[0].Field != "Idempotency-Key" {
		t.Errorf("unique violation: %d %+v; want 422 naming Idempotency-Key", status, env.Error)
	}

	status, env = httpx.MapError(s.mapWriteError(errors.New(`relation "secret" does not exist`), "creating booking"), "req_1")
	if status != http.StatusInternalServerError || env.Error.Code != "INTERNAL_SERVER_ERROR" {
		t.Errorf("unknown error: %d %s; want a generic 500", status, env.Error.Code)
	}
}
