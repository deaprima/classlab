package bookings

import (
	"encoding/json"
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
	reader := httpx.RequireRoles(httpx.RoleMahasiswa, httpx.RoleOrganisasi, httpx.RoleTU, httpx.RoleKalab, httpx.RoleAdmin)
	manager := httpx.RequireRoles(httpx.RoleTU, httpx.RoleKalab, httpx.RoleAdmin)
	requester := httpx.RequireRoles(httpx.RoleMahasiswa, httpx.RoleOrganisasi)

	r.With(requester).Post("/", h.create)
	r.With(reader).Get("/", h.list)
	r.With(reader).Get("/{id}", h.get)
	r.With(reader).Get("/{id}/events", h.events)
	r.With(requester).Patch("/{id}", h.edit)
	r.With(manager).Post("/{id}/approve", h.approve)
	r.With(manager).Post("/{id}/reject", h.reject)
	r.With(reader).Post("/{id}/cancel", h.cancel)
}

type occurrenceRequest struct {
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
}

// createRequest takes either start_at and end_at (one date) or occurrences (one to MAX_OCCURRENCES dates).
type createRequest struct {
	RoomID             uuid.UUID            `json:"room_id"`
	StartAt            time.Time            `json:"start_at"`
	EndAt              time.Time            `json:"end_at"`
	Occurrences        *[]occurrenceRequest `json:"occurrences"`
	Purpose            string               `json:"purpose"`
	Participants       *int                 `json:"participants"`
	Notes              *string              `json:"notes"`
	Details            json.RawMessage      `json:"details"`
	LetterAttachmentID *uuid.UUID           `json:"letter_attachment_id"`
	LetterNumber       *string              `json:"letter_number"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	var req createRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	in := CreateInput{
		RoomID: req.RoomID, StartAt: req.StartAt, EndAt: req.EndAt, Purpose: req.Purpose,
		Participants: req.Participants, Notes: req.Notes, Details: req.Details,
		LetterAttachmentID: req.LetterAttachmentID, LetterNumber: req.LetterNumber,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	}
	if req.Occurrences != nil {
		in.HasOccurrences = true
		for _, o := range *req.Occurrences {
			in.Occurrences = append(in.Occurrences, Occurrence{StartAt: o.StartAt, EndAt: o.EndAt})
		}
	}

	res, err := h.svc.Create(r.Context(), actor, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if res.Replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	if res.Group != nil {
		httpx.JSON(w, http.StatusCreated, res.Group)
		return
	}
	httpx.JSON(w, http.StatusCreated, res.Booking)
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
		if !dbgen.BookingStatus(v).Valid() {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "status", Message: "Status booking tidak dikenal."}))
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
	if v := q.Get("type"); v != "" {
		if !dbgen.RoomType(v).Valid() {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "type", Message: "Tipe ruang tidak dikenal."}))
			return
		}
		f.RoomType = &v
	}
	if v := q.Get("group_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "group_id", Message: "group_id harus berupa UUID."}))
			return
		}
		f.GroupID = &id
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
	Scope  string `json:"scope"`
}

// decodeReason reads an optional {"reason": "..."} body. An empty body means no reason, so the
// service decides whether that is acceptable (422 for reject, fine for a student cancelling).
func decodeReason(w http.ResponseWriter, r *http.Request) (reasonRequest, error) {
	var req reasonRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil && !errors.Is(err, io.EOF) {
		return reasonRequest{}, err
	}
	return req, nil
}

func (h *Handler) reject(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	req, err := decodeReason(w, r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	b, err := h.svc.Reject(r.Context(), actor, id, req.Reason)
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
	req, err := decodeReason(w, r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	b, err := h.svc.Cancel(r.Context(), actor, id, req.Reason, req.Scope)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, b)
}

type editRequest struct {
	StartAt            *time.Time       `json:"start_at"`
	EndAt              *time.Time       `json:"end_at"`
	Purpose            *string          `json:"purpose"`
	Participants       *int             `json:"participants"`
	Notes              *string          `json:"notes"`
	Details            *json.RawMessage `json:"details"`
	LetterAttachmentID *uuid.UUID       `json:"letter_attachment_id"`
	LetterNumber       *string          `json:"letter_number"`
}

// edit serves PATCH /bookings/{id}. The room is not editable: an unknown field such as room_id is a 400.
func (h *Handler) edit(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req editRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	b, err := h.svc.Edit(r.Context(), actor, id, EditInput(req))
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
