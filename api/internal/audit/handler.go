package audit

import (
	"net/http"
	"time"

	"classlab/api/internal/httpx"

	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// List serves GET /audit-logs. It is read-only: the log has no write or delete endpoint.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	page, perPage, err := httpx.ParsePage(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	q := r.URL.Query()
	f := ListFilter{Action: q.Get("action"), EntityType: q.Get("entity_type"), Page: page, PerPage: perPage}

	if f.ActorID, err = optionalUUID(q.Get("actor_id"), "actor_id"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if f.EntityID, err = optionalUUID(q.Get("entity_id"), "entity_id"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if f.From, err = optionalTime(r, "from"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if f.To, err = optionalTime(r, "to"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items, total, err := h.svc.List(r.Context(), actor, f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ListResponse[Log]{
		Data: items,
		Meta: httpx.Meta{Page: page, PerPage: perPage, Total: total},
	})
}

func optionalUUID(raw, field string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, httpx.ValidationError(httpx.ErrorDetail{Field: field, Message: field + " harus berupa UUID."})
	}
	return &id, nil
}

func optionalTime(r *http.Request, name string) (*time.Time, error) {
	if r.URL.Query().Get(name) == "" {
		return nil, nil
	}
	t, err := httpx.ParseTimeParam(r, name)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
