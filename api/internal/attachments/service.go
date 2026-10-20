package attachments

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/roomtypes"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	maxOriginalNameRunes = 120

	// dirAttachments is where every new file goes, relative to the upload directory.
	dirAttachments = "attachments/"
)

// Limits are the size caps from configuration (MAX_LETTER_MB, MAX_UPLOAD_MB).
type Limits struct {
	MaxLetterBytes int64
	MaxPhotoBytes  int64
}

// Attachment is the API representation of a stored file.
type Attachment struct {
	ID           uuid.UUID `json:"id"`
	Kind         string    `json:"kind"`
	ContentType  string    `json:"content_type"`
	SizeBytes    int       `json:"size_bytes"`
	OriginalName *string   `json:"original_name"`
	CreatedAt    time.Time `json:"created_at"`
}

// Prepared is a checked file ready to be stored. Checking and re-encoding cost CPU, so they happen before
// a transaction opens; Insert and Write then take almost no time inside it.
type Prepared struct {
	kind        dbgen.AttachmentKind
	contentType string
	data        []byte
	rel         string // path relative to the upload directory, with a name we generated
}

// Size is the number of bytes that will be stored.
func (p Prepared) Size() int { return len(p.data) }

// Stored is a row that has been inserted whose file is not yet on disk. Write puts it there, as the last
// step of the transaction, so a failed write rolls the row back; Remove undoes a write when something later
// fails (including the commit).
type Stored struct {
	ID    uuid.UUID
	rel   string
	data  []byte
	store *Storage
	log   *slog.Logger
}

func (st Stored) Write() error { return st.store.Save(st.rel, st.data) }

func (st Stored) Remove(ctx context.Context) {
	if err := st.store.Remove(st.rel); err != nil {
		st.log.ErrorContext(ctx, "removing attachment file of a failed write", slog.String("file", st.rel), slog.String("error", err.Error()))
	}
}

type Service struct {
	pool   *db.Pool
	q      *dbgen.Queries
	store  *Storage
	clock  clock.Clock
	limits Limits
	types  *roomtypes.Store
	log    *slog.Logger
}

func NewService(pool *db.Pool, store *Storage, clk clock.Clock, limits Limits, types *roomtypes.Store, log *slog.Logger) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), store: store, clock: clk, limits: limits, types: types, log: log}
}

// Limits returns the size caps so the HTTP layer can bound the request body.
func (s *Service) Limits() Limits { return s.limits }

var errGone = httpx.NewCustomError(http.StatusGone, "GONE",
	"File ini sudah dihapus karena melewati masa penyimpanan.", nil, httpx.ErrNotFound)

// PrepareLetter checks a letter (BR-19): a PDF is kept exactly as sent and never parsed; an image is
// re-encoded as JPEG. field names the form field in error details.
func (s *Service) PrepareLetter(data []byte, field string) (Prepared, error) {
	if int64(len(data)) > s.limits.MaxLetterBytes {
		return Prepared{}, tooLarge(field, s.limits.MaxLetterBytes)
	}
	if isPDF(data) {
		return Prepared{kind: dbgen.AttachmentKindLETTER, contentType: "application/pdf", data: data, rel: newPath("pdf")}, nil
	}
	return s.prepareImage(dbgen.AttachmentKindLETTER, data, field, "Surat harus berformat PDF, JPG, PNG, atau WebP.")
}

// PreparePhoto checks a photo (BR-16): JPEG, PNG or WebP, decoded and written again as JPEG.
func (s *Service) PreparePhoto(data []byte, field string) (Prepared, error) {
	if int64(len(data)) > s.limits.MaxPhotoBytes {
		return Prepared{}, tooLarge(field, s.limits.MaxPhotoBytes)
	}
	return s.prepareImage(dbgen.AttachmentKindPHOTO, data, field, "Foto harus berformat JPG, PNG, atau WebP.")
}

