// Package app wires configuration, services and routes into one http.Handler.
// Keeping this out of main lets integration tests drive the real router.
package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"classlab/api/internal/attachments"
	"classlab/api/internal/audit"
	"classlab/api/internal/auth"
	"classlab/api/internal/availability"
	"classlab/api/internal/bookings"
	"classlab/api/internal/config"
	"classlab/api/internal/httpx"
	"classlab/api/internal/imports"
	"classlab/api/internal/jobs"
	"classlab/api/internal/notifications"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/password"
	"classlab/api/internal/platform/ratelimit"
	"classlab/api/internal/reports"
	"classlab/api/internal/rooms"
	"classlab/api/internal/roomtypes"
	"classlab/api/internal/schedules"
	"classlab/api/internal/users"
	"classlab/api/internal/violations"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

const (
	loginMaxAttempts = 5
	loginWindow      = 15 * time.Minute

	// maxStreamsPerUser is how many notification streams one user may hold open (tabs, devices).
	maxStreamsPerUser = 5
)

// NewRouter wires configuration, services and routes; config.Load has already validated every setting at start.
// ctx bounds background work owned by the router (the notification listener) and should last as long
// as the server.
func NewRouter(ctx context.Context, cfg *config.Config, pool *db.Pool, log *slog.Logger, clk clock.Clock) (http.Handler, error) {
	hasher := password.NewHasher(cfg.Argon2MemoryKiB, cfg.Argon2Iterations, cfg.Argon2Parallelism)

	types := roomtypes.NewStore(pool)

	// In-app is the only channel for now; Web Push (Phase 8) is added here as one more Medium.
	notifier := notifications.NewNotifier(notifications.InApp{})

	usersSvc := users.NewService(pool, hasher, clk, users.Options{
		MinPasswordLength:  cfg.Business.MinPasswordLength,
		TempPasswordLength: cfg.Business.TempPasswordLength,
		ViolationWindow:    cfg.Business.ViolationWindow,
	})
	tokens := auth.NewTokenIssuer(cfg.JWTSecret, cfg.JWTAccessExpiry, clk)
	limiter := auth.NewLoginLimiter(loginMaxAttempts, loginWindow, clk)
	authSvc := auth.NewService(pool, usersSvc, hasher, tokens, limiter, clk, cfg.RefreshTokenExpiry, log)

	authenticate := httpx.Authenticate(authSvc)

	// The QR links to the frontend, which is the first allowed CORS origin.
	publicBaseURL := ""
	if len(cfg.CORSAllowedOrigins) > 0 {
		publicBaseURL = cfg.CORSAllowedOrigins[0]
	}
	roomsHandler := rooms.NewHandler(rooms.NewService(pool, clk, types, publicBaseURL))
	schedulesHandler := schedules.NewHandler(schedules.NewService(pool, clk, types))
	availabilityHandler := availability.NewHandler(availability.NewService(pool, types))
	importsHandler := imports.NewHandler(imports.NewService(pool, hasher, clk, notifier, imports.Options{
		Limits:             imports.Limits{MaxBytes: cfg.Business.ImportMaxBytes, MaxRows: cfg.Business.ImportMaxRows},
		TempPasswordLength: cfg.Business.TempPasswordLength,
		MaxWeeks:           cfg.Business.ImportMaxWeeks,
	}, log))

	roomTypesHandler := roomtypes.NewHandler(roomtypes.NewService(pool, clk))
	files := attachments.NewService(pool, attachments.NewStorage(cfg.UploadDir), clk, attachments.Limits{
		MaxLetterBytes: cfg.Business.MaxLetterBytes,
		MaxPhotoBytes:  cfg.Business.MaxUploadBytes,
	}, types, log)
	attachmentsHandler := attachments.NewHandler(files)

	notificationsHandler := notifications.NewHandler(
		notifications.NewService(pool, clk),
		notifications.NewHub(ctx, pool.Config().ConnString(), log),
		// A stream ends before the access token it was opened with expires.
		notifications.StreamOptions{Heartbeat: cfg.SSEHeartbeat, MaxLifetime: cfg.JWTAccessExpiry, MaxPerUser: maxStreamsPerUser},
		log,
	)
	reportsSvc := reports.NewService(pool, clk, files, log, types, notifier)
	reportsHandler := reports.NewHandler(reportsSvc, cfg.Business.MaxUploadBytes)
	bookingsHandler := bookings.NewHandler(bookings.NewService(pool, clk, types, notifier, files, reportsSvc, bookingSettings(cfg)), cfg.Business.MaxUploadBytes)
	violationsHandler := violations.NewHandler(violations.NewService(pool, clk, cfg.Business.ViolationWindow))

	// One budget per user for the multipart uploads of letters and photos, and a larger one of its own for the
	// CSV imports (TU and administrators work in bulk; previews are free).
	uploadLimit := httpx.LimitUploads(ratelimit.New(cfg.Business.UploadsPerMinute, time.Minute, clk))
	importLimit := httpx.LimitUploads(ratelimit.New(cfg.Business.ImportsPerMinute, time.Minute, clk), httpx.SkipDryRun())

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(httpx.NoSniff)
	r.Use(httpx.WithClientInfo(cfg.TrustedProxies))
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
		r.With(authenticate, httpx.RequirePasswordChanged).Mount("/users", users.NewHandler(usersSvc).Routes())
		r.With(authenticate, httpx.RequirePasswordChanged, importLimit).Mount("/imports", importsHandler.Routes())
		r.With(authenticate, httpx.RequirePasswordChanged, httpx.RequireRoles(httpx.RoleAdmin)).Get("/audit-logs", audit.NewHandler(audit.NewService(pool)).List)
		r.Group(func(r chi.Router) {
			r.Use(authenticate, httpx.RequirePasswordChanged, uploadLimit)
			r.Route("/rooms", func(r chi.Router) {
				roomsHandler.Register(r)
				r.Get("/{id}/schedule", availabilityHandler.Timeline)
			})
			r.Route("/attachments", attachmentsHandler.Register)
			r.Route("/room-types", roomTypesHandler.Register)
			r.Route("/schedules", schedulesHandler.Register)
			r.Route("/bookings", bookingsHandler.Register)
			bookingsHandler.RegisterRoot(r)
			violationsHandler.Register(r)
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

// bookingSettings maps the business configuration (BR-11 to BR-28) to what the booking workflow reads.
func bookingSettings(cfg *config.Config) bookings.Settings {
	b := cfg.Business
	return bookings.Settings{
		PendingTTL: b.PendingTTL, EditCutoff: b.EditCutoff, LateCancel: b.LateCancel,
		MaxOccurrences: b.MaxOccurrences, AutoApproveMaxPoints: b.AutoApproveMaxPoints,
		MaxActiveBookings: b.MaxActiveBookings, MaxActiveBookingsOrg: b.MaxActiveBookingsOrg,
		PointsLateCancel: b.PointsLateCancel, CheckinEarly: b.CheckinEarly, CheckinLate: b.CheckinLate,
		Violations: violations.Settings{Window: b.ViolationWindow, Threshold: b.BlockThresholdPoints, BlockFor: b.BlockDuration},
	}
}

// NewJobRunner wires the scheduled jobs (BR-13) with the same business settings the router uses. The caller
// starts it with Run, so the jobs live exactly as long as the server.
func NewJobRunner(cfg *config.Config, pool *db.Pool, log *slog.Logger, clk clock.Clock) *jobs.Runner {
	b := cfg.Business
	files := attachments.NewService(pool, attachments.NewStorage(cfg.UploadDir), clk, attachments.Limits{
		MaxLetterBytes: b.MaxLetterBytes,
		MaxPhotoBytes:  b.MaxUploadBytes,
	}, roomtypes.NewStore(pool), log)
	return jobs.New(pool, clk, notifications.NewNotifier(notifications.InApp{}), files, jobs.Settings{
		CheckinEarly: b.CheckinEarly, CheckinLate: b.CheckinLate, CheckoutGrace: b.CheckoutGrace,
		ReminderCheckinLast: b.ReminderCheckinLast, ReminderCheckout: b.ReminderCheckout,
		PointsNoShow: b.PointsNoShow, PointsNoCheckout: b.PointsNoCheckout,
		Violations: violations.Settings{Window: b.ViolationWindow, Threshold: b.BlockThresholdPoints, BlockFor: b.BlockDuration},
		Purge:      attachments.PurgePolicy{OrphanAge: b.OrphanAttachmentAge, PhotoRetention: b.PhotoRetention, LetterRetention: b.LetterRetention},
	}, log)
}
