package bookings

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"classlab/api/internal/attachments"
	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// maxCheckoutNoteRunes is the report description limit: a problem noted at check-out becomes the report text.
const maxCheckoutNoteRunes = 2000

// PreviousCheckout is how the previous user left the room (BR-11), a picture of the room's starting state.
type PreviousCheckout struct {
	CheckedOutAt time.Time `json:"checked_out_at"`
	Condition    string    `json:"condition"`
	AttachmentID uuid.UUID `json:"attachment_id"`
}

type CheckInResult struct {
	Booking          Booking           `json:"booking"`
	PreviousCheckout *PreviousCheckout `json:"previous_checkout"`
}

type CheckInInput struct {
	RoomCode string
	QRToken  string
}

func qrInvalid() error {
	return httpx.NewCustomError(http.StatusUnprocessableEntity, "QR_INVALID",
		"QR ruang tidak valid atau sudah diganti. Pindai ulang QR yang terpasang di pintu.", nil, httpx.ErrValidation)
}

// noEligibleBooking is the 409 of BR-11; when the user has a later booking in the room, details say when
// its check-in window opens and closes.
func noEligibleBooking(opens, closes *time.Time) error {
	var details []httpx.ErrorDetail
	if opens != nil && closes != nil {
		details = []httpx.ErrorDetail{
			{Field: "next_window.opens_at", Message: clock.ToWIB(*opens).Format(time.RFC3339)},
			{Field: "next_window.closes_at", Message: clock.ToWIB(*closes).Format(time.RFC3339)},
		}
	}
	return httpx.NewCustomError(http.StatusConflict, "NO_ELIGIBLE_BOOKING",
		"Tidak ada booking disetujui milik Anda di ruang ini pada jendela check-in.", details, httpx.ErrInvalidState)
}

// CheckIn starts the use of a room (BR-11). The QR token is compared in constant time, the window is
// judged from the server clock, and the booking row is locked before its status changes so that a
// cancellation or the no-show job cannot interleave (brief 4.4).
func (s *Service) CheckIn(ctx context.Context, actor httpx.Principal, in CheckInInput) (CheckInResult, error) {
	if !isRequester(actor) {
		return CheckInResult{}, httpx.ErrForbidden
	}
	in.RoomCode, in.QRToken = strings.TrimSpace(in.RoomCode), strings.TrimSpace(in.QRToken)
	var problems []httpx.ErrorDetail
	if in.RoomCode == "" {
		problems = append(problems, httpx.ErrorDetail{Field: "room_code", Message: "room_code wajib diisi."})
	}
	if in.QRToken == "" {
		problems = append(problems, httpx.ErrorDetail{Field: "qr_token", Message: "qr_token wajib diisi."})
	}
	if len(problems) > 0 {
		return CheckInResult{}, httpx.ValidationError(problems...)
	}

	var result CheckInResult
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		// An unknown room and a wrong token answer the same, so the response does not tell which rooms exist.
		room, err := q.GetRoomForCheckin(ctx, in.RoomCode)
		known := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("reading room: %w", err)
		}
		stored := room.QrToken
		if subtle.ConstantTimeCompare([]byte(stored), []byte(in.QRToken)) != 1 || !known {
			return qrInvalid()
		}

		candidates, err := q.LockCheckinCandidates(ctx, dbgen.LockCheckinCandidatesParams{
			RequesterID: actor.UserID, RoomID: room.ID,
			WindowFrom: now.Add(-s.settings.CheckinLate), WindowTo: now.Add(s.settings.CheckinEarly),
		})
		if err != nil {
			return fmt.Errorf("finding booking to check in: %w", err)
		}
		if len(candidates) == 0 {
			next, err := q.NextCheckinWindowStart(ctx, dbgen.NextCheckinWindowStartParams{
				RequesterID: actor.UserID, RoomID: room.ID, StartAt: now.Add(s.settings.CheckinEarly),
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return noEligibleBooking(nil, nil)
			}
			if err != nil {
				return fmt.Errorf("finding next check-in window: %w", err)
			}
			opens, closes := next.Add(-s.settings.CheckinEarly), next.Add(s.settings.CheckinLate)
			return noEligibleBooking(&opens, &closes)
		}
		target := candidates[0] // the earliest start when back-to-back bookings overlap the window

		if _, err := q.CheckInBooking(ctx, dbgen.CheckInBookingParams{ID: target.ID, Now: pgtype.Timestamptz{Time: now, Valid: true}}); err != nil {
			return fmt.Errorf("checking in: %w", err)
		}
		if err := q.CreateBookingEvent(ctx, dbgen.CreateBookingEventParams{
			BookingID: target.ID, ActorID: pgtype.UUID{Bytes: actor.UserID, Valid: true},
			FromStatus: dbgen.NullBookingStatus{BookingStatus: dbgen.BookingStatusDISETUJUI, Valid: true},
			ToStatus:   dbgen.BookingStatusBERLANGSUNG, Kind: "STATUS", CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("recording check-in: %w", err)
		}

		view, err := q.GetBooking(ctx, target.ID)
		if err != nil {
			return fmt.Errorf("reading checked-in booking: %w", err)
		}
		result.Booking = fromRow(view)

		prev, err := q.LatestCheckoutOfRoom(ctx, dbgen.LatestCheckoutOfRoomParams{RoomID: room.ID, ID: target.ID})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return fmt.Errorf("reading previous check-out: %w", err)
		default:
			result.PreviousCheckout = &PreviousCheckout{
				CheckedOutAt: clock.ToWIB(prev.CheckedOutAt.Time),
				Condition:    string(prev.CheckoutCondition.CheckoutCondition),
				AttachmentID: uuid.UUID(prev.CheckoutAttachmentID.Bytes),
			}
		}
		return nil
	})
	if err != nil {
		return CheckInResult{}, s.mapWriteError(err, "checking in")
	}
	return result, nil
}

