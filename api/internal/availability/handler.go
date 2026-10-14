package availability

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

// Check serves GET /availability. Every authenticated role may read it.
func (h *Handler) Check(w http.ResponseWriter, r *http.Request) {
	start, err := httpx.ParseTimeParam(r, "start_at")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	end, err := httpx.ParseTimeParam(r, "end_at")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	f := Filter{Start: start, End: end}
	q := r.URL.Query()
	if v := q.Get("type"); v != "" {
		if v != "KELAS" && v != "LAB" {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "type", Message: "Tipe ruang harus KELAS atau LAB."}))
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

	items, err := h.svc.Check(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.DataResponse[[]RoomAvailability]{Data: items})
}

// Timeline serves GET /rooms/{id}/schedule.
func (h *Handler) Timeline(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	from, err := httpx.ParseTimeParam(r, "from")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	to, err := httpx.ParseTimeParam(r, "to")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	items, err := h.svc.Timeline(r.Context(), id, from, to)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.DataResponse[[]TimelineItem]{Data: items})
}
