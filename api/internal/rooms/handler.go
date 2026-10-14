package rooms

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

// Register mounts the room routes on a router that is already behind httpx.Authenticate.
// Reads are open to every role; writes are limited to TU_KALAB and KAPRODI.
func (h *Handler) Register(r chi.Router) {
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Group(func(r chi.Router) {
		r.Use(httpx.RequireRoles(httpx.RoleTUKalab, httpx.RoleKaprodi))
		r.Post("/", h.create)
		r.Patch("/{id}", h.update)
		r.Delete("/{id}", h.delete)
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, perPage, err := httpx.ParsePage(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := r.URL.Query()
	f := ListFilter{Query: q.Get("q"), Page: page, PerPage: perPage}

	if v := q.Get("type"); v != "" {
		if problems := validateType(v); problems != nil {
			httpx.WriteError(w, r, httpx.ValidationError(problems...))
			return
		}
		f.Type = &v
	}
	if v := q.Get("capacity_min"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "capacity_min", Message: "capacity_min harus bilangan bulat >= 0."}))
			return
		}
		f.CapacityMin = &n
	}
	if v := q.Get("is_active"); v != "" {
		active, err := strconv.ParseBool(v)
		if err != nil {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "is_active", Message: "is_active harus true atau false."}))
			return
		}
		f.IsActive = &active
	}

	items, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ListResponse[Room]{
		Data: items,
		Meta: httpx.Meta{Page: page, PerPage: perPage, Total: total},
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	room, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, room)
}

type createRequest struct {
	OrgUnitID  uuid.UUID `json:"org_unit_id"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Capacity   int       `json:"capacity"`
	Facilities []string  `json:"facilities"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	var req createRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	room, err := h.svc.Create(r.Context(), actor, CreateInput(req))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, room)
}

type updateRequest struct {
	Name       *string   `json:"name"`
	Type       *string   `json:"type"`
	Capacity   *int      `json:"capacity"`
	Facilities *[]string `json:"facilities"`
	IsActive   *bool     `json:"is_active"`
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
	room, err := h.svc.Update(r.Context(), actor, id, UpdateInput(req))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, room)
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

// parseID treats a malformed id as "not found": it can never match a row.
func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return uuid.Nil, false
	}
	return id, true
}
