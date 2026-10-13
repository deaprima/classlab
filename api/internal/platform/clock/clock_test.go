package clock_test

import (
	"testing"
	"time"

	"classlab/api/internal/platform/clock"
)

func TestMockClock(t *testing.T) {
	initial := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	clk := clock.NewMockClock(initial)

	if !clk.Now().Equal(initial) {
		t.Fatalf("expected %v, got %v", initial, clk.Now())
	}

	clk.Add(30 * time.Minute)
	expectedAfterAdd := time.Date(2026, 10, 20, 8, 30, 0, 0, time.UTC)
	if !clk.Now().Equal(expectedAfterAdd) {
		t.Fatalf("expected %v, got %v", expectedAfterAdd, clk.Now())
	}

	newTime := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	clk.Set(newTime)
	if !clk.Now().Equal(newTime) {
		t.Fatalf("expected %v, got %v", newTime, clk.Now())
	}
}

func TestRealClock(t *testing.T) {
	clk := clock.NewRealClock()
	now := clk.Now()
	if now.IsZero() {
		t.Fatalf("expected non-zero time")
	}
}
