package bookings

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"classlab/api/internal/config"
	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"

	"github.com/jackc/pgx/v5/pgconn"
)

func testService() *Service {
	return &Service{rules: config.BookingRules{
		OpenMinute: 7 * 60, CloseMinute: 21 * 60,
		MinDuration: time.Hour, MaxDuration: 4 * time.Hour, MaxFutureDays: 7,
	}}
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

func TestValidateWindowHonoursConfiguredLimits(t *testing.T) {
	s := testService()
	now := wib(14, 10, 0)

	cases := []struct {
		name       string
		start, end time.Time
		want       []string // fields that must be reported, empty for a valid window
	}{
		{"valid", wib(15, 8, 0), wib(15, 10, 0), nil},
		{"exactly the minimum", wib(15, 8, 0), wib(15, 9, 0), nil},
		{"exactly the maximum", wib(15, 8, 0), wib(15, 12, 0), nil},
		{"below the configured minimum", wib(15, 8, 0), wib(15, 8, 30), []string{"end_at"}},
		{"above the configured maximum", wib(15, 8, 0), wib(15, 12, 30), []string{"end_at"}},
		{"opens at the opening minute", wib(15, 7, 0), wib(15, 9, 0), nil},
		{"closes at the closing minute", wib(15, 19, 0), wib(15, 21, 0), nil},
		{"starts a slot early", wib(15, 6, 30), wib(15, 8, 30), []string{"start_at"}},
		{"ends a slot late", wib(15, 19, 30), wib(15, 21, 30), []string{"end_at"}},
		{"on the last allowed day", wib(21, 8, 0), wib(21, 10, 0), nil},
		{"one day too far", wib(22, 8, 0), wib(22, 10, 0), []string{"start_at"}},
		{"starting right now is not the past", now, now.Add(time.Hour), nil},
		{"one slot ago", wib(14, 9, 30), wib(14, 11, 30), []string{"start_at"}},
		{"odd seconds", wib(15, 8, 0).Add(5 * time.Second), wib(15, 10, 0), []string{"start_at"}},
		{"both ends off the grid", wib(15, 8, 10), wib(15, 10, 20), []string{"end_at", "start_at"}},
		{"reversed", wib(15, 10, 0), wib(15, 8, 0), []string{"end_at"}},
		{"crosses midnight", wib(15, 20, 0), wib(16, 0, 0), []string{"end_at"}},
		{"nothing given", time.Time{}, time.Time{}, []string{"end_at", "start_at"}},
	}

	for _, tc := range cases {
		got := fieldsOf(s.validateWindow(tc.start, tc.end, now))
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: fields %v; want %v", tc.name, got, tc.want)
		}
	}
}

func TestValidateWindowUsesWIBForTheCalendarDay(t *testing.T) {
	s := testService()
	// 20:00 UTC on the 14th is 03:00 WIB on the 15th: "today" for the 7-day limit is the WIB date.
	now := time.Date(2026, 10, 14, 20, 0, 0, 0, time.UTC)
	if p := s.validateWindow(wib(22, 8, 0), wib(22, 10, 0), now); len(p) != 0 {
		t.Errorf("22nd is 7 days after the WIB date 15th, got %v", p)
	}
	if p := s.validateWindow(wib(23, 8, 0), wib(23, 10, 0), now); len(p) == 0 {
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
