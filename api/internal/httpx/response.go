package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"classlab/api/internal/platform/logger"
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
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			// Headers are already sent, so the only useful thing left is a log line.
			slog.Warn("failed to encode response", slog.String("error", err.Error()))
		}
	}
}

// WriteError writes a standard ErrorEnvelope response using MapError.
// Unmapped errors become 500 and are logged here, because the client only sees a generic message.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := logger.RequestIDFromContext(r.Context())
	statusCode, env := MapError(err, reqID)
	if statusCode >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "unhandled error", slog.String("error", err.Error()))
	}
	JSON(w, statusCode, env)
}
