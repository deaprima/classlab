// Package maintenance holds housekeeping that keeps tables from growing without bound.
package maintenance

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"
)

// tokenGrace keeps an expired refresh token around for a day after it expires, so a replay of a
// just-expired token is still recognised as such in the logs instead of as an unknown token.
const tokenGrace = 24 * time.Hour

// PurgeExpiredRefreshTokens deletes refresh tokens that expired more than tokenGrace ago and returns
// how many it removed. Every refresh adds a row, so without this the table only grows.
func PurgeExpiredRefreshTokens(ctx context.Context, pool *db.Pool, clk clock.Clock) (int64, error) {
	n, err := dbgen.New(pool).DeleteExpiredRefreshTokens(ctx, clk.Now().Add(-tokenGrace))
	if err != nil {
		return 0, fmt.Errorf("purging expired refresh tokens: %w", err)
	}
	return n, nil
}

// Run purges once at start and then every interval until ctx ends. A failed run is logged and
// retried at the next tick: housekeeping must never take the server down.
func Run(ctx context.Context, pool *db.Pool, clk clock.Clock, log *slog.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		n, err := PurgeExpiredRefreshTokens(ctx, pool, clk)
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			log.ErrorContext(ctx, "housekeeping failed", slog.String("error", err.Error()))
		case n > 0:
			log.InfoContext(ctx, "purged expired refresh tokens", slog.Int64("count", n))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
