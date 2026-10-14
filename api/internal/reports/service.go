package reports

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	maxDescriptionRunes = 2000
	maxNoteRunes        = 500
)

// statusOrder is the lifecycle of BR-08. A report only ever moves one step forward.
var statusOrder = []dbgen.ReportStatus{
	dbgen.ReportStatusBARU, dbgen.ReportStatusDITERIMA, dbgen.ReportStatusDIPROSES, dbgen.ReportStatusSELESAI,
}

type Service struct {
	pool          *db.Pool
	q             *dbgen.Queries
	clock         clock.Clock
	store         *Storage
	maxPhotoBytes int64
	log           *slog.Logger
}

func NewService(pool *db.Pool, clk clock.Clock, store *Storage, maxPhotoBytes int64, log *slog.Logger) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), clock: clk, store: store, maxPhotoBytes: maxPhotoBytes, log: log}
}

type CreateInput struct {
	RoomID      uuid.UUID
	Description string
	Photo       []byte // nil when no photo was sent
}

type ListFilter struct {
	Status  *string
	RoomID  *uuid.UUID
	Page    int
	PerPage int
}

// canViewAll covers the roles that see every report (brief section 6.2).
func canViewAll(actor httpx.Principal) bool {
	return actor.HasRole(httpx.RoleTUKalab, httpx.RoleKaprodi)
}

// visibleTo: handlers see everything, a reporter sees their own, and anyone else gets 404 so that
// another person's report is indistinguishable from a missing one.
func visibleTo(actor httpx.Principal, reporterID uuid.UUID) error {
	if canViewAll(actor) || actor.UserID == reporterID {
		return nil
	}
	return httpx.ErrNotFound
}

// Create files a report. The photo is checked by its magic bytes and stored under a name we generate.
// The file is written last inside the transaction, so a failed write rolls the report back, and it is
// removed again if anything fails afterwards (including the commit).
func (s *Service) Create(ctx context.Context, actor httpx.Principal, in CreateInput) (Report, error) {
	in.Description = strings.TrimSpace(in.Description)
	var problems []httpx.ErrorDetail
	if in.RoomID == uuid.Nil {
		problems = append(problems, httpx.ErrorDetail{Field: "room_id", Message: "room_id wajib diisi."})
	}
	if n := utf8.RuneCountInString(in.Description); n < 1 || n > maxDescriptionRunes {
		problems = append(problems, httpx.ErrorDetail{Field: "description", Message: fmt.Sprintf("Deskripsi wajib diisi (maksimal %d karakter).", maxDescriptionRunes)})
	}

	var photoName string
	if in.Photo != nil {
		switch kind, ok := sniffImage(in.Photo); {
		case int64(len(in.Photo)) > s.maxPhotoBytes:
			problems = append(problems, httpx.ErrorDetail{Field: "photo", Message: fmt.Sprintf("Ukuran foto maksimal %d MB.", s.maxPhotoBytes>>20)})
		case !ok:
			problems = append(problems, httpx.ErrorDetail{Field: "photo", Message: "Foto harus berformat JPG, PNG, atau WebP."})
		default:
			photoName = uuid.NewString() + kind.ext
		}
	}
	if len(problems) > 0 {
		return Report{}, httpx.ValidationError(problems...)
	}

	var created Report
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		if _, err := q.GetRoom(ctx, in.RoomID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.ValidationError(httpx.ErrorDetail{Field: "room_id", Message: "Ruang tidak ditemukan."})
			}
			return fmt.Errorf("reading room: %w", err)
		}

		row, err := q.CreateReport(ctx, dbgen.CreateReportParams{
			RoomID: in.RoomID, ReporterID: actor.UserID, Description: in.Description, CreatedAt: now,
			PhotoPath: pgtype.Text{String: photoName, Valid: photoName != ""},
		})
		if err != nil {
			return fmt.Errorf("creating report: %w", err)
		}
		if err := q.CreateReportEvent(ctx, dbgen.CreateReportEventParams{
			ReportID: row.ID, ActorID: actor.UserID, ToStatus: dbgen.ReportStatusBARU, CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("recording report event: %w", err)
		}

		view, err := q.GetReport(ctx, row.ID)
		if err != nil {
			return fmt.Errorf("reading created report: %w", err)
		}
		created = fromRow(view)
		if err := notifyCreated(ctx, q, created, now); err != nil {
			return err
		}

		if photoName != "" {
			return s.store.Save(photoName, in.Photo)
		}
		return nil
	})
	if err != nil {
		s.discardPhoto(ctx, photoName)
		return Report{}, mapError(err)
	}
	return created, nil
}

