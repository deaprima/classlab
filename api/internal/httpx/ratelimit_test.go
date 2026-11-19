package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/ratelimit"

	"github.com/google/uuid"
)

func TestLimitUploads(t *testing.T) {
	clk := clock.NewMockClock(time.Date(2026, 10, 14, 3, 0, 0, 0, time.UTC))
	handler := httpx.LimitUploads(ratelimit.New(2, time.Minute, clk))(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }),
	)
	alice, bob := uuid.New(), uuid.New()

	send := func(user uuid.UUID, method, contentType string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/x", strings.NewReader(""))
		req.Header.Set("Content-Type", contentType)
		req = req.WithContext(httpx.WithPrincipal(req.Context(), httpx.Principal{UserID: user}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	const multipart = "multipart/form-data; boundary=xyz"

	for i := 1; i <= 2; i++ {
		if rec := send(alice, http.MethodPost, multipart); rec.Code != http.StatusNoContent {
			t.Fatalf("upload %d: status %d, want 204", i, rec.Code)
		}
	}

	over := send(alice, http.MethodPost, multipart)
	if over.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd upload: status %d, want 429", over.Code)
	}
	if over.Header().Get("Retry-After") != "61" {
		t.Fatalf("Retry-After = %q, want 61", over.Header().Get("Retry-After"))
	}
	if !strings.Contains(over.Body.String(), `"RATE_LIMITED"`) {
		t.Fatalf("body %s lacks the RATE_LIMITED code", over.Body.String())
	}

	// Other users, other methods and JSON bodies are not held back by Alice's uploads.
	if rec := send(bob, http.MethodPost, multipart); rec.Code != http.StatusNoContent {
		t.Fatalf("another user: status %d, want 204", rec.Code)
	}
	if rec := send(alice, http.MethodPost, "application/json"); rec.Code != http.StatusNoContent {
		t.Fatalf("JSON POST: status %d, want 204", rec.Code)
	}
	if rec := send(alice, http.MethodGet, multipart); rec.Code != http.StatusNoContent {
		t.Fatalf("GET: status %d, want 204", rec.Code)
	}

	// The window slides: a minute later Alice may upload again.
	clk.Add(time.Minute + time.Second)
	if rec := send(alice, http.MethodPost, multipart); rec.Code != http.StatusNoContent {
		t.Fatalf("after the window: status %d, want 204", rec.Code)
	}
}

func TestLimitUploadsDryRun(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	user := uuid.New()

	post := func(h http.Handler, target string) int {
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(""))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=xyz")
		req = req.WithContext(httpx.WithPrincipal(req.Context(), httpx.Principal{UserID: user}))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	newClock := func() *clock.MockClock { return clock.NewMockClock(time.Date(2026, 10, 14, 3, 0, 0, 0, time.UTC)) }

	// With the option, previews are free and only the real request is counted.
	withSkip := httpx.LimitUploads(ratelimit.New(1, time.Minute, newClock()), httpx.SkipDryRun())(next)
	for i := 0; i < 5; i++ {
		if code := post(withSkip, "/x?dry_run=true"); code != http.StatusNoContent {
			t.Fatalf("preview %d: status %d, want 204", i+1, code)
		}
	}
	if code := post(withSkip, "/x"); code != http.StatusNoContent {
		t.Fatalf("first real request: status %d, want 204", code)
	}
	if code := post(withSkip, "/x"); code != http.StatusTooManyRequests {
		t.Fatalf("second real request: status %d, want 429", code)
	}

	// Without it, ?dry_run=true must not be a way around the limit.
	plain := httpx.LimitUploads(ratelimit.New(1, time.Minute, newClock()))(next)
	if code := post(plain, "/x?dry_run=true"); code != http.StatusNoContent {
		t.Fatalf("first request: status %d, want 204", code)
	}
	if code := post(plain, "/x?dry_run=true"); code != http.StatusTooManyRequests {
		t.Fatalf("dry_run without the option: status %d, want 429", code)
	}
}