func (s *Service) prepareImage(kind dbgen.AttachmentKind, data []byte, field, notImage string) (Prepared, error) {
	jpegBytes, err := processImage(data)
	switch {
	case errors.Is(err, ErrNotImage):
		return Prepared{}, httpx.ValidationError(httpx.ErrorDetail{Field: field, Message: notImage})
	case errors.Is(err, ErrImageTooLarge):
		return Prepared{}, httpx.ValidationError(httpx.ErrorDetail{Field: field, Message: "Dimensi gambar terlalu besar."})
	case errors.Is(err, ErrBrokenImage):
		return Prepared{}, httpx.ValidationError(httpx.ErrorDetail{Field: field, Message: "Gambar rusak atau tidak dapat dibaca."})
	case err != nil:
		return Prepared{}, err
	}
	return Prepared{kind: kind, contentType: "image/jpeg", data: jpegBytes, rel: newPath("jpg")}, nil
}

func tooLarge(field string, max int64) error {
	return httpx.ValidationError(httpx.ErrorDetail{Field: field, Message: fmt.Sprintf("Ukuran file maksimal %d MB.", max>>20)})
}

func newPath(ext string) string { return dirAttachments + uuid.NewString() + "." + ext }

// Insert records the file in the caller's transaction and returns the handle that writes it. The server
// time is used for created_at: a client-supplied time or the file's own metadata is never trusted.
func (s *Service) Insert(ctx context.Context, q *dbgen.Queries, p Prepared, uploader uuid.UUID, originalName string, now time.Time) (Stored, error) {
	name := sanitizeName(originalName)
	row, err := q.InsertAttachment(ctx, dbgen.InsertAttachmentParams{
		UploadedBy: uploader, Kind: p.kind, OriginalName: pgtype.Text{String: name, Valid: name != ""},
		StoragePath: p.rel, ContentType: p.contentType, SizeBytes: int32(len(p.data)), CreatedAt: now,
	})
	if err != nil {
		return Stored{}, fmt.Errorf("recording attachment: %w", err)
	}
	return Stored{ID: row.ID, rel: p.rel, data: p.data, store: s.store, log: s.log}, nil
}

// UploadLetter stores a letter for a requester (POST /attachments, BR-19).
func (s *Service) UploadLetter(ctx context.Context, actor httpx.Principal, originalName string, data []byte) (Attachment, error) {
	if !isRequester(actor) {
		return Attachment{}, httpx.ErrForbidden
	}
	p, err := s.PrepareLetter(data, "file")
	if err != nil {
		return Attachment{}, err
	}

	var out Attachment
	var stored Stored
	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		now := s.clock.Now()
		st, err := s.Insert(ctx, s.q.WithTx(tx), p, actor.UserID, originalName, now)
		if err != nil {
			return err
		}
		stored = st
		name := sanitizeName(originalName)
		out = Attachment{ID: st.ID, Kind: string(p.kind), ContentType: p.contentType, SizeBytes: len(p.data), CreatedAt: clock.ToWIB(now)}
		if name != "" {
			out.OriginalName = &name
		}
		// Last, so a failed write rolls the row back.
		return st.Write()
	})
	if err != nil {
		if stored.store != nil {
			stored.Remove(ctx) // the commit may be what failed
		}
		return Attachment{}, fmt.Errorf("storing letter: %w", err)
	}
	return out, nil
}

