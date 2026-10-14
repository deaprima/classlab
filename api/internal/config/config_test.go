package config

import (
	"testing"
	"time"
)

func validBookingConfig() *Config {
	return &Config{
		OperatingHoursStart:       "07:00",
		OperatingHoursEnd:         "21:00",
		BookingMinDurationMinutes: 30,
		BookingMaxDurationHours:   8,
		BookingMaxFutureDays:      30,
	}
}

func TestBookingRulesParsesSettings(t *testing.T) {
	r, err := validBookingConfig().BookingRules()
	if err != nil {
		t.Fatal(err)
	}
	want := BookingRules{OpenMinute: 7 * 60, CloseMinute: 21 * 60, MinDuration: 30 * time.Minute, MaxDuration: 8 * time.Hour, MaxFutureDays: 30}
	if r != want {
		t.Errorf("rules = %+v; want %+v", r, want)
	}

	c := validBookingConfig()
	c.OperatingHoursStart, c.OperatingHoursEnd = "08:30", "24:00"
	if r, err = c.BookingRules(); err != nil || r.OpenMinute != 8*60+30 || r.CloseMinute != 24*60 {
		t.Errorf("08:30-24:00: %+v, %v", r, err)
	}
}

func TestBookingRulesRejectsBadSettings(t *testing.T) {
	cases := map[string]func(*Config){
		"start not a time":      func(c *Config) { c.OperatingHoursStart = "pagi" },
		"start hour 25":         func(c *Config) { c.OperatingHoursStart = "25:00" },
		"minutes 60":            func(c *Config) { c.OperatingHoursEnd = "20:60" },
		"24:30":                 func(c *Config) { c.OperatingHoursEnd = "24:30" },
		"missing colon":         func(c *Config) { c.OperatingHoursStart = "0700" },
		"closes before opening": func(c *Config) { c.OperatingHoursStart, c.OperatingHoursEnd = "21:00", "07:00" },
		"zero-width day":        func(c *Config) { c.OperatingHoursEnd = c.OperatingHoursStart },
		"zero min duration":     func(c *Config) { c.BookingMinDurationMinutes = 0 },
		"zero max duration":     func(c *Config) { c.BookingMaxDurationHours = 0 },
		"min above max":         func(c *Config) { c.BookingMinDurationMinutes, c.BookingMaxDurationHours = 600, 8 },
		"no future days":        func(c *Config) { c.BookingMaxFutureDays = 0 },
	}
	for name, mutate := range cases {
		c := validBookingConfig()
		mutate(c)
		if _, err := c.BookingRules(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
