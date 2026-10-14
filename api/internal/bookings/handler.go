package bookings

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts the booking routes on a router that is already behind httpx.Authenticate.
// The role guards here are the coarse gate; the service re-checks roles and ownership itself.
func (h *Handler) Register(r chi.Router) {
	reader := httpx.RequireRoles(httpx.RoleMahasiswa, httpx.RoleTUKalab, httpx.RoleKaprodi)

	r.With(httpx.RequireRoles(httpx.RoleMahasiswa)).Post("/", h.create)
	r.With(reader).Get("/", h.list)
	r.With(reader).Get("/{id}", h.get)
	r.With(reader).Get("/{id}/events", h.events)
	r.With(httpx.RequireRoles(httpx.RoleTUKalab)).Post("/{id}/approve", h.approve)
	r.With(httpx.RequireRoles(httpx.RoleTUKalab)).Post("/{id}/reject", h.reject)
	r.With(httpx.RequireRoles(httpx.RoleMahasiswa, httpx.RoleTUKalab)).Post("/{id}/cancel", h.cancel)
}

type createRequest struct {
	RoomID       uuid.UUID `json:"room_id"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
	Purpose      string    `json:"purpose"`
	Participants *int      `json:"participants"`
	Notes        *string   `json:"notes"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	var req createRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	created, replayed, err := h.svc.Create(r.Context(), actor, CreateInput{
		RoomID: req.RoomID, StartAt: req.StartAt, EndAt: req.EndAt, Purpose: req.Purpose,
		Participants: req.Participants, Notes: req.Notes, IdempotencyKey: r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	httpx.JSON(w, http.StatusCreated, created)
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
		switch dbgen.BookingStatus(v) {
		case dbgen.BookingStatusMENUNGGU, dbgen.BookingStatusDISETUJUI, dbgen.BookingStatusDITOLAK, dbgen.BookingStatusDIBATALKAN:
			f.Status = &v
		default:
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "status", Message: "status harus MENUNGGU, DISETUJUI, DITOLAK, atau DIBATALKAN."}))
			return
		}
	}
	if v := q.Get("room_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "room_id", Message: "room_id harus berupa UUID."}))
			return
		}
		f.RoomID = &id
	}
	if v := q.Get("mine"); v != "" {
		mine, err := strconv.ParseBool(v)
		if err != nil {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "mine", Message: "mine harus true atau false."}))
			return
		}
		f.Mine = mine
	}
	for name, dst := range map[string]**time.Time{"from": &f.From, "to": &f.To} {
		if q.Get(name) == "" {
			continue
		}
		t, err := httpx.ParseTimeParam(r, name)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		*dst = &t
	}
	if f.From != nil && f.To != nil && !f.From.Before(*f.To) {
		httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "to", Message: "to harus setelah from."}))
		return
	}

	items, total, err := h.svc.List(r.Context(), actor, f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ListResponse[Booking]{
		Data: items, Meta: httpx.Meta{Page: page, PerPage: perPage, Total: total},
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	b, err := h.svc.Get(r.Context(), actor, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, b)
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	items, err := h.svc.Events(r.Context(), actor, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.DataResponse[[]Event]{Data: items})
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	b, err := h.svc.Approve(r.Context(), actor, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, b)
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

// decodeReason reads an optional {"reason": "..."} body. An empty body means no reason, so the
// service decides whether that is acceptable (422 for reject, fine for a student cancelling).
func decodeReason(w http.ResponseWriter, r *http.Request) (string, error) {
	var req reasonRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return req.Reason, nil
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	reason, err := decodeReason(w, r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	b, err := h.svc.Reject(r.Context(), actor, id, reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, b)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	reason, err := decodeReason(w, r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	b, err := h.svc.Cancel(r.Context(), actor, id, reason)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, b)
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return uuid.Nil, false
	}
	return id, true
}
