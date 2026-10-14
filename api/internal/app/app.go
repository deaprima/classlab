// Package app wires configuration, services and routes into one http.Handler.
// Keeping this out of main lets integration tests drive the real router.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"classlab/api/internal/auth"
	"classlab/api/internal/availability"
	"classlab/api/internal/bookings"
	"classlab/api/internal/config"
	"classlab/api/internal/httpx"
	"classlab/api/internal/notifications"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/password"
	"classlab/api/internal/reports"
	"classlab/api/internal/rooms"
	"classlab/api/internal/schedules"
	"classlab/api/internal/users"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

const (
	loginMaxAttempts = 5
	loginWindow      = 15 * time.Minute

	// maxStreamsPerUser is how many notification streams one user may hold open (tabs, devices).
	maxStreamsPerUser = 5
)

// NewRouter fails only on an invalid booking configuration; config.Load already rejects that at start.
// ctx bounds background work owned by the router (the notification listener) and should last as long
// as the server.
func NewRouter(ctx context.Context, cfg *config.Config, pool *db.Pool, log *slog.Logger, clk clock.Clock) (http.Handler, error) {
	rules, err := cfg.BookingRules()
	if err != nil {
		return nil, fmt.Errorf("booking configuration: %w", err)
	}

	hasher := password.NewHasher(cfg.Argon2MemoryKiB, cfg.Argon2Iterations, cfg.Argon2Parallelism)

	usersSvc := users.NewService(pool, hasher, clk)
	tokens := auth.NewTokenIssuer(cfg.JWTSecret, cfg.JWTAccessExpiry, clk)
	limiter := auth.NewLoginLimiter(loginMaxAttempts, loginWindow, clk)
	authSvc := auth.NewService(pool, usersSvc, hasher, tokens, limiter, clk, cfg.RefreshTokenExpiry, log)

	authenticate := httpx.Authenticate(authSvc)

	roomsHandler := rooms.NewHandler(rooms.NewService(pool, clk))
	schedulesHandler := schedules.NewHandler(schedules.NewService(pool, clk))
	availabilityHandler := availability.NewHandler(availability.NewService(pool))
	bookingsHandler := bookings.NewHandler(bookings.NewService(pool, clk, rules))

	notificationsHandler := notifications.NewHandler(
		notifications.NewService(pool, clk),
		notifications.NewHub(ctx, pool.Config().ConnString(), log),
		// A stream ends before the access token it was opened with expires.
		notifications.StreamOptions{Heartbeat: cfg.SSEHeartbeat, MaxLifetime: cfg.JWTAccessExpiry, MaxPerUser: maxStreamsPerUser},
		log,
	)
	reportsHandler := reports.NewHandler(
		reports.NewService(pool, clk, reports.NewStorage(cfg.UploadDir), cfg.UploadMaxBytes, log),
		cfg.UploadMaxBytes,
	)

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(httpx.RequestLogger(log))
	r.Use(httpx.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link", "X-Request-Id", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) { httpx.WriteError(w, r, httpx.ErrNotFound) })
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, r, httpx.NewCustomError(http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED",
			"Metode HTTP tidak diizinkan untuk endpoint ini.", nil, httpx.ErrBadRequest))
	})

	r.Get("/health", health(pool, clk))

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(httpx.NoStore)
		r.Mount("/auth", auth.NewHandler(authSvc, cfg.CookieSecure).Routes(authenticate))
		r.With(authenticate).Mount("/users", users.NewHandler(usersSvc).Routes())
		r.Group(func(r chi.Router) {
			r.Use(authenticate)
			r.Route("/rooms", func(r chi.Router) {
				roomsHandler.Register(r)
				r.Get("/{id}/schedule", availabilityHandler.Timeline)
			})
			r.Route("/schedules", schedulesHandler.Register)
			r.Route("/bookings", bookingsHandler.Register)
			r.Route("/notifications", notificationsHandler.Register)
			r.Route("/reports", reportsHandler.Register)
			r.Get("/availability", availabilityHandler.Check)
		})
	})

	return r, nil
}

func health(pool *db.Pool, clk clock.Clock) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		now := clk.Now().Format(time.RFC3339)
		if err := pool.Ping(pingCtx); err != nil {
			// The driver error can name hosts and users, so it goes to the log, not the response.
			slog.ErrorContext(r.Context(), "health check: database ping failed", slog.String("error", err.Error()))
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "unhealthy", "database": "error", "time": now,
			})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{
			"status": "healthy", "database": "ok", "version": "0.1.0", "time": now,
		})
	}
}
