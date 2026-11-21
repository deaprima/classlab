package integration_test

import (
	"net/http"
	"testing"
	"time"

	"classlab/api/internal/config"
)

// One user gets UploadsPerMinute files a minute across every upload route; the next one is turned away
// before its body is read, and nobody else is held back.
func TestUploadsAreRateLimitedPerUser(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Business.UploadsPerMinute = 2 })
	a := e.login(mahasiswaA).accessToken
	b := e.login(mahasiswaB).accessToken

	for i := 1; i <= 2; i++ {
		mustStatus(t, e.uploadLetter(a, "LETTER", pdfBytes()), http.StatusCreated)
	}
	stored := len(e.uploadedFiles())

	over := e.uploadLetter(a, "LETTER", pdfBytes())
	mustStatus(t, over, http.StatusTooManyRequests)
	if over.errorCode() != "RATE_LIMITED" {
		t.Fatalf("error code = %q, want RATE_LIMITED", over.errorCode())
	}
	if over.Header().Get("Retry-After") == "" {
		t.Fatal("429 without Retry-After")
	}
	if n := len(e.uploadedFiles()); n != stored {
		t.Fatalf("a rejected upload left %d new file(s) behind", n-stored)
	}

	// Another student, and the same student's ordinary requests, are not affected.
	mustStatus(t, e.uploadLetter(b, "LETTER", pdfBytes()), http.StatusCreated)
	mustStatus(t, e.do("GET", "/api/v1/attachments", nil, bearer(a)), http.StatusOK)

	e.clock.Add(time.Minute + time.Second)
	mustStatus(t, e.uploadLetter(a, "LETTER", pdfBytes()), http.StatusCreated)
}

// Imports have a budget of their own, previews are free, and the free preview is not a way around the limit
// of the routes that do not know about it.
func TestImportsHaveTheirOwnBudgetAndPreviewsAreFree(t *testing.T) {
	e := newEnv(t, func(c *config.Config) {
		c.Business.ImportsPerMinute = 2
		c.Business.UploadsPerMinute = 1
	})
	tu := e.tuToken()

	for i := 0; i < 5; i++ {
		mustStatus(t, e.importPreview(tu, importCSV, importSemester), http.StatusOK)
	}
	for i := 1; i <= 2; i++ {
		if res := e.importApply(tu, importCSV, importSemester, "bukan-hash"); res.Code == http.StatusTooManyRequests {
			t.Fatalf("apply %d was limited; the budget is 2", i)
		}
	}
	mustStatus(t, e.importApply(tu, importCSV, importSemester, "bukan-hash"), http.StatusTooManyRequests)

	// The imports budget is separate from the letters-and-photos one: TU's imports used none of it.
	student := e.login(mahasiswaA).accessToken
	mustStatus(t, e.uploadLetter(student, "LETTER", pdfBytes()), http.StatusCreated)
	// ...and ?dry_run=true on a route that is not an import is counted like any other upload.
	res := e.multipartPost("/api/v1/attachments?dry_run=true", student, map[string]string{"kind": "LETTER"}, pdfBytes())
	mustStatus(t, res, http.StatusTooManyRequests)
}