// discardPhoto removes a stored file after a failed create. A leftover file is a leak, not a
// correctness problem, so a failure to remove it is logged rather than returned.
func (s *Service) discardPhoto(ctx context.Context, name string) {
	if name == "" {
		return
	}
	if err := s.store.Remove(name); err != nil {
		s.log.ErrorContext(ctx, "removing photo of a failed report", slog.String("file", name), slog.String("error", err.Error()))
	}
}

func (s *Service) List(ctx context.Context, actor httpx.Principal, f ListFilter) ([]Report, int64, error) {
	var reporter pgtype.UUID
	if !canViewAll(actor) {
		reporter = pgtype.UUID{Bytes: actor.UserID, Valid: true}
	}
	var status dbgen.NullReportStatus
	if f.Status != nil {
		status = dbgen.NullReportStatus{ReportStatus: dbgen.ReportStatus(*f.Status), Valid: true}
	}
	var room pgtype.UUID
	if f.RoomID != nil {
		room = pgtype.UUID{Bytes: *f.RoomID, Valid: true}
	}

	rows, err := s.q.ListReports(ctx, dbgen.ListReportsParams{
		ReporterID: reporter, Status: status, RoomID: room,
		PageLimit: int32(f.PerPage), PageOffset: int32((f.Page - 1) * f.PerPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing reports: %w", err)
	}
	total, err := s.q.CountReports(ctx, dbgen.CountReportsParams{ReporterID: reporter, Status: status, RoomID: room})
	if err != nil {
		return nil, 0, fmt.Errorf("counting reports: %w", err)
	}

	out := make([]Report, len(rows))
	for i, r := range rows {
		out[i] = fromRow(dbgen.GetReportRow(r))
	}
	return out, total, nil
}

func (s *Service) Get(ctx context.Context, actor httpx.Principal, id uuid.UUID) (Detail, error) {
	row, err := s.load(ctx, actor, id)
	if err != nil {
		return Detail{}, err
	}
	events, err := s.q.ListReportEvents(ctx, id)
	if err != nil {
		return Detail{}, fmt.Errorf("listing report events: %w", err)
	}
	d := Detail{Report: fromRow(row), Events: make([]Event, len(events))}
	for i, e := range events {
		d.Events[i] = eventFromRow(e)
	}
	return d, nil
}

func (s *Service) load(ctx context.Context, actor httpx.Principal, id uuid.UUID) (dbgen.GetReportRow, error) {
	row, err := s.q.GetReport(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.GetReportRow{}, httpx.ErrNotFound
	}
	if err != nil {
		return dbgen.GetReportRow{}, fmt.Errorf("reading report: %w", err)
	}
	if err := visibleTo(actor, row.ReporterID); err != nil {
		return dbgen.GetReportRow{}, err
	}
	return row, nil
}

// Photo is an open stored photo and what is needed to serve it. The caller closes File.
type Photo struct {
	File        *os.File
	ContentType string
	ModTime     time.Time
}

func (s *Service) Photo(ctx context.Context, actor httpx.Principal, id uuid.UUID) (Photo, error) {
	row, err := s.load(ctx, actor, id)
	if err != nil {
		return Photo{}, err
	}
	if !row.PhotoPath.Valid {
		return Photo{}, httpx.ErrNotFound
	}
	f, err := s.store.Open(row.PhotoPath.String)
	if err != nil {
		// A missing or malformed file is an operational problem, but the client only needs to know
		// there is nothing to show; the details go to the log.
		s.log.ErrorContext(ctx, "opening report photo", slog.String("report_id", id.String()), slog.String("error", err.Error()))
		return Photo{}, httpx.ErrNotFound
	}
	info, err := f.Stat()
	if err != nil {
		closeErr := f.Close()
		return Photo{}, errors.Join(fmt.Errorf("reading photo file info: %w", err), closeErr)
	}
	return Photo{File: f, ContentType: imageForName(row.PhotoPath.String).contentType, ModTime: info.ModTime()}, nil
}

// UpdateStatus moves a report exactly one step forward (BR-08), records the event and tells the
// reporter, all in one transaction. The report row is locked first so that two concurrent updates
// cannot both pass the one-step check.
func (s *Service) UpdateStatus(ctx context.Context, actor httpx.Principal, id uuid.UUID, to, note string) (Report, error) {
	if actor.Role != httpx.RoleTUKalab {
		return Report{}, httpx.ErrForbidden
	}

	next, ok := parseStatus(to)
	if !ok {
		return Report{}, httpx.ValidationError(httpx.ErrorDetail{Field: "status", Message: "status harus BARU, DITERIMA, DIPROSES, atau SELESAI."})
	}
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > maxNoteRunes {
		return Report{}, httpx.ValidationError(httpx.ErrorDetail{Field: "note", Message: fmt.Sprintf("Catatan maksimal %d karakter.", maxNoteRunes)})
	}

	var updated Report
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		cur, err := q.LockReport(ctx, id)
		if err != nil {
			return err
		}
		if want := nextStatus(cur.Status); want == nil || *want != next {
			return invalidState(cur.Status, next)
		}

		noteText := pgtype.Text{String: note, Valid: note != ""}
		if err := q.UpdateReportStatus(ctx, dbgen.UpdateReportStatusParams{
			ID: id, Status: next, Now: now, HandledBy: pgtype.UUID{Bytes: actor.UserID, Valid: true},
		}); err != nil {
			return fmt.Errorf("updating report status: %w", err)
		}
		if err := q.CreateReportEvent(ctx, dbgen.CreateReportEventParams{
			ReportID: id, ActorID: actor.UserID, ToStatus: next, CreatedAt: now, Note: noteText,
			FromStatus: dbgen.NullReportStatus{ReportStatus: cur.Status, Valid: true},
		}); err != nil {
			return fmt.Errorf("recording report event: %w", err)
		}

		view, err := q.GetReport(ctx, id)
		if err != nil {
			return fmt.Errorf("reading updated report: %w", err)
		}
		updated = fromRow(view)
		return notifyStatusChanged(ctx, q, updated, actor, note, now)
	})
	if err != nil {
		return Report{}, mapError(err)
	}
	return updated, nil
}

