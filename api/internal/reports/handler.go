package reports

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"classlab/api/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	// formOverhead is room in the request for the text fields and multipart framing on top of the photo.
	formOverhead = 64 << 10
	// formMemory is how much of a multipart body is held in memory; the body cap keeps it below this.
	formMemory = 8 << 20
)

type Handler struct {
	svc           *Service
	maxPhotoBytes int64
}

func NewHandler(svc *Service, maxPhotoBytes int64) *Handler {
	return &Handler{svc: svc, maxPhotoBytes: maxPhotoBytes}
}

// Register mounts the report routes on a router that is already behind httpx.Authenticate.
// Any role may file and read their own reports; only TU/Kalab changes status.
func (h *Handler) Register(r chi.Router) {
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Get("/{id}/photo", h.photo)
	r.With(httpx.RequireRoles(httpx.RoleTUKalab)).Patch("/{id}/status", h.updateStatus)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	if mt := r.Header.Get("Content-Type"); !strings.HasPrefix(strings.ToLower(mt), "multipart/form-data") {
		httpx.WriteError(w, r, httpx.NewCustomError(http.StatusBadRequest, "BAD_REQUEST",
			"Permintaan harus berformat multipart/form-data.", nil, httpx.ErrBadRequest))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxPhotoBytes+formOverhead)
	if err := r.ParseMultipartForm(formMemory); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "photo", Message: "Ukuran foto terlalu besar."}))
			return
		}
		httpx.WriteError(w, r, httpx.NewCustomError(http.StatusBadRequest, "BAD_REQUEST",
			"Body multipart tidak dapat dibaca.", nil, err))
		return
	}
	defer func() {
		if err := r.MultipartForm.RemoveAll(); err != nil {
			// Temp files are only used for bodies above formMemory, which the body cap rules out.
			httpx.LogError(r, "removing multipart temp files", err)
		}
	}()

	in := CreateInput{Description: r.FormValue("description")}
	if v := r.FormValue("room_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "room_id", Message: "room_id harus berupa UUID."}))
			return
		}
		in.RoomID = id
	}

	photo, err := h.readPhoto(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	in.Photo = photo

	created, err := h.svc.Create(r.Context(), actor, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

// readPhoto returns the uploaded photo bytes, or nil when none was sent. Browsers send an empty file
// part when no file is chosen; that counts as no photo. Reading stops one byte past the limit so an
// oversized file is detected without loading all of it.
func (h *Handler) readPhoto(r *http.Request) ([]byte, error) {
	file, header, err := r.FormFile("photo")
	if errors.Is(err, http.ErrMissingFile) {
		return nil, nil
	}
	if err != nil {
		return nil, httpx.NewCustomError(http.StatusBadRequest, "BAD_REQUEST", "Berkas foto tidak dapat dibaca.", nil, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			httpx.LogError(r, "closing uploaded photo", err)
		}
	}()

	if header.Filename == "" && header.Size == 0 {
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(file, h.maxPhotoBytes+1))
	if err != nil {
		return nil, httpx.NewCustomError(http.StatusBadRequest, "BAD_REQUEST", "Berkas foto tidak dapat dibaca.", nil, err)
	}
	if data == nil {
		data = []byte{} // a named but empty file is a photo (and an invalid one), not "no photo"
	}
	return data, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	page, perPage, err := httpx.ParsePage(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	f := ListFilter{Page: page, PerPage: perPage}

	q := r.URL.Query()
	if v := q.Get("status"); v != "" {
		if _, ok := parseStatus(v); !ok {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "status", Message: "status harus BARU, DITERIMA, DIPROSES, atau SELESAI."}))
			return
		}
		f.Status = &v
	}
	if v := q.Get("room_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "room_id", Message: "room_id harus berupa UUID."}))
			return
		}
		f.RoomID = &id
	}

	items, total, err := h.svc.List(r.Context(), actor, f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ListResponse[Report]{
		Data: items, Meta: httpx.Meta{Page: page, PerPage: perPage, Total: total},
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	d, err := h.svc.Get(r.Context(), actor, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

// photo serves the stored image to the people allowed to see the report. The type comes from our own
// record of the file, and nosniff stops browsers from second-guessing it.
func (h *Handler) photo(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	p, err := h.svc.Photo(r.Context(), actor, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() {
		if err := p.File.Close(); err != nil {
			httpx.LogError(r, "closing report photo", err)
		}
	}()

	hdr := w.Header()
	hdr.Set("Content-Type", p.ContentType)
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Disposition", "inline")
	hdr.Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, "", p.ModTime, p.File)
}

type statusRequest struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req statusRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	updated, err := h.svc.UpdateStatus(r.Context(), actor, id, req.Status, req.Note)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return uuid.Nil, false
	}
	return id, true
}
