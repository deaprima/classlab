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

// ShiftedClock runs at real speed from a chosen starting instant. It exists for local end-to-end runs that
// need a fixed time of day (DEV_CLOCK_START); production always uses RealClock.
type ShiftedClock struct {
	offset time.Duration
}

func NewShiftedClock(start time.Time) *ShiftedClock {
	return &ShiftedClock{offset: time.Until(start)}
}

func (s *ShiftedClock) Now() time.Time {
	return time.Now().Add(s.offset).UTC()
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

// WIB is the display zone for API responses; storage and arithmetic stay in UTC.
// A fixed zone avoids depending on tzdata being installed (e.g. on Windows or scratch images).
var WIB = time.FixedZone("WIB", 7*60*60)

func ToWIB(t time.Time) time.Time {
	return t.In(WIB)
}
