package users

import (
	"net/http"
	"strconv"

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

// Routes must be mounted behind httpx.Authenticate. Every route is Kaprodi-only.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequireRoles(httpx.RoleKaprodi))
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Patch("/{id}", h.update)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	page, perPage, err := httpx.ParsePage(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	f := ListFilter{Query: r.URL.Query().Get("q"), Page: page, PerPage: perPage}

	if v := r.URL.Query().Get("role"); v != "" {
		role := httpx.Role(v)
		if !role.Valid() {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "role", Message: "Peran tidak dikenal."}))
			return
		}
		f.Role = &role
	}
	if v := r.URL.Query().Get("is_active"); v != "" {
		active, err := strconv.ParseBool(v)
		if err != nil {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "is_active", Message: "is_active harus true atau false."}))
			return
		}
		f.IsActive = &active
	}

	items, total, err := h.svc.List(r.Context(), actor, f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ListResponse[User]{
		Data: items,
		Meta: httpx.Meta{Page: page, PerPage: perPage, Total: total},
	})
}

type createRequest struct {
	OrgUnitID  uuid.UUID  `json:"org_unit_id"`
	Identifier string     `json:"identifier"`
	Name       string     `json:"name"`
	Email      string     `json:"email"`
	Password   string     `json:"password"`
	Role       httpx.Role `json:"role"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	var req createRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	u, err := h.svc.Create(r.Context(), actor, CreateInput(req))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, u)
}

type updateRequest struct {
	Name     *string     `json:"name"`
	Email    *string     `json:"email"`
	Role     *httpx.Role `json:"role"`
	IsActive *bool       `json:"is_active"`
	Password *string     `json:"password"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		// A malformed id can never match a row.
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}

	var req updateRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	u, err := h.svc.Update(r.Context(), actor, id, UpdateInput(req))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}
