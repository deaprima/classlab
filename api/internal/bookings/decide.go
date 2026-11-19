package bookings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/overlap"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/violations"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Cancel scopes (BR-27): one date or the whole multi-date request.
const (
	ScopeOccurrence = "OCCURRENCE"
	ScopeGroup      = "GROUP"
)

// target is what a decision works on, read and locked together: the room, and either the one booking or every
// row of its group. Every writer takes the locks in the same order (room, then group rows), so two decisions
// on one group cannot interleave and the re-read cannot go stale between the check and the update.
type target struct {
	room      dbgen.LockRoomRow
	addressed dbgen.Booking
	rows      []dbgen.Booking // the group, or just the addressed booking
}

func (s *Service) lockTarget(ctx context.Context, q *dbgen.Queries, id uuid.UUID, peek dbgen.GetBookingRow) (target, error) {
	room, err := q.LockRoom(ctx, peek.RoomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return target{}, httpx.ErrNotFound
		}
		return target{}, fmt.Errorf("locking room: %w", err)
	}
	t := target{room: room}
	if peek.GroupID.Valid {
		if t.rows, err = q.LockGroupBookings(ctx, peek.GroupID); err != nil {
			return target{}, fmt.Errorf("locking group: %w", err)
		}
		for _, r := range t.rows {
			if r.ID == id {
				t.addressed = r
			}
		}
		if t.addressed.ID == uuid.Nil {
			return target{}, httpx.ErrNotFound
		}
		return t, nil
	}
	t.addressed, err = q.LoadBooking(ctx, id)
	if err != nil {
		return target{}, fmt.Errorf("re-reading booking: %w", err)
	}
	t.rows = []dbgen.Booking{t.addressed}
	return t, nil
}

// peek reads the booking without locks. It finds the room to lock, and lets a requester who does not own
// the booking get a 404 without ever contending for the lock. A manager's authority is checked under the
// lock, against the room's current type.
func (s *Service) peek(ctx context.Context, actor httpx.Principal, id uuid.UUID) (dbgen.GetBookingRow, error) {
	row, err := s.q.GetBooking(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.GetBookingRow{}, httpx.ErrNotFound
	}
	if err != nil {
		return dbgen.GetBookingRow{}, fmt.Errorf("reading booking: %w", err)
	}
	if isRequester(actor) && row.RequesterID != actor.UserID {
		return dbgen.GetBookingRow{}, httpx.ErrNotFound
	}
	return row, nil
}

// Approve moves a waiting request to DISETUJUI, and every other waiting date of its group with it: there is
// no partial approval (BR-27). Each date is checked again against what occupies the room (brief 4.4), and a
// request past its expiry is refused even if the job has not marked it yet (BR-22). Only an approver of the
// room's type may do it (BR-10).
func (s *Service) Approve(ctx context.Context, actor httpx.Principal, id uuid.UUID) (Booking, error) {
	if !isManager(actor) {
		return Booking{}, httpx.ErrForbidden
	}
	return s.decideWaiting(ctx, actor, id, dbgen.BookingStatusDISETUJUI, "")
}

// Reject moves a waiting request, and its whole group, to DITOLAK; the reason is mandatory (BR-04).
func (s *Service) Reject(ctx context.Context, actor httpx.Principal, id uuid.UUID, reason string) (Booking, error) {
	if !isManager(actor) {
		return Booking{}, httpx.ErrForbidden
	}
	reason, err := requireReason(reason)
	if err != nil {
		return Booking{}, err
	}
	return s.decideWaiting(ctx, actor, id, dbgen.BookingStatusDITOLAK, reason)
}

