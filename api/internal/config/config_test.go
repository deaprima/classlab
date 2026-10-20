package config

import (
	"testing"
)

func TestBookingRulesRequiresAPositiveQuota(t *testing.T) {
	r, err := (&Config{BookingMaxActivePerUser: 5}).BookingRules()
	if err != nil || r.MaxActivePerUser != 5 {
		t.Errorf("quota 5: %+v, %v", r, err)
	}
	for _, n := range []int{0, -1} {
		if _, err := (&Config{BookingMaxActivePerUser: n}).BookingRules(); err == nil {
			t.Errorf("quota %d accepted", n)
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
