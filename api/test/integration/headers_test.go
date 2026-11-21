package app_test

import (
	"testing"
)

// Every response carries nosniff, not just the file downloads: an error body or a JSON list must not be
// something a browser may reinterpret.
func TestResponsesAreNoSniff(t *testing.T) {
	e := newEnv(t)
	for name, res := range map[string]response{
		"health":            e.do("GET", "/health", nil),
		"unauthenticated":   e.doUnchecked("GET", "/api/v1/rooms", nil),
		"unknown route":     e.doUnchecked("GET", "/api/v1/nope", nil),
		"authenticated api": e.do("GET", "/api/v1/rooms", nil, bearer(e.adminToken())),
	} {
		if got := res.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s (status %d): X-Content-Type-Options = %q, want nosniff", name, res.Code, got)
		}
	}
}
