package roomtypes

import (
	"net/http"

	"classlab/api/internal/httpx"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts the routes on a router that is already behind httpx.Authenticate.
// Reading is open to every role; only administrators change a policy.
func (h *Handler) Register(r chi.Router) {
	r.Get("/", h.list)
	r.With(httpx.RequireRoles(httpx.RoleAdmin)).Patch("/{type}", h.update)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.DataResponse[[]View]{Data: items})
}

type updateRequest struct {
	ApproverRoles  *[]string `json:"approver_roles"`
	RequesterRoles *[]string `json:"requester_roles"`
	ApprovalPolicy *string   `json:"approval_policy"`
	LetterRequired *bool     `json:"letter_required"`
	OpenTime       *string   `json:"open_time"`
	CloseTime      *string   `json:"close_time"`
	MinDurationMin *int      `json:"min_duration_min"`
	MaxDurationMin *int      `json:"max_duration_min"`
	MaxAdvanceDays *int      `json:"max_advance_days"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	var req updateRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	view, err := h.svc.Update(r.Context(), actor, chi.URLParam(r, "type"), UpdateInput(req))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, view)
}
