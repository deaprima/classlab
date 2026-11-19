package imports

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"classlab/api/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// formOverheadBytes covers multipart boundaries and the small text fields around the file.
const formOverheadBytes = 64 << 10

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Routes must be mounted behind httpx.Authenticate and httpx.RequirePasswordChanged. TU and
// administrators only; the service re-checks.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequireRoles(httpx.RoleTU, httpx.RoleAdmin))
	r.Get("/", h.list)
	r.Post("/users", h.users)
	r.Post("/schedules", h.schedules)
	r.Post("/{id}/rollback", h.rollback)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	page, perPage, err := httpx.ParsePage(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items, total, err := h.svc.List(r.Context(), actor, ListFilter{Kind: r.URL.Query().Get("kind"), Page: page, PerPage: perPage})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ListResponse[Import]{
		Data: items,
		Meta: httpx.Meta{Page: page, PerPage: perPage, Total: total},
	})
}

// upload is a multipart form already read into memory and bounded.
type upload struct {
	file     []byte
	fileName string
	fields   map[string]string
}

// readUpload reads the multipart body without letting the standard library spool it to a temporary
// file: the CSV is never written to disk (BR-23, BR-24). Fields other than "file" are short text.
func readUpload(w http.ResponseWriter, r *http.Request, maxFileBytes int64) (upload, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFileBytes+formOverheadBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		return upload{}, httpx.NewCustomError(http.StatusBadRequest, "BAD_REQUEST", "Permintaan harus berupa multipart/form-data.", nil, err)
	}

	up := upload{fields: map[string]string{}}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return upload{}, httpx.NewCustomError(http.StatusBadRequest, "BAD_REQUEST", "Isi form tidak dapat dibaca atau terlalu besar.", nil, err)
		}

		switch part.FormName() {
		case "file":
			// One byte more than allowed lets the parser report "too large" instead of silently truncating.
			data, err := io.ReadAll(io.LimitReader(part, maxFileBytes+1))
			if err != nil {
				return upload{}, httpx.NewCustomError(http.StatusBadRequest, "BAD_REQUEST", "File tidak dapat dibaca.", nil, err)
			}
			up.file, up.fileName = data, sanitizeFileName(part.FileName())
		default:
			val, err := io.ReadAll(io.LimitReader(part, 1<<10))
			if err != nil {
				return upload{}, httpx.NewCustomError(http.StatusBadRequest, "BAD_REQUEST", "Isi form tidak dapat dibaca.", nil, err)
			}
			up.fields[part.FormName()] = string(bytes.TrimSpace(val))
		}
	}
	if up.file == nil {
		return upload{}, httpx.ValidationError(httpx.ErrorDetail{Field: "file", Message: "File CSV wajib diunggah pada field 'file'."})
	}
	return up, nil
}

// sanitizeFileName keeps only the base name and a safe length; it is shown in the history, nothing more.
func sanitizeFileName(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' || name[i] == '\\' {
			name = name[i+1:]
			break
		}
	}
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

// users serves POST /imports/users: a preview with ?dry_run=true, otherwise the apply.
func (h *Handler) users(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	up, err := readUpload(w, r, h.svc.Limits().MaxBytes)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	semester := up.fields["semester"]

	if r.URL.Query().Get("dry_run") == "true" {
		preview, err := h.svc.PreviewUsers(r.Context(), actor, bytes.NewReader(up.file), semester)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, preview)
		return
	}

	result, err := h.svc.ApplyUsers(r.Context(), actor, up.fileName, bytes.NewReader(up.file), semester, up.fields["plan_hash"])
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// The response carries temporary passwords, so it must never be cached or replayed.
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, result)
}

// schedules serves POST /imports/schedules: a preview with ?dry_run=true, otherwise the apply.
func (h *Handler) schedules(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	up, err := readUpload(w, r, h.svc.Limits().MaxBytes)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if r.URL.Query().Get("dry_run") == "true" {
		preview, err := h.svc.PreviewSchedules(r.Context(), actor, bytes.NewReader(up.file))
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, preview)
		return
	}

	result, err := h.svc.ApplySchedules(r.Context(), actor, up.fileName, bytes.NewReader(up.file), up.fields["plan_hash"])
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// rollback serves POST /imports/{id}/rollback.
func (h *Handler) rollback(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	result, err := h.svc.Rollback(r.Context(), actor, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}
