package bookings

import (
	"context"
	"fmt"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	typeCreated    = "BOOKING_CREATED"
	typeApproved   = "BOOKING_APPROVED"
	typeRejected   = "BOOKING_REJECTED"
	typeCancelled  = "BOOKING_CANCELLED"
	refTypeBooking = "BOOKING"
)

// The notify helpers take the transaction's *dbgen.Queries on purpose: a notification must commit or
// roll back together with the status change that caused it (brief section 5, "Tipe notifikasi").

// notifyCreated tells every active approver of the room's type that a request awaits a decision.
func notifyCreated(ctx context.Context, q *dbgen.Queries, b Booking, now time.Time) error {
	err := q.CreateNotificationForApprovers(ctx, dbgen.CreateNotificationForApproversParams{
		RoomType:  dbgen.RoomType(b.Room.Type),
		Type:      typeCreated,
		Title:     "Pengajuan booking baru",
		Body:      fmt.Sprintf("%s mengajukan %s pada %s.", b.Requester.Name, b.Room.Code, slotText(b)),
		RefType:   pgtype.Text{String: refTypeBooking, Valid: true},
		RefID:     pgtype.UUID{Bytes: b.ID, Valid: true},
		CreatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("notifying approvers: %w", err)
	}
	return nil
}

// notifyDecided tells the borrower about the outcome. When the borrower made the change themselves
// (cancelling their own request) there is nobody to inform.
func notifyDecided(ctx context.Context, q *dbgen.Queries, b Booking, actor httpx.Principal, reason string, now time.Time) error {
	if actor.UserID == b.RequesterID {
		return nil
	}

	var typ, title, body string
	switch dbgen.BookingStatus(b.Status) {
	case dbgen.BookingStatusDISETUJUI:
		typ, title = typeApproved, "Booking disetujui"
		body = fmt.Sprintf("Pengajuan %s pada %s telah disetujui.", b.Room.Code, slotText(b))
	case dbgen.BookingStatusDITOLAK:
		typ, title = typeRejected, "Booking ditolak"
		body = fmt.Sprintf("Pengajuan %s pada %s ditolak. Alasan: %s", b.Room.Code, slotText(b), reason)
	case dbgen.BookingStatusDIBATALKAN:
		typ, title = typeCancelled, "Booking dibatalkan"
		body = fmt.Sprintf("Booking %s pada %s dibatalkan oleh pengelola. Alasan: %s", b.Room.Code, slotText(b), reason)
	default:
		return fmt.Errorf("no notification for status %s", b.Status)
	}

	err := q.CreateNotification(ctx, dbgen.CreateNotificationParams{
		UserID: b.RequesterID, Type: typ, Title: title, Body: body,
		RefType:   pgtype.Text{String: refTypeBooking, Valid: true},
		RefID:     pgtype.UUID{Bytes: b.ID, Valid: true},
		CreatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("notifying requester: %w", err)
	}
	return nil
}

// slotText renders a booking's slot for people, e.g. "20/10/2026 13:00-15:00 WIB".
func slotText(b Booking) string {
	return fmt.Sprintf("%s-%s WIB", clock.ToWIB(b.StartAt).Format("02/01/2006 15:04"), clock.ToWIB(b.EndAt).Format("15:04"))
}
