// Package overlap finds what already occupies a room, for the writes that must not double-book it:
// schedule create/update and booking create/approve (brief 4.4). Callers hold the room lock.
package overlap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	pgExclusionViolation = "23P01"

	// MaxList bounds how many collisions one response reports.
	MaxList = 50
)

// Window is one concrete interval, as UTC instants.
type Window struct {
	Start time.Time
	End   time.Time
}

// Find lists existing schedules and live bookings (MENUNGGU holds the slot, BR-03; so does BERLANGSUNG) that overlap any
// window. Intervals are half-open, so back-to-back sessions do not collide (BR-02).
// excludeSchedule and excludeBooking name the row being edited or approved, which must not conflict
// with itself; pass uuid.Nil for none.
func Find(ctx context.Context, q *dbgen.Queries, roomID uuid.UUID, windows []Window, excludeSchedule, excludeBooking uuid.UUID) ([]httpx.ErrorDetail, error) {
	var out []httpx.ErrorDetail
	seen := map[uuid.UUID]bool{}

	for _, w := range windows {
		scheds, err := q.ListSchedulesOverlapping(ctx, dbgen.ListSchedulesOverlappingParams{
			RoomIds: []uuid.UUID{roomID}, RangeStart: w.Start, RangeEnd: w.End,
		})
		if err != nil {
			return nil, fmt.Errorf("checking schedule overlaps: %w", err)
		}
		for _, c := range scheds {
			if c.ID == excludeSchedule || seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			out = append(out, httpx.ErrorDetail{
				Kind:    "SCHEDULE",
				Message: fmt.Sprintf("Bentrok dengan jadwal %q.", c.Title),
				StartAt: clock.ToWIB(c.StartAt).Format(time.RFC3339),
				EndAt:   clock.ToWIB(c.EndAt).Format(time.RFC3339),
			})
		}

		books, err := q.ListBookingsOverlapping(ctx, dbgen.ListBookingsOverlappingParams{
			RoomIds:    []uuid.UUID{roomID},
			Statuses:   []string{string(dbgen.BookingStatusMENUNGGU), string(dbgen.BookingStatusDISETUJUI), string(dbgen.BookingStatusBERLANGSUNG)},
			RangeStart: w.Start, RangeEnd: w.End,
		})
		if err != nil {
			return nil, fmt.Errorf("checking booking overlaps: %w", err)
		}
		for _, c := range books {
			if c.ID == excludeBooking || seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			out = append(out, httpx.ErrorDetail{
				Kind:    "BOOKING",
				Status:  string(c.Status),
				Message: fmt.Sprintf("Bentrok dengan booking %s.", strings.ToLower(string(c.Status))),
				StartAt: clock.ToWIB(c.StartAt).Format(time.RFC3339),
				EndAt:   clock.ToWIB(c.EndAt).Format(time.RFC3339),
			})
		}
	}

	if len(out) > MaxList {
		out = out[:MaxList]
	}
	return out, nil
}

// Error is the 409 BOOKING_CONFLICT response carrying what collided.
func Error(message string, details []httpx.ErrorDetail) error {
	return httpx.NewCustomError(http.StatusConflict, "BOOKING_CONFLICT", message, details, httpx.ErrBookingConflict)
}

// MapWriteError turns storage errors into API errors. The exclusion constraint is the last line of
// defence (brief 4.4); the room lock should make it unreachable, but a race must still be a 409.
func MapWriteError(err error, op, conflictMessage string) error {
	var domain *httpx.CustomDomainError
	if errors.As(err, &domain) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgExclusionViolation {
		return Error(conflictMessage, nil)
	}
	return fmt.Errorf("%s: %w", op, err)
}
