package auth

import (
	"time"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/ratelimit"
)

// LoginLimiter allows at most `max` login attempts per key inside a sliding window. The same limiter
// also guards uploads (httpx.LimitUploads).
type LoginLimiter = ratelimit.Limiter

func NewLoginLimiter(max int, window time.Duration, clk clock.Clock) *LoginLimiter {
	return ratelimit.New(max, window, clk)
}
