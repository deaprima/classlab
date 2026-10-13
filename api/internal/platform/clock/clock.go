package clock

import (
	"sync"
	"time"
)

// Clock provides an interface to access current time, allowing deterministic mocking in tests.
type Clock interface {
	Now() time.Time
}

// RealClock returns the current UTC wall clock time.
type RealClock struct{}

func NewRealClock() Clock {
	return &RealClock{}
}

func (RealClock) Now() time.Time {
	return time.Now().UTC()
}

// MockClock is a controllable clock for testing.
type MockClock struct {
	mu  sync.RWMutex
	now time.Time
}

func NewMockClock(t time.Time) *MockClock {
	return &MockClock{now: t.UTC()}
}

func (m *MockClock) Now() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.now
}

func (m *MockClock) Set(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = t.UTC()
}

func (m *MockClock) Add(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = m.now.Add(d)
}
