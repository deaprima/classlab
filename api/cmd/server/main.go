package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"classlab/api/internal/config"
	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/logger"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	appLogger := logger.New(cfg.LogLevel)
	slog.SetDefault(appLogger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appLogger.Info("connecting to postgresql database...")
	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		appLogger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()
	appLogger.Info("database connection established successfully")

	r := chi.NewRouter()

	// Global Middlewares
	r.Use(httpx.RequestID)
	r.Use(httpx.RequestLogger(appLogger))
	r.Use(middleware.Recoverer)

	// CORS Setup
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-Id", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link", "X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoint (Liveness + DB Ping)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, pingCancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer pingCancel()

		dbStatus := "ok"
		if err := pool.Ping(pingCtx); err != nil {
			dbStatus = "error: " + err.Error()
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]any{
				"status":   "unhealthy",
				"database": dbStatus,
				"time":     time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		httpx.JSON(w, http.StatusOK, map[string]any{
			"status":   "healthy",
			"database": dbStatus,
			"version":  "0.1.0",
			"time":     time.Now().UTC().Format(time.RFC3339),
		})
	})

	// API v1 router group
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			httpx.JSON(w, http.StatusOK, map[string]string{
				"message": "ClassLab API v1",
			})
		})
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		appLogger.Info("starting classlab api server", slog.String("port", cfg.Port), slog.String("env", cfg.AppEnv))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-shutdownChan
	appLogger.Info("shutting down server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		appLogger.Error("server forced to shutdown", slog.String("error", err.Error()))
	}

	appLogger.Info("server exited cleanly")
}