type CheckOutInput struct {
	Condition string
	Note      string
	Photo     []byte // nil when none was sent
}

type CheckOutResult struct {
	Booking  Booking    `json:"booking"`
	ReportID *uuid.UUID `json:"report_id"`
}

// CheckOut ends the use of a room with a photo of how it was left (BR-12). A room left with a problem gets a
// damage report in the same transaction, with the same photo, and its approvers are told; either everything
// commits or nothing does. Leaving early frees the rest of the slot, because SELESAI no longer holds it.
func (s *Service) CheckOut(ctx context.Context, actor httpx.Principal, id uuid.UUID, in CheckOutInput) (CheckOutResult, error) {
	if !isRequester(actor) {
		return CheckOutResult{}, httpx.ErrForbidden
	}

	in.Note = strings.TrimSpace(in.Note)
	var problems []httpx.ErrorDetail
	condition := dbgen.CheckoutCondition(in.Condition)
	if !condition.Valid() {
		problems = append(problems, httpx.ErrorDetail{Field: "condition", Message: "condition harus NORMAL atau ADA_MASALAH."})
	}
	if condition == dbgen.CheckoutConditionADAMASALAH && in.Note == "" {
		problems = append(problems, httpx.ErrorDetail{Field: "note", Message: "note wajib diisi bila kondisi ADA_MASALAH."})
	}
	if utf8.RuneCountInString(in.Note) > maxCheckoutNoteRunes {
		problems = append(problems, httpx.ErrorDetail{Field: "note", Message: fmt.Sprintf("note maksimal %d karakter.", maxCheckoutNoteRunes)})
	}
	if in.Photo == nil {
		problems = append(problems, httpx.ErrorDetail{Field: "photo", Message: "photo wajib diunggah."})
	}
	// The photo is checked and re-encoded before the transaction opens (BR-16): CPU work stays out of it.
	var prepared attachments.Prepared
	if in.Photo != nil {
		p, err := s.files.PreparePhoto(in.Photo, "photo")
		var domain *httpx.CustomDomainError
		switch {
		case errors.As(err, &domain):
			problems = append(problems, domain.Details...)
		case err != nil:
			return CheckOutResult{}, err
		default:
			prepared = p
		}
	}
	if len(problems) > 0 {
		return CheckOutResult{}, httpx.ValidationError(problems...)
	}

	var result CheckOutResult
	var stored attachments.Stored
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		cur, err := q.LockBooking(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && cur.RequesterID != actor.UserID) {
			return httpx.ErrNotFound // someone else's booking is indistinguishable from a missing one
		}
		if err != nil {
			return fmt.Errorf("locking booking: %w", err)
		}
		if cur.Status != dbgen.BookingStatusBERLANGSUNG {
			return invalidState(fmt.Sprintf("Booking berstatus %s tidak dapat di-check-out.", cur.Status))
		}

		st, err := s.files.Insert(ctx, q, prepared, actor.UserID, "", now)
		if err != nil {
			return err
		}
		stored = st
		attachment := pgtype.UUID{Bytes: st.ID, Valid: true}

		if _, err := q.CheckOutBooking(ctx, dbgen.CheckOutBookingParams{
			ID: id, Now: pgtype.Timestamptz{Time: now, Valid: true}, Condition: condition,
			Note:         pgtype.Text{String: in.Note, Valid: in.Note != ""},
			AttachmentID: attachment,
		}); err != nil {
			return fmt.Errorf("checking out: %w", err)
		}
		if err := q.CreateBookingEvent(ctx, dbgen.CreateBookingEventParams{
			BookingID: id, ActorID: pgtype.UUID{Bytes: actor.UserID, Valid: true},
			FromStatus: dbgen.NullBookingStatus{BookingStatus: dbgen.BookingStatusBERLANGSUNG, Valid: true},
			ToStatus:   dbgen.BookingStatusSELESAI, Kind: "STATUS", CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("recording check-out: %w", err)
		}

		if condition == dbgen.CheckoutConditionADAMASALAH {
			report, err := s.reports.FileIn(ctx, q, actor.UserID, cur.RoomID, in.Note, attachment, now)
			if err != nil {
				return err
			}
			result.ReportID = &report.ID
		}

		view, err := q.GetBooking(ctx, id)
		if err != nil {
			return fmt.Errorf("reading checked-out booking: %w", err)
		}
		result.Booking = fromRow(view)

		// The file is written last, so a failed write rolls everything above back.
		return stored.Write()
	})
	if err != nil {
		if stored.ID != uuid.Nil {
			stored.Remove(ctx) // the commit may be what failed
		}
		return CheckOutResult{}, s.mapWriteError(err, "checking out")
	}
	return result, nil
}

