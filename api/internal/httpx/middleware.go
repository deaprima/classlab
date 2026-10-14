package httpx

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"classlab/api/internal/platform/logger"

	"github.com/google/uuid"
)

// RequestID middleware generates or preserves a unique X-Request-Id header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = "req_" + uuid.New().String()
		}

		w.Header().Set("X-Request-Id", reqID)
		ctx := logger.WithRequestID(r.Context(), reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the real writer, which streaming responses (SSE) need
// for Flush and for clearing the server's write deadline.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// RequestLogger logs HTTP requests using log/slog without leaking sensitive data.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rec, r)

			duration := time.Since(start)
			reqID := logger.RequestIDFromContext(r.Context())

			log.InfoContext(r.Context(), "http_request",
				slog.String("request_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.Int("status", rec.statusCode),
				slog.Duration("duration", duration),
			)
		})
	}
}

// Recoverer turns a panic into a logged 500 in the standard error format.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			slog.ErrorContext(r.Context(), "panic recovered",
				slog.Any("panic", rec), slog.String("stack", string(debug.Stack())))
			WriteError(w, r, ErrInternal)
		}()
		next.ServeHTTP(w, r)
	})
}
