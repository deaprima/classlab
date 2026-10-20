package config

import (
	"github.com/joho/godotenv"
	"strings"
	"testing"
	"time"
)

func TestBusinessDefaultsMatchBrief(t *testing.T) {
	got, err := loadBusiness()
	if err != nil {
		t.Fatal(err)
	}
	want := Business{
		PendingTTL:           24 * time.Hour,
		EditCutoff:           60 * time.Minute,
		MaxOccurrences:       14,
		ImportMaxBytes:       2 * megabyte,
		ImportMaxRows:        5000,
		ImportMaxWeeks:       30,
		TempPasswordLength:   10,
		MinPasswordLength:    10,
		ReminderCheckinLast:  5 * time.Minute,
		ReminderCheckout:     10 * time.Minute,
		AutoApproveMaxPoints: 1,
		MaxActiveBookings:    2,
		MaxActiveBookingsOrg: 5,
		MaxLetterBytes:       5 * megabyte,
		LetterRetention:      365 * 24 * time.Hour,
		OrphanAttachmentAge:  24 * time.Hour,
		CheckinEarly:         15 * time.Minute,
		CheckinLate:          15 * time.Minute,
		CheckoutGrace:        15 * time.Minute,
		LateCancel:           2 * time.Hour,
		ViolationWindow:      30 * 24 * time.Hour,
		BlockThresholdPoints: 6,
		BlockDuration:        14 * 24 * time.Hour,
		PointsNoShow:         2,
		PointsNoCheckout:     1,
		PointsLateCancel:     1,
		PhotoRetention:       60 * 24 * time.Hour,
		MaxUploadBytes:       5 * megabyte,
	}
	if got != want {
		t.Errorf("defaults = %+v\nwant       %+v", got, want)
	}
}

func TestBusinessReadsEnvironmentWithUnits(t *testing.T) {
	t.Setenv("PENDING_TTL_HOURS", "12")
	t.Setenv("EDIT_CUTOFF_MIN", "30")
	t.Setenv("IMPORT_MAX_MB", "3")
	t.Setenv("BLOCK_DAYS", "7")
	t.Setenv("MAX_ACTIVE_BOOKINGS", "4")
	t.Setenv("POINTS_NO_SHOW", "3")

	got, err := loadBusiness()
	if err != nil {
		t.Fatal(err)
	}
	if got.PendingTTL != 12*time.Hour || got.EditCutoff != 30*time.Minute || got.ImportMaxBytes != 3*megabyte ||
		got.BlockDuration != 7*24*time.Hour || got.MaxActiveBookings != 4 || got.PointsNoShow != 3 {
		t.Errorf("overrides not applied: %+v", got)
	}
	if got.MaxOccurrences != 14 {
		t.Errorf("untouched setting changed: MaxOccurrences = %d", got.MaxOccurrences)
	}
}

func TestBusinessRejectsMalformedValues(t *testing.T) {
	for key, value := range map[string]string{
		"PENDING_TTL_HOURS":   "dua puluh empat",
		"EDIT_CUTOFF_MIN":     "1.5",
		"MAX_OCCURRENCES":     "14 ",
		"IMPORT_MAX_ROWS":     "5_000",
		"BLOCK_DAYS":          "14d",
		"MIN_PASSWORD_LENGTH": "-",
	} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)
			_, err := loadBusiness()
			if err == nil {
				t.Fatalf("%s=%q accepted", key, value)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("error does not name %s: %v", key, err)
			}
		})
	}
}

func TestBusinessRejectsOutOfRangeValues(t *testing.T) {
	for key, value := range map[string]string{
		"PENDING_TTL_HOURS":         "0",
		"EDIT_CUTOFF_MIN":           "-1",
		"MAX_OCCURRENCES":           "0",
		"IMPORT_MAX_MB":             "0",
		"MIN_PASSWORD_LENGTH":       "7",
		"TEMP_PASSWORD_LENGTH":      "4",
		"REMINDER_CHECKIN_LAST_MIN": "16", // later than CHECKIN_LATE_MIN (15)
		"REMINDER_CHECKOUT_MIN":     "0",
		"AUTO_APPROVE_MAX_POINTS":   "-1",
		"MAX_ACTIVE_BOOKINGS":       "0",
		"MAX_ACTIVE_BOOKINGS_ORG":   "0",
		"MAX_LETTER_MB":             "0",
		"LETTER_RETENTION_DAYS":     "0",
		"ORPHAN_ATTACHMENT_HOURS":   "0",
		"CHECKIN_EARLY_MIN":         "-5",
		"LATE_CANCEL_HOURS":         "-1",
		"VIOLATION_WINDOW_DAYS":     "0",
		"BLOCK_THRESHOLD_POINTS":    "0",
		"BLOCK_DAYS":                "0",
		"POINTS_NO_SHOW":            "0",
		"PHOTO_RETENTION_DAYS":      "0",
		"MAX_UPLOAD_MB":             "0",
	} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)
			_, err := loadBusiness()
			if err == nil {
				t.Fatalf("%s=%q accepted", key, value)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("error does not name %s: %v", key, err)
			}
		})
	}
}

func TestBusinessReportsEveryProblemAtOnce(t *testing.T) {
	t.Setenv("MAX_OCCURRENCES", "banyak")
	t.Setenv("BLOCK_DAYS", "tujuh")
	_, err := loadBusiness()
	if err == nil {
		t.Fatal("accepted")
	}
	for _, key := range []string{"MAX_OCCURRENCES", "BLOCK_DAYS"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not name %s: %v", key, err)
		}
	}
}

func TestBusinessAllowsZeroAutoApprovePoints(t *testing.T) {
	t.Setenv("AUTO_APPROVE_MAX_POINTS", "0")
	got, err := loadBusiness()
	if err != nil || got.AutoApproveMaxPoints != 0 {
		t.Errorf("AUTO_APPROVE_MAX_POINTS=0: %+v, %v", got, err)
	}
}

// The example file is what a new developer copies, so it must always start the server and must agree
// with the defaults in code.
func TestEnvExampleLoadsAndMatchesDefaults(t *testing.T) {
	values, err := godotenv.Read("../../../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range values {
		t.Setenv(k, v)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with .env.example: %v", err)
	}
	// cfg.Business came from the file; blank every variable it names to get the in-code defaults.
	for k := range values {
		t.Setenv(k, "")
	}
	clean, err := loadBusiness()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Business != clean {
		t.Errorf(".env.example differs from the code defaults:\nfile    %+v\ndefault %+v", cfg.Business, clean)
	}
}
