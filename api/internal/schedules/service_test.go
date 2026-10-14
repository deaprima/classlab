package schedules

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapWriteErrorTurnsExclusionViolationInto409(t *testing.T) {
	err := mapWriteError(fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23P01"}), "creating schedule")

	status, env := httpx.MapError(err, "req_1")
	if status != http.StatusConflict || env.Error.Code != "BOOKING_CONFLICT" {
		t.Fatalf("got %d %s; want 409 BOOKING_CONFLICT", status, env.Error.Code)
	}
}

func TestMapWriteErrorPassesDomainErrorsThrough(t *testing.T) {
	in := httpx.ValidationError(httpx.ErrorDetail{Field: "room_id", Message: "x"})
	if got := mapWriteError(in, "op"); got != error(in) {
		t.Errorf("domain error was rewrapped: %v", got)
	}
	if got := mapWriteError(pgx.ErrNoRows, "op"); !errors.Is(got, httpx.ErrNotFound) {
		t.Errorf("no rows = %v; want not found", got)
	}
}

func TestMapWriteErrorDoesNotLeakInternals(t *testing.T) {
	err := mapWriteError(errors.New(`pq: relation "secret_table" does not exist`), "creating schedule")
	status, env := httpx.MapError(err, "req_1")
	if status != http.StatusInternalServerError || env.Error.Message == "" || env.Error.Code != "INTERNAL_SERVER_ERROR" {
		t.Fatalf("got %d %+v", status, env.Error)
	}
	if got := env.Error.Message; got == err.Error() {
		t.Errorf("response message leaks the internal error: %q", got)
	}
}

func TestValidateWindow(t *testing.T) {
	d := func(h, m int) time.Time { return time.Date(2026, 10, 20, h, m, 0, 0, clock.WIB) }

	if p := validateWindow(d(8, 0), d(10, 0)); p != nil {
		t.Errorf("ordinary window rejected: %v", p)
	}
	if p := validateWindow(d(0, 0), d(23, 59)); p != nil {
		t.Errorf("whole working day rejected: %v", p)
	}
	for name, w := range map[string][2]time.Time{
		"zero start":    {{}, d(10, 0)},
		"equal":         {d(8, 0), d(8, 0)},
		"reversed":      {d(10, 0), d(8, 0)},
		"next day":      {d(22, 0), d(22, 0).Add(3 * time.Hour)},
		"ends midnight": {d(22, 0), d(22, 0).Add(2 * time.Hour)},
	} {
		if p := validateWindow(w[0], w[1]); len(p) == 0 {
			t.Errorf("%s: accepted", name)
		}
	}

	// 17:00 UTC is already the next day in WIB, so this window crosses midnight there.
	utcStart := time.Date(2026, 10, 20, 16, 0, 0, 0, time.UTC)
	if p := validateWindow(utcStart, utcStart.Add(2*time.Hour)); len(p) == 0 {
		t.Error("a window that is one UTC day but two WIB days must be rejected")
	}
}
