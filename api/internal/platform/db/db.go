package db

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool struct {
	*pgxpool.Pool
}

// New creates a new PostgreSQL connection pool. lockTimeout bounds how long any statement waits for a
// row lock: without it one stuck transaction would queue every booking for that room behind it until
// the pool of connections is used up. A statement that gives up fails with SQLSTATE 55P03.
func New(ctx context.Context, connString string, lockTimeout time.Duration) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parsing db config: %w", err)
	}

	cfg.MaxConns = 25
	cfg.ConnConfig.RuntimeParams["lock_timeout"] = strconv.FormatInt(lockTimeout.Milliseconds(), 10) // milliseconds
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 1 * time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating db pool: %w", err)
	}

	// Verify connectivity
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging db: %w", err)
	}

	return &Pool{Pool: pool}, nil
}

// WithTx executes the provided fn inside a database transaction.
func (p *Pool) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning tx: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing tx: %w", err)
	}

	return nil
}
