package violations

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

// Register mounts the routes on the authenticated API router. Reading is for the approvers' roles and,
// for their own record only, for requesters; annulling is for ADMIN. The service checks the roles again.
func (h *Handler) Register(r chi.Router) {
	r.With(httpx.RequireRoles(httpx.RoleTU, httpx.RoleKalab, httpx.RoleAdmin, httpx.RoleMahasiswa, httpx.RoleOrganisasi)).Get("/users/{id}/violations", h.list)
	r.With(httpx.RequireRoles(httpx.RoleAdmin)).Post("/violations/{id}/annul", h.annul)
}

type listResponse struct {
	Data                []Violation `json:"data"`
	Meta                httpx.Meta  `json:"meta"`
	ActivePoints        int         `json:"active_points"`
	BookingBlockedUntil *time.Time  `json:"booking_blocked_until"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	page, perPage, err := httpx.ParsePage(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hist, err := h.svc.List(r.Context(), actor, id, page, perPage)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, listResponse{
		Data: hist.Data, Meta: httpx.Meta{Page: page, PerPage: perPage, Total: hist.Total},
		ActivePoints: hist.ActivePoints, BookingBlockedUntil: hist.BookingBlockedUntil,
	})
}

type annulRequest struct {
	Reason string `json:"reason"`
}

func (h *Handler) annul(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	var req annulRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	v, err := h.svc.Annul(r.Context(), actor, id, req.Reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}
