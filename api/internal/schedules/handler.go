package schedules

import (
	"net/http"
	"time"

	"classlab/api/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts the schedule routes on a router that is already behind httpx.Authenticate.
func (h *Handler) Register(r chi.Router) {
	r.Use(httpx.RequireRoles(httpx.RoleTU, httpx.RoleKalab, httpx.RoleAdmin))
	r.Post("/", h.create)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
}

type createRequest struct {
	RoomID  uuid.UUID `json:"room_id"`
	Title   string    `json:"title"`
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
	Source  string    `json:"source"`
	Repeat  *struct {
		Freq  string `json:"freq"`
		Until string `json:"until"`
	} `json:"repeat"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	var req createRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	in := CreateInput{RoomID: req.RoomID, Title: req.Title, StartAt: req.StartAt, EndAt: req.EndAt, Source: req.Source}
	if req.Repeat != nil {
		in.Repeat = &Repeat{Freq: req.Repeat.Freq, Until: req.Repeat.Until}
	}

	created, err := h.svc.Create(r.Context(), actor, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, httpx.DataResponse[[]Schedule]{Data: created})
}

type updateRequest struct {
	Title   *string    `json:"title"`
	StartAt *time.Time `json:"start_at"`
	EndAt   *time.Time `json:"end_at"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var req updateRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	updated, err := h.svc.Update(r.Context(), actor, id, UpdateInput(req))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), actor, id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return uuid.Nil, false
	}
	return id, true
}
