// Command createadmin makes the first administrator of a new database. Run it once after the
// migrations (deploy/README.md); the temporary password is printed once and must be changed at
// the first sign-in.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"classlab/api/internal/bootstrap"
	"classlab/api/internal/config"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/password"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "createadmin: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var in bootstrap.Admin
	flag.StringVar(&in.Identifier, "identifier", "", "NIP or login name of the administrator (required)")
	flag.StringVar(&in.Name, "name", "", "full name (required)")
	flag.StringVar(&in.Email, "email", "", "email address (optional)")
	flag.StringVar(&in.OrgUnit, "org", bootstrap.DefaultOrgUnit, "organisation unit; created when missing")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	ctx := context.Background()
	pool, err := db.New(ctx, cfg.DatabaseURL, cfg.DBLockTimeout)
	if err != nil {
		return err
	}
	defer pool.Close()

	hasher := password.NewHasher(cfg.Argon2MemoryKiB, cfg.Argon2Iterations, cfg.Argon2Parallelism)
	temp, err := bootstrap.CreateAdmin(ctx, pool, hasher, clock.NewRealClock(), in, cfg.Business.TempPasswordLength)
	if err != nil {
		var invalid *bootstrap.ValidationError
		switch {
		case errors.As(err, &invalid):
			flag.Usage()
			return err
		case errors.Is(err, bootstrap.ErrExists):
			return fmt.Errorf("%s already exists; nothing was changed", in.Identifier)
		}
		return err
	}

	// The only place the password is ever shown. It is not logged and not stored in plain text.
	fmt.Printf("Administrator %s created.\nTemporary password (shown once): %s\nIt must be changed at the first sign-in.\n", in.Identifier, temp)
	return nil
}
