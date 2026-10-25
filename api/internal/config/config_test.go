package config

import (
	"testing"
	"time"
)

func TestParseDevClockStart(t *testing.T) {
	if got, err := parseDevClockStart("", "production"); err != nil || !got.IsZero() {
		t.Errorf("unset: %v, %v; want zero and no error", got, err)
	}
	got, err := parseDevClockStart("2026-10-26T08:50:00+07:00", "development")
	if err != nil || !got.Equal(time.Date(2026, 10, 26, 1, 50, 0, 0, time.UTC)) {
		t.Errorf("valid value: %v, %v", got, err)
	}
	if _, err := parseDevClockStart("2026-10-26T08:50:00+07:00", "production"); err == nil {
		t.Error("accepted a shifted clock in production")
	}
	for _, bad := range []string{"2026-10-26 08:50", "besok", "2026-10-26"} {
		if _, err := parseDevClockStart(bad, "development"); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestValidateCORSOrigins(t *testing.T) {
	for _, ok := range [][]string{
		{"http://localhost:5173"},
		{"https://classlab.kampus.ac.id", "http://localhost:5173"},
		{"https://classlab.kampus.ac.id:8443"},
		nil,
	} {
		if err := validateCORSOrigins(ok); err != nil {
			t.Errorf("%v rejected: %v", ok, err)
		}
	}
	for name, bad := range map[string][]string{
		"wildcard":            {"*"},
		"wildcard among good": {"https://a.example", "*"},
		"path":                {"https://a.example/app"},
		"trailing slash":      {"https://a.example/"},
		"no scheme":           {"a.example"},
		"wrong scheme":        {"ftp://a.example"},
		"credentials in url":  {"https://user:pw@a.example"},
		"query":               {"https://a.example?x=1"},
		"empty entry":         {""},
	} {
		if err := validateCORSOrigins(bad); err == nil {
			t.Errorf("%s: accepted %v", name, bad)
		}
	}
}
