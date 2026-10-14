package reports

import (
	"context"
	"fmt"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/jackc/pgx/v5/pgtype"
)

const (
	typeCreated       = "REPORT_CREATED"
	typeStatusChanged = "REPORT_STATUS_CHANGED"
	refTypeReport     = "REPORT"

	excerptRunes = 80
)

// The notify helpers take the transaction's *dbgen.Queries on purpose: a notification must commit or
// roll back together with the change that caused it (brief section 5, "Tipe notifikasi").

// notifyCreated tells every active TU/Kalab about a new report, except the reporter if they are one.
func notifyCreated(ctx context.Context, q *dbgen.Queries, r Report, now time.Time) error {
	err := q.CreateNotificationForRole(ctx, dbgen.CreateNotificationForRoleParams{
		Role:          dbgen.UserRoleTUKALAB,
		Type:          typeCreated,
		Title:         "Laporan kerusakan baru",
		Body:          fmt.Sprintf("%s melaporkan kerusakan di %s: %s", r.Reporter.Name, r.Room.Code, excerpt(r.Description)),
		RefType:       pgtype.Text{String: refTypeReport, Valid: true},
		RefID:         pgtype.UUID{Bytes: r.ID, Valid: true},
		CreatedAt:     now,
		ExcludeUserID: pgtype.UUID{Bytes: r.ReporterID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("notifying handlers: %w", err)
	}
	return nil
}

// notifyStatusChanged tells the reporter about progress. A reporter who changed the status
// themselves (a TU/Kalab handling their own report) is not told about their own action.
func notifyStatusChanged(ctx context.Context, q *dbgen.Queries, r Report, actor httpx.Principal, note string, now time.Time) error {
	if actor.UserID == r.ReporterID {
		return nil
	}
	body := fmt.Sprintf("Laporan kerusakan di %s kini berstatus %s.", r.Room.Code, r.Status)
	if note != "" {
		body += " Catatan: " + note
	}
	err := q.CreateNotification(ctx, dbgen.CreateNotificationParams{
		UserID: r.ReporterID, Type: typeStatusChanged, Title: "Status laporan diperbarui", Body: body,
		RefType:   pgtype.Text{String: refTypeReport, Valid: true},
		RefID:     pgtype.UUID{Bytes: r.ID, Valid: true},
		CreatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("notifying reporter: %w", err)
	}
	return nil
}

// excerpt shortens a description for a notification, cutting on a character boundary.
func excerpt(s string) string {
	runes := []rune(s)
	if len(runes) <= excerptRunes {
		return s
	}
	return string(runes[:excerptRunes]) + "…"
}
