package ratelimit

import (
	"testing"
	"time"

	"classlab/api/internal/platform/clock"
)

var epoch = time.Date(2026, 10, 14, 3, 0, 0, 0, time.UTC)

func TestLimiterBlocksSixthAttempt(t *testing.T) {
	clk := clock.NewMockClock(epoch)
	l := New(5, 15*time.Minute, clk)

	for i := 1; i <= 5; i++ {
		if _, ok := l.Allow("a|1.1.1.1"); !ok {
			t.Fatalf("attempt %d should be allowed", i)
		}
		clk.Add(time.Second)
	}
	retry, ok := l.Allow("a|1.1.1.1")
	if ok {
		t.Fatal("6th attempt within the window must be blocked")
	}
	// Oldest attempt was 5s ago, so it frees up in 15m - 5s.
	if want := 15*time.Minute - 5*time.Second; retry != want {
		t.Fatalf("retryAfter = %v; want %v", retry, want)
	}
}

func TestLimiterWindowSlides(t *testing.T) {
	clk := clock.NewMockClock(epoch)
	l := New(2, 15*time.Minute, clk)

	l.Allow("k")
	clk.Add(10 * time.Minute)
	l.Allow("k")
	if _, ok := l.Allow("k"); ok {
		t.Fatal("third attempt should be blocked")
	}

	// The first attempt leaves the window at +15m; the second is still inside it.
	clk.Add(5 * time.Minute)
	if _, ok := l.Allow("k"); !ok {
		t.Fatal("one slot should be free once the oldest attempt ages out")
	}
	if _, ok := l.Allow("k"); ok {
		t.Fatal("window must still hold two attempts")
	}
}

func TestLimiterKeysAreIndependent(t *testing.T) {
	l := New(1, time.Minute, clock.NewMockClock(epoch))
	if _, ok := l.Allow("victim|1.1.1.1"); !ok {
		t.Fatal("first attempt blocked")
	}
	if _, ok := l.Allow("victim|2.2.2.2"); !ok {
		t.Fatal("a different IP must have its own budget")
	}
	if _, ok := l.Allow("other|1.1.1.1"); !ok {
		t.Fatal("a different identifier must have its own budget")
	}
}

func TestLimiterReset(t *testing.T) {
	l := New(1, time.Minute, clock.NewMockClock(epoch))
	l.Allow("k")
	l.Reset("k")
	if _, ok := l.Allow("k"); !ok {
		t.Fatal("Reset must clear the budget")
	}
}

func TestLimiterSweepsIdleKeys(t *testing.T) {
	clk := clock.NewMockClock(epoch)
	l := New(5, time.Minute, clk)
	for i := 0; i < 100; i++ {
		l.Allow(string(rune('a'+i%26)) + string(rune('A'+i/26)))
	}
	clk.Add(2 * time.Minute)
	l.Allow("trigger")

	if n := len(l.attempts); n != 1 {
		t.Fatalf("expected idle keys to be swept, %d keys remain", n)
	}
}
