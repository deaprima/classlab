package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// WithRequestID returns a new context with the given request ID.
func WithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, requestIDKey, reqID)
}

// RequestIDFromContext retrieves the request ID from context if present.
func RequestIDFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(requestIDKey).(string); ok {
		return val
	}
	return ""
}

// ContextHandler wraps an slog.Handler to inject request_id from context.
type ContextHandler struct {
	slog.Handler
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if reqID := RequestIDFromContext(ctx); reqID != "" {
		r.AddAttrs(slog.String("request_id", reqID))
	}
	return h.Handler.Handle(ctx, r)
}

// New creates a JSON slog.Logger with the specified level.
func New(levelStr string) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(&ContextHandler{Handler: handler})
}
