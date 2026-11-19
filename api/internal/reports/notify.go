package reports

import (
	"context"
	"fmt"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/notifications"
	"classlab/api/internal/platform/db/dbgen"
)

const excerptRunes = 80

// The notify helpers take the transaction's *dbgen.Queries on purpose: a notification must commit or
// roll back together with the change that caused it (brief section 5, "Tipe notifikasi").

// notifyCreated tells every active approver of the room's type about a new report, except the reporter if they are one.
func (s *Service) notifyCreated(ctx context.Context, q *dbgen.Queries, r Report, now time.Time) error {
	return s.notifier.ToApprovers(ctx, q, r.Room.Type, notifications.Message{
		Type:    notifications.TypeReportCreated,
		Title:   "Laporan kerusakan baru",
		Body:    fmt.Sprintf("%s melaporkan kerusakan di %s: %s", r.Reporter.Name, r.Room.Code, excerpt(r.Description)),
		RefType: notifications.RefReport, RefID: r.ID,
	}, r.ReporterID, now)
}

// notifyStatusChanged tells the reporter about progress. A reporter who changed the status
// themselves (a TU/Kalab handling their own report) is not told about their own action.
func (s *Service) notifyStatusChanged(ctx context.Context, q *dbgen.Queries, r Report, actor httpx.Principal, note string, now time.Time) error {
	if actor.UserID == r.ReporterID {
		return nil
	}
	body := fmt.Sprintf("Laporan kerusakan di %s kini berstatus %s.", r.Room.Code, r.Status)
	if note != "" {
		body += " Catatan: " + note
	}
	return s.notifier.ToUser(ctx, q, r.ReporterID, notifications.Message{
		Type: notifications.TypeReportStatusChanged, Title: "Status laporan diperbarui", Body: body,
		RefType: notifications.RefReport, RefID: r.ID,
	}, now)
}

// excerpt shortens a description for a notification, cutting on a character boundary.
func excerpt(s string) string {
	runes := []rune(s)
	if len(runes) <= excerptRunes {
		return s
	}
	return string(runes[:excerptRunes]) + "…"
}
