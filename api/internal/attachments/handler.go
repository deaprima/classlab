package attachments

import (
	"errors"
	"io"
	"net/http"
	"strings"

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

// Register mounts the routes on a router that is already behind httpx.Authenticate.
// Uploading and listing letters is for requesters; any signed-in user may ask to open a file, and the
// service decides whether it is theirs to see.
func (h *Handler) Register(r chi.Router) {
	requester := httpx.RequireRoles(httpx.RoleMahasiswa, httpx.RoleOrganisasi)
	r.With(requester).Post("/", h.upload)
	r.With(requester).Get("/", h.list)
	r.Get("/{id}", h.get)
}

// upload serves POST /attachments (multipart: kind=LETTER, file). The body is read without letting the
// standard library spool it to a temporary file, and reading stops one byte past the limit.
func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	limit := h.svc.Limits().MaxLetterBytes

	r.Body = http.MaxBytesReader(w, r.Body, limit+formOverheadBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		httpx.WriteError(w, r, httpx.NewCustomError(http.StatusBadRequest, "BAD_REQUEST", "Permintaan harus berupa multipart/form-data.", nil, err))
		return
	}

	var (
		kind     string
		data     []byte
		haveFile bool
		fileName string
	)
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "file", Message: "Isi form tidak dapat dibaca atau terlalu besar."}))
			return
		}
		switch part.FormName() {
		case "kind":
			val, err := io.ReadAll(io.LimitReader(part, 64))
			if err != nil {
				httpx.WriteError(w, r, httpx.NewCustomError(http.StatusBadRequest, "BAD_REQUEST", "Isi form tidak dapat dibaca.", nil, err))
				return
			}
			kind = strings.TrimSpace(string(val))
		case "file":
			data, err = io.ReadAll(io.LimitReader(part, limit+1))
			if err != nil {
				httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "file", Message: "File tidak dapat dibaca atau terlalu besar."}))
				return
			}
			haveFile, fileName = true, part.FileName()
		}
	}

	var problems []httpx.ErrorDetail
	if kind != "LETTER" {
		problems = append(problems, httpx.ErrorDetail{Field: "kind", Message: "kind harus LETTER."})
	}
	if !haveFile || len(data) == 0 {
		problems = append(problems, httpx.ErrorDetail{Field: "file", Message: "File wajib diunggah pada field 'file'."})
	}
	if len(problems) > 0 {
		httpx.WriteError(w, r, httpx.ValidationError(problems...))
		return
	}

	att, err := h.svc.UploadLetter(r.Context(), actor, fileName, data)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, att)
}

// list serves GET /attachments?kind=LETTER: the requester's own letters.
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	if kind := r.URL.Query().Get("kind"); kind != "" && kind != "LETTER" {
		httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "kind", Message: "kind harus LETTER."}))
		return
	}
	page, perPage, err := httpx.ParsePage(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items, total, err := h.svc.ListLetters(r.Context(), actor, page, perPage)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ListResponse[Attachment]{
		Data: items,
		Meta: httpx.Meta{Page: page, PerPage: perPage, Total: total},
	})
}

// get serves GET /attachments/{id}. The content type comes from our own record, never from the client's,
// and a PDF is always a download so the browser never renders it.
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	f, err := h.svc.Open(r.Context(), actor, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() {
		if err := f.File.Close(); err != nil {
			httpx.LogError(r, "closing attachment", err)
		}
	}()

	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	disposition := "inline"
	if f.Download {
		disposition = "attachment"
	}
	w.Header().Set("Content-Disposition", disposition+`; filename="`+f.Name+`"`)
	http.ServeContent(w, r, f.Name, f.ModTime, f.File)
}
