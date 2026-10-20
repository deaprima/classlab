package bookings

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"classlab/api/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// checkoutFormOverhead is room in the request for the text fields and multipart framing around the photo.
const checkoutFormOverhead = 64 << 10

type checkInRequest struct {
	RoomCode string `json:"room_code"`
	QRToken  string `json:"qr_token"`
}

func (h *Handler) checkIn(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	var req checkInRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	res, err := h.svc.CheckIn(r.Context(), actor, CheckInInput(req))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

// checkOut serves POST /bookings/{id}/check-out (multipart: condition, note, photo). The body is read part
// by part without spooling to a temporary file, and the photo is read one byte past its limit so an oversized
// file is noticed without loading all of it.
func (h *Handler) checkOut(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxPhotoBytes+checkoutFormOverhead)
	mr, err := r.MultipartReader()
	if err != nil {
		httpx.WriteError(w, r, httpx.NewCustomError(http.StatusBadRequest, "BAD_REQUEST",
			"Permintaan harus berformat multipart/form-data.", nil, err))
		return
	}

	var in CheckOutInput
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "photo", Message: "Isi form tidak dapat dibaca atau terlalu besar."}))
			return
		}
		switch part.FormName() {
		case "condition", "note":
			val, err := io.ReadAll(io.LimitReader(part, maxCheckoutNoteRunes*4+1)) // runes are at most 4 bytes
			if err != nil {
				httpx.WriteError(w, r, httpx.NewCustomError(http.StatusBadRequest, "BAD_REQUEST", "Isi form tidak dapat dibaca.", nil, err))
				return
			}
			if part.FormName() == "condition" {
				in.Condition = strings.TrimSpace(string(val))
			} else {
				in.Note = string(val)
			}
		case "photo":
			data, err := io.ReadAll(io.LimitReader(part, h.maxPhotoBytes+1))
			if err != nil {
				httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "photo", Message: "Foto tidak dapat dibaca atau terlalu besar."}))
				return
			}
			if data == nil {
				data = []byte{} // a named but empty file is a photo (and an invalid one), not "no photo"
			}
			if part.FileName() != "" || len(data) > 0 {
				in.Photo = data
			}
		}
	}

	res, err := h.svc.CheckOut(r.Context(), actor, id, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) checkouts(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	page, perPage, err := httpx.ParsePage(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	f := CheckoutFilter{Page: page, PerPage: perPage}

	q := r.URL.Query()
	if v := q.Get("room_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "room_id", Message: "room_id harus berupa UUID."}))
			return
		}
		f.RoomID = &id
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
		httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "from", Message: "from harus lebih awal dari to."}))
		return
	}

	items, total, err := h.svc.Checkouts(r.Context(), actor, f)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ListResponse[Checkout]{
		Data: items, Meta: httpx.Meta{Page: page, PerPage: perPage, Total: total},
	})
}
