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

	"classlab/api/internal/app"
	"classlab/api/internal/config"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/logger"
	"classlab/api/internal/seed"
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

	// Dev seed accounts share a published password; they must never be live in production.
	if cfg.AppEnv == "production" {
		if err := seed.AssertNoDevAccounts(ctx, pool); err != nil {
			appLogger.Error("refusing to start", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      app.NewRouter(cfg, pool, appLogger, clock.NewRealClock()),
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
