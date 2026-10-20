package bookings

import (
	"context"
	"fmt"
	"time"

	"classlab/api/internal/notifications"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
)

// The notify helpers take the transaction's *dbgen.Queries on purpose: a notification must commit or
// roll back together with the status change that caused it (brief section 5, "Tipe notifikasi").
// A multi-date request produces one notification per action, not one per date (BR-27).

// notifyCreated tells every active approver of the room's type that a request awaits a decision. It is
// only called for requests that need a person; an automatically approved one tells nobody (BR-17).
func (s *Service) notifyCreated(ctx context.Context, q *dbgen.Queries, b Booking, dates int, now time.Time) error {
	return s.notifier.ToApprovers(ctx, q, b.Room.Type, notifications.Message{
		Type:    notifications.TypeBookingCreated,
		Title:   "Pengajuan booking baru",
		Body:    fmt.Sprintf("%s mengajukan %s %s.", b.Requester.Name, b.Room.Code, whenText(b, dates)),
		RefType: notifications.RefBooking, RefID: b.ID,
	}, uuid.Nil, now)
}

// notifyEdited tells the approvers that a request they may have been looking at changed and waits again (BR-26).
func (s *Service) notifyEdited(ctx context.Context, q *dbgen.Queries, b Booking, now time.Time) error {
	return s.notifier.ToApprovers(ctx, q, b.Room.Type, notifications.Message{
		Type:    notifications.TypeBookingEdited,
		Title:   "Pengajuan booking diubah",
		Body:    fmt.Sprintf("%s mengubah pengajuan %s pada %s dan menunggu persetujuan.", b.Requester.Name, b.Room.Code, slotText(b)),
		RefType: notifications.RefBooking, RefID: b.ID,
	}, uuid.Nil, now)
}

// notifyDecided tells the requester about the outcome. When the requester made the change themselves
// (cancelling their own request) there is nobody to inform.
func (s *Service) notifyDecided(ctx context.Context, q *dbgen.Queries, b Booking, dates int, actorID uuid.UUID, reason string, now time.Time) error {
	if actorID == b.RequesterID {
		return nil
	}

	var typ, title, body string
	switch dbgen.BookingStatus(b.Status) {
	case dbgen.BookingStatusDISETUJUI:
		typ, title = notifications.TypeBookingApproved, "Booking disetujui"
		body = fmt.Sprintf("Pengajuan %s %s telah disetujui.", b.Room.Code, whenText(b, dates))
	case dbgen.BookingStatusDITOLAK:
		typ, title = notifications.TypeBookingRejected, "Booking ditolak"
		body = fmt.Sprintf("Pengajuan %s %s ditolak. Alasan: %s", b.Room.Code, whenText(b, dates), reason)
	case dbgen.BookingStatusDIBATALKAN:
		typ, title = notifications.TypeBookingCancelled, "Booking dibatalkan"
		body = fmt.Sprintf("Booking %s %s dibatalkan oleh pengelola. Alasan: %s", b.Room.Code, whenText(b, dates), reason)
	default:
		return fmt.Errorf("no notification for status %s", b.Status)
	}

	return s.notifier.ToUser(ctx, q, b.RequesterID, notifications.Message{
		Type: typ, Title: title, Body: body, RefType: notifications.RefBooking, RefID: b.ID,
	}, now)
}

// slotText renders a booking's slot for people, e.g. "20/10/2026 13:00-15:00 WIB".
func slotText(b Booking) string {
	return fmt.Sprintf("%s-%s WIB", clock.ToWIB(b.StartAt).Format("02/01/2006 15:04"), clock.ToWIB(b.EndAt).Format("15:04"))
}

// whenText is "pada <slot>" for one date and "untuk N tanggal mulai <slot>" for a group.
func whenText(b Booking, dates int) string {
	if dates > 1 {
		return fmt.Sprintf("untuk %d tanggal mulai %s", dates, slotText(b))
	}
	return "pada " + slotText(b)
}
