package ratelimit

import (
	"sync"
	"time"

	"classlab/api/internal/platform/clock"
)

// Limiter allows at most `max` attempts per key inside a sliding window (login guesses, uploads).
// State is in memory: it resets on restart and is not shared between instances, which is
// acceptable for the single-instance deployment this MVP targets.
type Limiter struct {
	mu        sync.Mutex
	max       int
	window    time.Duration
	clock     clock.Clock
	attempts  map[string][]time.Time
	lastSweep time.Time
}

func New(max int, window time.Duration, clk clock.Clock) *Limiter {
	return &Limiter{max: max, window: window, clock: clk, attempts: make(map[string][]time.Time), lastSweep: clk.Now()}
}

// Allow records an attempt and reports whether it may proceed. The attempt is counted up front
// (not after the password check) so parallel guesses cannot all slip past a not-yet-updated counter.
func (l *Limiter) Allow(key string) (retryAfter time.Duration, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	l.sweep(now)

	recent := l.prune(key, now)
	if len(recent) >= l.max {
		return recent[0].Add(l.window).Sub(now), false
	}
	l.attempts[key] = append(recent, now)
	return 0, true
}

// Reset forgets earlier attempts for a key, e.g. after a successful login.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func (l *Limiter) prune(key string, now time.Time) []time.Time {
	cutoff := now.Add(-l.window)
	list := l.attempts[key]
	i := 0
	for i < len(list) && !list[i].After(cutoff) {
		i++
	}
	list = list[i:]
	if len(list) == 0 {
		delete(l.attempts, key)
	}
	return list
}

// sweep drops keys nobody has touched for a full window so the map cannot grow without bound.
func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for key := range l.attempts {
		l.prune(key, now)
	}
}
