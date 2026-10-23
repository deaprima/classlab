package httpx

import (
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"time"

	"classlab/api/internal/platform/logger"

	"github.com/google/uuid"
)

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// RequestID middleware generates or preserves a unique X-Request-Id header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A caller-supplied id is kept so a request can be traced across services, but only if it is
		// short and plain: it is echoed in a header, the error body and every log line.
		reqID := r.Header.Get("X-Request-Id")
		if !validRequestID.MatchString(reqID) {
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
				slog.String("remote_addr", ClientInfoFrom(r.Context()).IP),
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

// NoStore marks API responses as uncacheable: they are per-user and often sensitive. A handler that
// wants caching (the report photo) overrides the header.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// NoSniff stops a browser from guessing a response's type, on every response (JSON, errors, health), not
// only the file downloads that set it themselves. The API is normally reached through Caddy, which adds the
// same header; this keeps it when the API is reached any other way.
func NoSniff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
