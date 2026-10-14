package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"classlab/api/internal/app"
	"classlab/api/internal/auth"
	"classlab/api/internal/config"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/password"
	"classlab/api/internal/platform/testdb"
	"classlab/api/internal/seed"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m))
}

// Seed account identifiers (see internal/seed). All share seed.DevPassword.
const (
	kaprodiID  = "197501012000011001"
	tuKalabID  = "198203152008012002"
	obID       = "199001102015011003"
	mahasiswaA = "2021101001"
	mahasiswaB = "2021101002"
)

var startTime = time.Date(2026, 10, 14, 3, 0, 0, 0, time.UTC)

type env struct {
	t      *testing.T
	pool   *db.Pool
	clock  *clock.MockClock
	cfg    *config.Config
	router http.Handler
	logs   *syncBuffer // everything the server logged, for tests that check what must not be logged
}

// syncBuffer is a bytes.Buffer safe for the server goroutines that log into it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newEnv(t *testing.T, mutate ...func(*config.Config)) *env {
	t.Helper()
	pool := testdb.New(t)
	clk := clock.NewMockClock(startTime)

	cfg := &config.Config{
		AppEnv:             "test",
		JWTSecret:          "0123456789abcdef0123456789abcdef",
		JWTAccessExpiry:    15 * time.Minute,
		RefreshTokenExpiry: 7 * 24 * time.Hour,
		Argon2MemoryKiB:    8,
		Argon2Iterations:   1,
		Argon2Parallelism:  1,
		CORSAllowedOrigins: []string{"http://localhost:5173"},

		// BR-01 defaults, as in .env.example.
		OperatingHoursStart:       "07:00",
		OperatingHoursEnd:         "21:00",
		BookingMinDurationMinutes: 30,
		BookingMaxDurationHours:   8,
		BookingMaxFutureDays:      30,
		// High on purpose: most tests make many bookings for one student. Quota tests set it to 5, the real default.
		BookingMaxActivePerUser: 100,

		UploadDir:      t.TempDir(),
		UploadMaxBytes: 5 << 20,
		SSEHeartbeat:   25 * time.Second,
	}
	for _, fn := range mutate {
		fn(cfg)
	}

	hasher := password.NewHasher(cfg.Argon2MemoryKiB, cfg.Argon2Iterations, cfg.Argon2Parallelism)
	if _, err := seed.Run(context.Background(), pool, hasher, clk); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	logs := &syncBuffer{}
	var logOut io.Writer = logs
	if os.Getenv("TEST_LOG") != "" { // set TEST_LOG=1 to see server logs while debugging a failing test
		logOut = io.MultiWriter(logs, os.Stderr)
	}
	log := slog.New(slog.NewTextHandler(logOut, nil))
	router, err := app.NewRouter(t.Context(), cfg, pool, log, clk)
	if err != nil {
		t.Fatalf("building router: %v", err)
	}
	return &env{t: t, pool: pool, clock: clk, cfg: cfg, router: router, logs: logs}
}

type reqOpt func(*http.Request)

func bearer(token string) reqOpt {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

func refreshCookie(value string) reqOpt {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: auth.RefreshCookieName, Value: value}) }
}

func fromIP(ip string) reqOpt {
	return func(r *http.Request) { r.RemoteAddr = ip + ":50000" }
}

type response struct {
	*httptest.ResponseRecorder
}

func (r response) json() map[string]any {
	var out map[string]any
	if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
		panic("response is not a JSON object: " + r.Body.String())
	}
	return out
}

// errorCode returns error.code, or "" when the body is not the standard error envelope.
func (r response) errorCode() string {
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &env); err != nil {
		return ""
	}
	return env.Error.Code
}

func (r response) cookie(name string) *http.Cookie {
	for _, c := range r.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func (e *env) do(method, path string, body any, opts ...reqOpt) response {
	e.t.Helper()
	var raw []byte
	switch b := body.(type) {
	case nil:
	case string:
		raw = []byte(b)
	default:
		var err error
		if raw, err = json.Marshal(b); err != nil {
			e.t.Fatalf("marshalling request body: %v", err)
		}
	}

	var reader io.Reader
	if raw != nil {
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.RemoteAddr = "192.0.2.10:50000"
	if raw != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, o := range opts {
		o(req)
	}

	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	e.checkContract(req, raw, rec)
	return response{rec}
}

type session struct {
	accessToken string
	refresh     string
	userID      string
}

// login signs in a seed account and fails the test if it does not succeed.
func (e *env) login(identifier string, opts ...reqOpt) session {
	e.t.Helper()
	res := e.do("POST", "/api/v1/auth/login", map[string]string{"identifier": identifier, "password": seed.DevPassword}, opts...)
	if res.Code != http.StatusOK {
		e.t.Fatalf("login %s: status %d, body %s", identifier, res.Code, res.Body.String())
	}
	body := res.json()
	c := res.cookie(auth.RefreshCookieName)
	if c == nil {
		e.t.Fatalf("login %s: no refresh cookie", identifier)
	}
	user, _ := body["user"].(map[string]any)
	id, _ := user["id"].(string)
	token, _ := body["access_token"].(string)
	return session{accessToken: token, refresh: c.Value, userID: id}
}

// doUnchecked sends a request without validating the response against openapi.yaml. It is for tests
// that provoke a 500, which the contract deliberately does not describe.
func (e *env) doUnchecked(method, path string, body any, opts ...reqOpt) response {
	e.t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		e.t.Fatalf("marshalling request body: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.RemoteAddr = "192.0.2.10:50000"
	req.Header.Set("Content-Type", "application/json")
	for _, o := range opts {
		o(req)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return response{rec}
}
