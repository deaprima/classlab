package main

import (
	"context"
	"fmt"
	"os"

	"classlab/api/internal/config"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/password"
	"classlab/api/internal/seed"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	if cfg.AppEnv == "production" {
		return fmt.Errorf("refusing to seed dev accounts when APP_ENV=production")
	}

	ctx := context.Background()
	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	hasher := password.NewHasher(cfg.Argon2MemoryKiB, cfg.Argon2Iterations, cfg.Argon2Parallelism)
	created, err := seed.Run(ctx, pool, hasher, clock.NewRealClock())
	if err != nil {
		return err
	}

	fmt.Printf("Seed done: %d new account(s). Password for all: %s\n", created, seed.DevPassword)
	for _, line := range seed.Accounts() {
		fmt.Println("  " + line)
	}
	return nil
}