func (s *Service) decideWaiting(ctx context.Context, actor httpx.Principal, id uuid.UUID, to dbgen.BookingStatus, reason string) (Booking, error) {
	peek, err := s.peek(ctx, actor, id)
	if err != nil {
		return Booking{}, err
	}

	var result Booking
	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		t, err := s.lockTarget(ctx, q, id, peek)
		if err != nil {
			return err
		}
		if err := s.types.RequireApprover(ctx, actor, string(t.room.Type)); err != nil {
			return err
		}

		verb := "disetujui"
		if to == dbgen.BookingStatusDITOLAK {
			verb = "ditolak"
		}
		if t.addressed.Status != dbgen.BookingStatusMENUNGGU {
			return invalidState(fmt.Sprintf("Booking berstatus %s tidak dapat %s.", t.addressed.Status, verb))
		}
		var waiting []dbgen.Booking
		for _, r := range t.rows {
			if r.Status == dbgen.BookingStatusMENUNGGU {
				waiting = append(waiting, r)
			}
		}

		for _, r := range waiting {
			if r.ExpiresAt.Valid && now.After(r.ExpiresAt.Time) {
				return invalidState("Pengajuan sudah kedaluwarsa dan tidak dapat diputuskan lagi.")
			}
			if to == dbgen.BookingStatusDISETUJUI {
				if !t.room.IsActive {
					return httpx.ErrRoomInactive
				}
				if !r.EndAt.After(now) {
					return invalidState("Waktu booking sudah lewat sehingga tidak dapat disetujui.")
				}
				conflicts, err := overlap.Find(ctx, q, t.room.ID, []overlap.Window{{Start: r.StartAt, End: r.EndAt}}, uuid.Nil, r.ID)
				if err != nil {
					return err
				}
				if len(conflicts) > 0 {
					return overlap.Error(conflictMessage, tagList(conflicts, occurrenceOf(r)))
				}
			}
		}

		for _, r := range waiting {
			if err := s.writeDecision(ctx, q, actor, r, to, reason, now); err != nil {
				return err
			}
		}

		view, err := q.GetBooking(ctx, id)
		if err != nil {
			return fmt.Errorf("reading updated booking: %w", err)
		}
		result = fromRow(view)
		return s.notifyDecided(ctx, q, result, len(waiting), actor.UserID, reason, now)
	})
	if err != nil {
		return Booking{}, s.mapWriteError(err, "updating booking")
	}
	return result, nil
}