// Checkout is one row of the check-out feed that OB and the managers read to see how rooms were left.
type Checkout struct {
	BookingID    uuid.UUID `json:"booking_id"`
	RoomID       uuid.UUID `json:"room_id"`
	RoomCode     string    `json:"room_code"`
	RoomName     string    `json:"room_name"`
	RoomType     string    `json:"room_type"`
	CheckedOutAt time.Time `json:"checked_out_at"`
	Condition    string    `json:"condition"`
	Note         *string   `json:"note"`
	AttachmentID uuid.UUID `json:"attachment_id"`
}

type CheckoutFilter struct {
	RoomID  *uuid.UUID
	From    *time.Time
	To      *time.Time
	Page    int
	PerPage int
}

// Checkouts lists the latest check-outs. OB and ADMIN see every room; TU and KALAB the room types they approve.
func (s *Service) Checkouts(ctx context.Context, actor httpx.Principal, f CheckoutFilter) ([]Checkout, int64, error) {
	var roomTypes []string
	switch {
	case actor.Role == httpx.RoleAdmin || actor.Role == httpx.RoleOB:
	case isManager(actor):
		types, err := s.types.ApprovableTypes(ctx, actor.Role)
		if err != nil {
			return nil, 0, err
		}
		roomTypes = types // non-nil even when empty: an empty array matches nothing, NULL would match all
	default:
		return nil, 0, httpx.ErrForbidden
	}

	var room pgtype.UUID
	if f.RoomID != nil {
		room = pgtype.UUID{Bytes: *f.RoomID, Valid: true}
	}
	var from, to pgtype.Timestamptz
	if f.From != nil {
		from = pgtype.Timestamptz{Time: f.From.UTC(), Valid: true}
	}
	if f.To != nil {
		to = pgtype.Timestamptz{Time: f.To.UTC(), Valid: true}
	}

	rows, err := s.q.ListCheckouts(ctx, dbgen.ListCheckoutsParams{
		RoomID: room, RangeStart: from, RangeEnd: to, RoomTypes: roomTypes,
		PageLimit: int32(f.PerPage), PageOffset: int32((f.Page - 1) * f.PerPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing check-outs: %w", err)
	}
	total, err := s.q.CountCheckouts(ctx, dbgen.CountCheckoutsParams{RoomID: room, RangeStart: from, RangeEnd: to, RoomTypes: roomTypes})
	if err != nil {
		return nil, 0, fmt.Errorf("counting check-outs: %w", err)
	}

	out := make([]Checkout, len(rows))
	for i, r := range rows {
		out[i] = Checkout{
			BookingID: r.BookingID, RoomID: r.RoomID, RoomCode: r.RoomCode, RoomName: r.RoomName, RoomType: string(r.RoomType),
			CheckedOutAt: clock.ToWIB(r.CheckedOutAt.Time), Condition: string(r.CheckoutCondition.CheckoutCondition),
			AttachmentID: uuid.UUID(r.CheckoutAttachmentID.Bytes),
		}
		if r.CheckoutNote.Valid {
			out[i].Note = &r.CheckoutNote.String
		}
	}
	return out, total, nil
}
