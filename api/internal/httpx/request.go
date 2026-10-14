package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
)

const maxJSONBodyBytes = 1 << 20

// DecodeJSON parses a single JSON object, rejecting unknown fields and oversized bodies.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return NewCustomError(http.StatusBadRequest, "BAD_REQUEST", "Body permintaan bukan JSON yang valid.", nil, err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return NewCustomError(http.StatusBadRequest, "BAD_REQUEST", "Body permintaan harus berisi satu objek JSON.", nil, ErrBadRequest)
	}
	return nil
}

const (
	defaultPerPage = 20
	maxPerPage     = 100
)

// ParsePage reads ?page= and ?per_page=, rejecting malformed or out-of-range values.
func ParsePage(r *http.Request) (page, perPage int, err error) {
	page, perPage = 1, defaultPerPage
	q := r.URL.Query()

	if v := q.Get("page"); v != "" {
		page, err = strconv.Atoi(v)
		if err != nil || page < 1 {
			return 0, 0, ValidationError(ErrorDetail{Field: "page", Message: "page harus bilangan bulat >= 1."})
		}
	}
	if v := q.Get("per_page"); v != "" {
		perPage, err = strconv.Atoi(v)
		if err != nil || perPage < 1 || perPage > maxPerPage {
			return 0, 0, ValidationError(ErrorDetail{Field: "per_page", Message: "per_page harus antara 1 dan 100."})
		}
	}
	return page, perPage, nil
}
