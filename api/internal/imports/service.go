package imports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/notifications"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/platform/password"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const pgUniqueViolation = "23505"

// Options are the settings the service reads from configuration.
type Options struct {
	Limits             Limits
	TempPasswordLength int
	MaxWeeks           int // meetings one schedule line may expand to (IMPORT_MAX_WEEKS)
}

type Service struct {
	pool   *db.Pool
	q      *dbgen.Queries
	hasher *password.Hasher
	clock  clock.Clock
	opts   Options
	log    *slog.Logger

	notifier *notifications.Notifier
}

func NewService(pool *db.Pool, hasher *password.Hasher, clk clock.Clock, notifier *notifications.Notifier, opts Options, log *slog.Logger) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), hasher: hasher, clock: clk, opts: opts, log: log, notifier: notifier}
}

// Limits returns the upload limits so the HTTP layer can bound the request body.
func (s *Service) Limits() Limits { return s.opts.Limits }

var errPlanChanged = httpx.NewCustomError(http.StatusConflict, "IMPORT_PLAN_CHANGED",
	"Rencana impor berubah sejak pratinjau. Lakukan pratinjau ulang sebelum menerapkan.", nil, httpx.ErrInvalidState)

var errAlreadyApplied = httpx.NewCustomError(http.StatusConflict, "IMPORT_ALREADY_APPLIED",
	"File ini sudah pernah diterapkan.", nil, httpx.ErrInvalidState)

// invalidImport is the 422 for a file with problems; details name each line and column.
func invalidImport(problems []RowError) error {
	details := make([]httpx.ErrorDetail, len(problems))
	for i, p := range problems {
		details[i] = httpx.ErrorDetail{Line: p.Line, Column: p.Column, Message: p.Message}
	}
	return httpx.NewCustomError(http.StatusUnprocessableEntity, "IMPORT_INVALID",
		"File impor memuat data yang tidak valid; tidak ada yang disimpan.", details, httpx.ErrValidation)
}

// fileError turns a parse failure into the API error, passing other errors through.
func fileError(err error) error {
	var fe *FileError
	if errors.As(err, &fe) {
		return invalidImport(fe.Problems)
	}
	return err
}

// isAlreadyAppliedRace recognises the unique index that keeps one COMMITTED import per file hash. It
// fires when two applies of the same file race past the check in front of it.
func isAlreadyAppliedRace(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "imports_kind_hash_uniq"
}

// Import is the API representation of one row of the import history.
type Import struct {
	ID           uuid.UUID       `json:"id"`
	Kind         string          `json:"kind"`
	UploadedBy   Uploader        `json:"uploaded_by"`
	FileName     *string         `json:"file_name"`
	Semester     *string         `json:"semester"`
	Status       string          `json:"status"`
	Summary      json.RawMessage `json:"summary"`
	CreatedAt    time.Time       `json:"created_at"`
	RolledBackAt *time.Time      `json:"rolled_back_at"`
}

type Uploader struct {
	ID         uuid.UUID `json:"id"`
	Identifier string    `json:"identifier"`
	Name       string    `json:"name"`
}

type ListFilter struct {
	Kind    string // optional: SCHEDULES or USERS
	Page    int
	PerPage int
}

func mayImport(actor httpx.Principal) bool {
	return actor.HasRole(httpx.RoleTU, httpx.RoleAdmin)
}

// List returns the import history, newest first.
func (s *Service) List(ctx context.Context, actor httpx.Principal, f ListFilter) ([]Import, int64, error) {
	if !mayImport(actor) {
		return nil, 0, httpx.ErrForbidden
	}
	var kind dbgen.NullImportKind
	if f.Kind != "" {
		k := dbgen.ImportKind(f.Kind)
		if !k.Valid() {
			return nil, 0, httpx.ValidationError(httpx.ErrorDetail{Field: "kind", Message: "kind harus SCHEDULES atau USERS."})
		}
		kind = dbgen.NullImportKind{ImportKind: k, Valid: true}
	}

	total, err := s.q.CountImports(ctx, kind)
	if err != nil {
		return nil, 0, fmt.Errorf("counting imports: %w", err)
	}
	rows, err := s.q.ListImports(ctx, dbgen.ListImportsParams{
		Kind: kind, PageLimit: int32(f.PerPage), PageOffset: int32((f.Page - 1) * f.PerPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing imports: %w", err)
	}

	out := make([]Import, len(rows))
	for i, r := range rows {
		out[i] = Import{
			ID: r.ID, Kind: string(r.Kind), Status: r.Status, Summary: r.Summary,
			UploadedBy: Uploader{ID: r.UploadedBy, Identifier: r.UploaderIdentifier, Name: r.UploaderName},
			CreatedAt:  clock.ToWIB(r.CreatedAt),
		}
		if r.FileName.Valid {
			out[i].FileName = &r.FileName.String
		}
		if r.Semester.Valid {
			out[i].Semester = &r.Semester.String
		}
		if r.RolledBackAt.Valid {
			t := clock.ToWIB(r.RolledBackAt.Time)
			out[i].RolledBackAt = &t
		}
	}
	return out, total, nil
}

func optionalText(v string) pgtype.Text { return pgtype.Text{String: v, Valid: v != ""} }
