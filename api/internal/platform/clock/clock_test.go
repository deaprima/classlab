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

func TestShiftedClockStartsAtTheGivenInstantAndKeepsRunning(t *testing.T) {
	start := time.Date(2026, 10, 26, 1, 50, 0, 0, time.UTC)
	clk := clock.NewShiftedClock(start)

	first := clk.Now()
	if d := first.Sub(start); d < 0 || d > time.Second {
		t.Fatalf("first reading %v is %v away from the start %v", first, d, start)
	}
	time.Sleep(20 * time.Millisecond)
	if !clk.Now().After(first) {
		t.Fatal("shifted clock did not advance")
	}
	if clk.Now().Location() != time.UTC {
		t.Fatal("shifted clock must report UTC like RealClock")
	}
}

func TestRealClock(t *testing.T) {
	clk := clock.NewRealClock()
	now := clk.Now()
	if now.IsZero() {
		t.Fatalf("expected non-zero time")
	}
}
