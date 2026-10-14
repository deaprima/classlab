// Package testdb gives tests a real PostgreSQL (via testcontainers) with the project's migrations applied.
// Constraints, triggers and locking are the point of several tests, so a fake database would prove nothing.
package testdb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"classlab/api/internal/platform/db"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const templateDB = "classlab_template"

var (
	adminURL string // connection string for the "postgres" maintenance database
	baseURL  string // connection string prefix; the database name is appended
	counter  atomic.Int64
)

// Run starts the container, executes the package's tests, and tears the container down.
// Call it from TestMain: os.Exit(testdb.Run(m)).
func Run(m *testing.M) int {
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("postgres"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdb: starting postgres container (is Docker running?): %v\n", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			fmt.Fprintf(os.Stderr, "testdb: terminating container: %v\n", err)
		}
	}()

	adminURL, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdb: connection string: %v\n", err)
		return 1
	}
	baseURL = adminURL[:len(adminURL)-len("postgres?sslmode=disable")]

	if err := buildTemplate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "testdb: building template database: %v\n", err)
		return 1
	}

	return m.Run()
}

// buildTemplate migrates once; each test then clones it, which is much faster than re-running DDL.
func buildTemplate(ctx context.Context) error {
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return fmt.Errorf("connecting as admin: %w", err)
	}
	defer admin.Close(ctx)

	if _, err := admin.Exec(ctx, "CREATE DATABASE "+templateDB); err != nil {
		return fmt.Errorf("creating template database: %w", err)
	}

	conn, err := pgx.Connect(ctx, baseURL+templateDB+"?sslmode=disable")
	if err != nil {
		return fmt.Errorf("connecting to template database: %w", err)
	}
	defer conn.Close(ctx) // must be closed before cloning: Postgres refuses to copy a database in use

	files, err := filepath.Glob(filepath.Join(migrationsDir(), "*.up.sql"))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no migrations found in %s: %v", migrationsDir(), err)
	}
	sort.Strings(files)
	for _, f := range files {
		sqlText, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("reading %s: %w", f, err)
		}
		if _, err := conn.Exec(ctx, string(sqlText)); err != nil {
			return fmt.Errorf("applying %s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations")
}

// New returns a pool on a fresh, fully migrated database that is dropped when the test ends.
func New(t testing.TB) *db.Pool {
	t.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("t_%d_%d", os.Getpid(), counter.Add(1))

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("testdb: connecting as admin: %v", err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, templateDB)); err != nil {
		t.Fatalf("testdb: cloning template: %v", err)
	}

	pool, err := db.New(ctx, baseURL+name+"?sslmode=disable")
	if err != nil {
		t.Fatalf("testdb: opening pool: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		cleanup, err := pgx.Connect(ctx, adminURL)
		if err != nil {
			t.Errorf("testdb: reconnecting for cleanup: %v", err)
			return
		}
		defer cleanup.Close(ctx)
		if _, err := cleanup.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name)); err != nil {
			t.Errorf("testdb: dropping %s: %v", name, err)
		}
	})
	return pool
}