// nextStatus is the single status a report may move to from cur, or nil when it is finished.
func nextStatus(cur dbgen.ReportStatus) *dbgen.ReportStatus {
	for i, s := range statusOrder {
		if s == cur && i+1 < len(statusOrder) {
			return &statusOrder[i+1]
		}
	}
	return nil
}

func invalidState(from, to dbgen.ReportStatus) error {
	msg := fmt.Sprintf("Status laporan tidak dapat diubah dari %s ke %s.", from, to)
	if want := nextStatus(from); want != nil {
		msg += fmt.Sprintf(" Langkah berikutnya adalah %s.", *want)
	} else {
		msg += " Laporan sudah selesai."
	}
	return httpx.NewCustomError(http.StatusConflict, "INVALID_STATE", msg, nil, httpx.ErrInvalidState)
}

func mapError(err error) error {
	var domain *httpx.CustomDomainError
	switch {
	case errors.As(err, &domain):
		return err
	case errors.Is(err, pgx.ErrNoRows):
		return httpx.ErrNotFound
	}
	return fmt.Errorf("report operation: %w", err)
}

func parseStatus(v string) (dbgen.ReportStatus, bool) {
	for _, s := range statusOrder {
		if string(s) == v {
			return s, true
		}
	}
	return "", false
}