// writeDecision changes one row's status and records the event beside it.
func (s *Service) writeDecision(ctx context.Context, q *dbgen.Queries, actor httpx.Principal, r dbgen.Booking, to dbgen.BookingStatus, reason string, now time.Time) error {
	text := pgtype.Text{String: reason, Valid: reason != ""}
	if _, err := q.DecideBooking(ctx, dbgen.DecideBookingParams{
		ID: r.ID, Status: to, DecisionReason: text,
		DecidedBy: pgtype.UUID{Bytes: actor.UserID, Valid: true},
		Now:       pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		return fmt.Errorf("updating booking status: %w", err)
	}
	if err := q.CreateBookingEvent(ctx, dbgen.CreateBookingEventParams{
		BookingID: r.ID, ActorID: pgtype.UUID{Bytes: actor.UserID, Valid: true}, ToStatus: to, Reason: text, Kind: "STATUS", CreatedAt: now,
		FromStatus: dbgen.NullBookingStatus{BookingStatus: r.Status, Valid: true},
	}); err != nil {
		return fmt.Errorf("recording booking event: %w", err)
	}
	return nil
}

// occurrenceOf is the date number of a row for error details, 0 for a booking that is not in a group.
func occurrenceOf(r dbgen.Booking) int {
	if r.OccurrenceNo.Valid {
		return int(r.OccurrenceNo.Int32)
	}
	return 0
}

// Cancel cancels one date or, with scope GROUP, every date of a multi-date request that can still be
// cancelled (BR-27). The owner may cancel a waiting or approved date that has not started; cancelling an
// approved date less than LATE_CANCEL before it starts earns a violation point (BR-14). An approver of the
// room's type may cancel an approved date, with a reason (BR-04). Any other combination is an invalid
// transition (brief 4.1).
func (s *Service) Cancel(ctx context.Context, actor httpx.Principal, id uuid.UUID, reason, scope string) (Booking, error) {
	var err error
	switch {
	case isManager(actor):
		reason, err = requireReason(reason)
	case isRequester(actor):
		reason, err = optionalReason(reason)
	default:
		return Booking{}, httpx.ErrForbidden
	}
	if err != nil {
		return Booking{}, err
	}
	switch scope {
	case "":
		scope = ScopeOccurrence
	case ScopeOccurrence, ScopeGroup:
	default:
		return Booking{}, httpx.ValidationError(httpx.ErrorDetail{Field: "scope", Message: "scope harus OCCURRENCE atau GROUP."})
	}

	peek, err := s.peek(ctx, actor, id)
	if err != nil {
		return Booking{}, err
	}

	var result Booking
	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		t, err := s.lockTarget(ctx, q, id, peek)
		if err != nil {
			return err
		}
		asManager := isManager(actor)
		if asManager {
			if err := s.types.RequireApprover(ctx, actor, string(t.room.Type)); err != nil {
				return err
			}
		}

		candidates := []dbgen.Booking{t.addressed}
		if scope == ScopeGroup {
			candidates = t.rows
		}
		var cancel []dbgen.Booking
		for _, r := range candidates {
			if cancellable(asManager, r, now) {
				cancel = append(cancel, r)
			}
		}
		if len(cancel) == 0 {
			return invalidState(cannotCancelMessage(asManager, t.addressed, now))
		}

		for _, r := range cancel {
			if err := s.writeDecision(ctx, q, actor, r, dbgen.BookingStatusDIBATALKAN, reason, now); err != nil {
				return err
			}
			// Cancelling an approved booking close to its start wastes the slot; the owner pays a point.
			if !asManager && r.Status == dbgen.BookingStatusDISETUJUI && r.StartAt.Sub(now) < s.settings.LateCancel {
				if err := violations.Record(ctx, q, s.notifier, s.settings.Violations, r.RequesterID, r.ID,
					dbgen.ViolationKindBATALTERLAMBAT, s.settings.PointsLateCancel, now); err != nil {
					return err
				}
			}
		}

		view, err := q.GetBooking(ctx, id)
		if err != nil {
			return fmt.Errorf("reading updated booking: %w", err)
		}
		result = fromRow(view)
		if asManager {
			return s.notifyDecided(ctx, q, result, len(cancel), actor.UserID, reason, now)
		}
		return nil
	})
	if err != nil {
		return Booking{}, s.mapWriteError(err, "updating booking")
	}
	return result, nil
}

// cancellable says whether a date may be cancelled by this kind of actor right now.
func cancellable(asManager bool, r dbgen.Booking, now time.Time) bool {
	if asManager {
		return r.Status == dbgen.BookingStatusDISETUJUI
	}
	return (r.Status == dbgen.BookingStatusMENUNGGU || r.Status == dbgen.BookingStatusDISETUJUI) && now.Before(r.StartAt)
}

func cannotCancelMessage(asManager bool, r dbgen.Booking, now time.Time) string {
	switch {
	case asManager && r.Status == dbgen.BookingStatusMENUNGGU:
		return "Pengelola hanya dapat membatalkan booking yang sudah disetujui; tolak pengajuan yang masih menunggu."
	case asManager:
		return fmt.Sprintf("Booking berstatus %s tidak dapat dibatalkan.", r.Status)
	case (r.Status == dbgen.BookingStatusMENUNGGU || r.Status == dbgen.BookingStatusDISETUJUI) && !now.Before(r.StartAt):
		return "Booking sudah dimulai dan tidak dapat dibatalkan."
	default:
		return fmt.Sprintf("Booking berstatus %s tidak dapat dibatalkan.", r.Status)
	}
}