// ListLetters returns the requester's own letters, newest first, so one can be used again.
func (s *Service) ListLetters(ctx context.Context, actor httpx.Principal, page, perPage int) ([]Attachment, int64, error) {
	if !isRequester(actor) {
		return nil, 0, httpx.ErrForbidden
	}
	total, err := s.q.CountAttachmentsByUploader(ctx, dbgen.CountAttachmentsByUploaderParams{UploadedBy: actor.UserID, Kind: dbgen.AttachmentKindLETTER})
	if err != nil {
		return nil, 0, fmt.Errorf("counting letters: %w", err)
	}
	rows, err := s.q.ListAttachmentsByUploader(ctx, dbgen.ListAttachmentsByUploaderParams{
		UploadedBy: actor.UserID, Kind: dbgen.AttachmentKindLETTER,
		PageLimit: int32(perPage), PageOffset: int32((page - 1) * perPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing letters: %w", err)
	}
	out := make([]Attachment, len(rows))
	for i, r := range rows {
		out[i] = Attachment{ID: r.ID, Kind: string(r.Kind), ContentType: r.ContentType, SizeBytes: int(r.SizeBytes), CreatedAt: clock.ToWIB(r.CreatedAt)}
		if r.OriginalName.Valid {
			out[i].OriginalName = &r.OriginalName.String
		}
	}
	return out, total, nil
}

// File is an opened attachment and what is needed to serve it. The caller closes File.
type File struct {
	File        *os.File
	ContentType string
	Name        string // suggested download name
	Download    bool   // true for PDFs: served as a download, never rendered in the browser
	ModTime     time.Time
}

// Open returns an attachment for a caller who may see it. Someone who may not gets 404, as for any
// private record, so a file's existence is not revealed; a file removed by retention answers 410.
//
// Who may see what (brief BR-16, BR-19, section 6.2): the uploader and ADMIN always; OB every photo; TU and
// KALAB a photo or letter that is attached to a booking or report of a room type they approve.
func (s *Service) Open(ctx context.Context, actor httpx.Principal, id uuid.UUID) (File, error) {
	row, err := s.q.GetAttachmentAccess(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return File{}, httpx.ErrNotFound
	}
	if err != nil {
		return File{}, fmt.Errorf("reading attachment: %w", err)
	}

	allowed, err := s.mayOpen(ctx, actor, row)
	if err != nil {
		return File{}, err
	}
	if !allowed {
		return File{}, httpx.ErrNotFound
	}
	if row.PurgedAt.Valid {
		return File{}, errGone
	}

	f, err := s.store.Open(row.StoragePath)
	if err != nil {
		// A missing or malformed file is an operational problem, but the client only needs to know there is
		// nothing to show; the details go to the log.
		s.log.ErrorContext(ctx, "opening attachment", slog.String("attachment_id", id.String()), slog.String("error", err.Error()))
		return File{}, httpx.ErrNotFound
	}
	info, err := f.Stat()
	if err != nil {
		closeErr := f.Close()
		return File{}, errors.Join(fmt.Errorf("reading attachment file info: %w", err), closeErr)
	}
	return File{
		File: f, ContentType: row.ContentType, ModTime: info.ModTime(),
		Download: row.ContentType == "application/pdf", Name: downloadName(row.Kind, id, row.ContentType),
	}, nil
}

func (s *Service) mayOpen(ctx context.Context, actor httpx.Principal, row dbgen.GetAttachmentAccessRow) (bool, error) {
	switch {
	case row.UploadedBy == actor.UserID, actor.Role == httpx.RoleAdmin:
		return true, nil
	case actor.Role == httpx.RoleOB:
		return row.Kind == dbgen.AttachmentKindPHOTO, nil
	case actor.HasRole(httpx.RoleTU, httpx.RoleKalab):
		types, err := s.types.ApprovableTypes(ctx, actor.Role)
		if err != nil {
			return false, err
		}
		for _, t := range row.BookingRoomTypes {
			if slices.Contains(types, t) {
				return true, nil
			}
		}
		if row.Kind == dbgen.AttachmentKindPHOTO {
			for _, t := range row.ReportRoomTypes {
				if slices.Contains(types, t) {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func isRequester(actor httpx.Principal) bool {
	return actor.HasRole(httpx.RoleMahasiswa, httpx.RoleOrganisasi)
}

// downloadName is generic on purpose: the name the uploader gave is shown in lists, but a header built
// from it would be one more place for a hostile name to do harm.
func downloadName(kind dbgen.AttachmentKind, id uuid.UUID, contentType string) string {
	ext := ".jpg"
	if contentType == "application/pdf" {
		ext = ".pdf"
	}
	prefix := "foto-"
	if kind == dbgen.AttachmentKindLETTER {
		prefix = "surat-"
	}
	return prefix + id.String()[:8] + ext
}

// sanitizeName keeps the base name only, drops control characters and limits the length. It is stored for
// display and never used to build a path.
func sanitizeName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > maxOriginalNameRunes {
		name = string([]rune(name)[:maxOriginalNameRunes])
	}
	return name
}
