package httpx

import (
	"encoding/json"
	"net/http"
)

type Meta struct {
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
	Total   int64 `json:"total"`
}

type ListResponse[T any] struct {
	Data []T  `json:"data"`
	Meta Meta `json:"meta"`
}

type DataResponse[T any] struct {
	Data T `json:"data"`
}

// JSON writes a JSON payload to the ResponseWriter with status code.
func JSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

// WriteError writes a standard ErrorEnvelope response using MapError.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := r.Header.Get("X-Request-Id")
	statusCode, env := MapError(err, reqID)
	JSON(w, statusCode, env)
}
