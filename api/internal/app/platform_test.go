package app_test

import (
	"net/http"
	"strings"
	"testing"

	"classlab/api/internal/platform/password"
	"classlab/api/internal/seed"
)

func TestErrorEnvelopeCarriesRequestID(t *testing.T) {
	e := newEnv(t)

	res := e.do("GET", "/api/v1/auth/me", nil)
	errBody, _ := res.json()["error"].(map[string]any)
	reqID, _ := errBody["request_id"].(string)
	if !strings.HasPrefix(reqID, "req_") {
		t.Fatalf("request_id = %q; want a generated req_ id", reqID)
	}
	if reqID != res.Header().Get("X-Request-Id") {
		t.Errorf("body request_id %q differs from X-Request-Id header %q", reqID, res.Header().Get("X-Request-Id"))
	}
	if details, ok := errBody["details"].([]any); !ok || details == nil {
		t.Errorf("details must be an array, got %v", errBody["details"])
	}
}

func TestUnknownRouteAndMethodUseErrorEnvelope(t *testing.T) {
	e := newEnv(t)

	if res := e.do("GET", "/api/v1/nope", nil); res.Code != http.StatusNotFound || res.errorCode() != "NOT_FOUND" {
		t.Errorf("unknown route: %d %s", res.Code, res.errorCode())
	}
	if res := e.do("DELETE", "/api/v1/auth/login", nil); res.Code != http.StatusMethodNotAllowed || res.errorCode() == "" {
		t.Errorf("wrong method: %d %s", res.Code, res.errorCode())
	}
}

func TestHealth(t *testing.T) {
	e := newEnv(t)
	res := e.do("GET", "/health", nil)
	if res.Code != http.StatusOK || res.json()["status"] != "healthy" || res.json()["database"] != "ok" {
		t.Errorf("health: %d %s", res.Code, res.Body.String())
	}
}

func TestSeedIsIdempotentAndGuardsProduction(t *testing.T) {
	e := newEnv(t) // newEnv already seeded once
	hasher := password.NewHasher(8, 1, 1)

	res, err := seed.Run(t.Context(), e.pool, hasher, e.clock)
	if err != nil {
		t.Fatal(err)
	}
	if res != (seed.Result{}) {
		t.Errorf("second seed run inserted %+v; want nothing", res)
	}
	var users, units int
	if err := e.pool.QueryRow(t.Context(), "SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM org_units)").Scan(&users, &units); err != nil {
		t.Fatal(err)
	}
	if users != 8 || units != 1 {
		t.Errorf("after two runs: %d users, %d org units; want 8 and 1", users, units)
	}

	// Six rooms (3 KELAS, 3 LAB) and a 12-class weekly timetable over 16 weeks, owned by TU/Kalab.
	var kelas, lab, schedules, byOthers int
	if err := e.pool.QueryRow(t.Context(), `SELECT
		(SELECT count(*) FROM rooms WHERE type = 'KELAS'),
		(SELECT count(*) FROM rooms WHERE type = 'LAB'),
		(SELECT count(*) FROM room_schedules WHERE source = 'AKADEMIK'),
		(SELECT count(*) FROM room_schedules WHERE created_by <> $1)`, e.userIDOf(tuKalabID)).Scan(&kelas, &lab, &schedules, &byOthers); err != nil {
		t.Fatal(err)
	}
	if kelas != 3 || lab != 3 || schedules != 12*16 || byOthers != 0 {
		t.Errorf("rooms: %d KELAS, %d LAB; schedules: %d (others' %d); want 3, 3, 192, 0", kelas, lab, schedules, byOthers)
	}

	// One account per role plus five students, as specified.
	rows, err := e.pool.Query(t.Context(), "SELECT role, count(*) FROM users GROUP BY role")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	perRole := map[string]int{}
	for rows.Next() {
		var role string
		var n int
		if err := rows.Scan(&role, &n); err != nil {
			t.Fatal(err)
		}
		perRole[role] = n
	}
	if perRole["KAPRODI"] != 1 || perRole["TU_KALAB"] != 1 || perRole["OB"] != 1 || perRole["MAHASISWA"] != 5 {
		t.Errorf("role distribution = %v", perRole)
	}

	// The production guard trips while dev accounts are active and clears once they are disabled.
	if err := seed.AssertNoDevAccounts(t.Context(), e.pool); err == nil {
		t.Error("AssertNoDevAccounts should fail while dev accounts are active")
	}
	if _, err := e.pool.Exec(t.Context(), "UPDATE users SET is_active = false"); err != nil {
		t.Fatal(err)
	}
	if err := seed.AssertNoDevAccounts(t.Context(), e.pool); err != nil {
		t.Errorf("AssertNoDevAccounts after deactivation: %v", err)
	}
}
