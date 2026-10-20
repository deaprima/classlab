// Package violations records the points a requester earns for not showing up, not checking out, or
// cancelling late, and blocks new requests when too many are active (brief BR-14). Recording is a function
// that runs inside the caller's transaction, so a violation exists exactly when the thing that caused it does.
package violations

import (
	"context"
	"fmt"
	"time"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	typeBlocked    = "BOOKING_BLOCKED"
	refTypeBooking = "BOOKING"
)

// Settings are the BR-14 values from configuration.
type Settings struct {
	Window    time.Duration // how far back points still count (VIOLATION_WINDOW_DAYS)
	Threshold int           // active points that trigger a block (BLOCK_THRESHOLD_POINTS)
	BlockFor  time.Duration // how long the block lasts (BLOCK_DAYS)
}

// Record stores one violation of kind for a booking. Recording the same kind for the same booking again
// does nothing, so a job that runs twice cannot double the points. When the user's active points reach
// the threshold the user is blocked and told, once; a block that already stands is not extended.
func Record(ctx context.Context, q *dbgen.Queries, s Settings, userID, bookingID uuid.UUID, kind dbgen.ViolationKind, points int, now time.Time) error {
	n, err := q.InsertViolation(ctx, dbgen.InsertViolationParams{
		UserID: userID, BookingID: bookingID, Kind: kind, Points: int32(points), CreatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("recording violation: %w", err)
	}
	if n == 0 {
		return nil
	}

	total, err := q.SumActiveViolationPoints(ctx, dbgen.SumActiveViolationPointsParams{UserID: userID, CreatedAt: now.Add(-s.Window)})
	if err != nil {
		return fmt.Errorf("summing violation points: %w", err)
	}
	if int(total) < s.Threshold {
		return nil
	}

	until := now.Add(s.BlockFor)
	started, err := q.BlockUserUntil(ctx, dbgen.BlockUserUntilParams{ID: userID, Until: pgtype.Timestamptz{Time: until, Valid: true}, Now: now})
	if err != nil {
		return fmt.Errorf("blocking user: %w", err)
	}
	if started == 0 {
		return nil
	}
	return q.CreateNotification(ctx, dbgen.CreateNotificationParams{
		UserID: userID, Type: typeBlocked, Title: "Pengajuan booking diblokir sementara",
		Body: fmt.Sprintf("Poin pelanggaran Anda mencapai %d. Anda tidak dapat mengajukan booking baru sampai %s WIB; booking yang sudah disetujui tetap berlaku.",
			total, clock.ToWIB(until).Format("02/01/2006 15:04")),
		RefType:   pgtype.Text{String: refTypeBooking, Valid: true},
		RefID:     pgtype.UUID{Bytes: bookingID, Valid: true},
		CreatedAt: now,
	})
}

// ActivePoints sums the points that are not annulled and still inside the window.
func ActivePoints(ctx context.Context, q *dbgen.Queries, s Settings, userID uuid.UUID, now time.Time) (int, error) {
	n, err := q.SumActiveViolationPoints(ctx, dbgen.SumActiveViolationPointsParams{UserID: userID, CreatedAt: now.Add(-s.Window)})
	if err != nil {
		return 0, fmt.Errorf("summing violation points: %w", err)
	}
	return int(n), nil
}
