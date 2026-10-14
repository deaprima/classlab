package httpx_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"classlab/api/internal/httpx"
)

func TestParseTimeParam(t *testing.T) {
	want := time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC) // 08:00 WIB

	for name, query := range map[string]string{
		"escaped plus":      "t=2026-10-20T08%3A00%3A00%2B07%3A00",
		"bare plus":         "t=2026-10-20T08:00:00+07:00", // decodes to a space
		"utc":               "t=2026-10-20T01:00:00Z",
		"negative offset":   "t=2026-10-19T20:00:00-05:00",
		"fractional second": "t=2026-10-20T08:00:00.000+07:00",
	} {
		got, err := httpx.ParseTimeParam(httptest.NewRequest("GET", "/?"+query, nil), "t")
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !got.Equal(want) || got.Location() != time.UTC {
			t.Errorf("%s: got %v; want %v in UTC", name, got, want)
		}
	}

	for name, query := range map[string]string{
		"missing":        "x=1",
		"empty":          "t=",
		"no offset":      "t=2026-10-20T08:00:00",
		"date only":      "t=2026-10-20",
		"garbage":        "t=besok",
		"space not plus": "t=2026-10-20T08:00:00%20garbage",
	} {
		if _, err := httpx.ParseTimeParam(httptest.NewRequest("GET", "/?"+query, nil), "t"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParsePageBounds(t *testing.T) {
	for _, ok := range []string{"", "page=1", "page=10000&per_page=100", "per_page=1"} {
		if _, _, err := httpx.ParsePage(httptest.NewRequest("GET", "/?"+ok, nil)); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	// (page-1)*per_page must fit the int32 OFFSET of the queries: a page beyond the cap used to turn
	// into a negative offset and a 500.
	for _, bad := range []string{
		"page=0", "page=-1", "page=x", "page=10001", "page=2147483648", "page=999999999999",
		"page=99999999999999999999", "per_page=0", "per_page=101", "per_page=x",
	} {
		if _, _, err := httpx.ParsePage(httptest.NewRequest("GET", "/?"+bad, nil)); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	page, per, err := httpx.ParsePage(httptest.NewRequest("GET", "/?page=10000&per_page=100", nil))
	if err != nil || (page-1)*per > 1<<31-1 {
		t.Errorf("the largest allowed page overflows int32: %d*%d (%v)", page, per, err)
	}
}
