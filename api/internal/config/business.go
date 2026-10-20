package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const megabyte = 1 << 20

// Business holds the tunable numbers behind BR-11 to BR-28. Unlike the older settings in Config, a
// malformed value stops the server at start instead of silently falling back to the default, because
// a typo here would quietly change who may book, who is blocked, or when a booking expires.
type Business struct {
	PendingTTL time.Duration // BR-22: how long a booking may wait for a decision
	EditCutoff time.Duration // BR-26: edits close this long before start_at

	MaxOccurrences int // BR-27: dates in one multi-date request

	ImportMaxBytes int64 // BR-23, BR-24
	ImportMaxRows  int
	ImportMaxWeeks int

	TempPasswordLength int // BR-24
	MinPasswordLength  int

	ReminderCheckinLast time.Duration // BR-28: minutes before the check-in window closes
	ReminderCheckout    time.Duration // BR-28: minutes before end_at

	AutoApproveMaxPoints int // BR-17: auto-approval needs fewer active points than this

	MaxActiveBookings    int // BR-15, per MAHASISWA
	MaxActiveBookingsOrg int // BR-15, per ORGANISASI

	MaxLetterBytes      int64 // BR-19
	LetterRetention     time.Duration
	OrphanAttachmentAge time.Duration

	CheckinEarly  time.Duration // BR-11: window opens this long before start_at
	CheckinLate   time.Duration // BR-11, BR-13: window closes this long after start_at
	CheckoutGrace time.Duration // BR-13: BERLANGSUNG becomes TANPA_CHECKOUT this long after end_at

	LateCancel time.Duration // BR-14: cancelling closer to start_at than this is a violation

	ViolationWindow      time.Duration // BR-14
	BlockThresholdPoints int
	BlockDuration        time.Duration
	PointsNoShow         int
	PointsNoCheckout     int
	PointsLateCancel     int

	PhotoRetention time.Duration // BR-16
	MaxUploadBytes int64
}

// envReader collects every malformed value so a start-up failure lists all of them at once.
type envReader struct{ errs []error }

func (r *envReader) integer(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s: %q is not a whole number", key, v))
		return def
	}
	return n
}

func (r *envReader) duration(key string, def int, unit time.Duration) time.Duration {
	return time.Duration(r.integer(key, def)) * unit
}

func (r *envReader) bytes(key string, defMB int) int64 {
	return int64(r.integer(key, defMB)) * megabyte
}

// loadBusiness reads the BR-11 to BR-28 settings; the defaults are the values in brief.md section 4.3.
func loadBusiness() (Business, error) {
	var r envReader
	b := Business{
		PendingTTL: r.duration("PENDING_TTL_HOURS", 24, time.Hour),
		EditCutoff: r.duration("EDIT_CUTOFF_MIN", 60, time.Minute),

		MaxOccurrences: r.integer("MAX_OCCURRENCES", 14),

		ImportMaxBytes: r.bytes("IMPORT_MAX_MB", 2),
		ImportMaxRows:  r.integer("IMPORT_MAX_ROWS", 5000),
		ImportMaxWeeks: r.integer("IMPORT_MAX_WEEKS", 30),

		TempPasswordLength: r.integer("TEMP_PASSWORD_LENGTH", 10),
		MinPasswordLength:  r.integer("MIN_PASSWORD_LENGTH", 10),

		ReminderCheckinLast: r.duration("REMINDER_CHECKIN_LAST_MIN", 5, time.Minute),
		ReminderCheckout:    r.duration("REMINDER_CHECKOUT_MIN", 10, time.Minute),

		AutoApproveMaxPoints: r.integer("AUTO_APPROVE_MAX_POINTS", 1),

		MaxActiveBookings:    r.integer("MAX_ACTIVE_BOOKINGS", 2),
		MaxActiveBookingsOrg: r.integer("MAX_ACTIVE_BOOKINGS_ORG", 5),

		MaxLetterBytes:      r.bytes("MAX_LETTER_MB", 5),
		LetterRetention:     r.duration("LETTER_RETENTION_DAYS", 365, 24*time.Hour),
		OrphanAttachmentAge: r.duration("ORPHAN_ATTACHMENT_HOURS", 24, time.Hour),

		CheckinEarly:  r.duration("CHECKIN_EARLY_MIN", 15, time.Minute),
		CheckinLate:   r.duration("CHECKIN_LATE_MIN", 15, time.Minute),
		CheckoutGrace: r.duration("CHECKOUT_GRACE_MIN", 15, time.Minute),

		LateCancel: r.duration("LATE_CANCEL_HOURS", 2, time.Hour),

		ViolationWindow:      r.duration("VIOLATION_WINDOW_DAYS", 30, 24*time.Hour),
		BlockThresholdPoints: r.integer("BLOCK_THRESHOLD_POINTS", 6),
		BlockDuration:        r.duration("BLOCK_DAYS", 14, 24*time.Hour),
		PointsNoShow:         r.integer("POINTS_NO_SHOW", 2),
		PointsNoCheckout:     r.integer("POINTS_NO_CHECKOUT", 1),
		PointsLateCancel:     r.integer("POINTS_LATE_CANCEL", 1),

		PhotoRetention: r.duration("PHOTO_RETENTION_DAYS", 60, 24*time.Hour),
		MaxUploadBytes: r.bytes("MAX_UPLOAD_MB", 5),
	}
	if len(r.errs) > 0 {
		return Business{}, errors.Join(r.errs...)
	}
	if err := b.validate(); err != nil {
		return Business{}, err
	}
	return b, nil
}

// validate reports every out-of-range value, not just the first.
func (b Business) validate() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if b.PendingTTL <= 0 {
		bad("PENDING_TTL_HOURS must be at least 1")
	}
	if b.EditCutoff < 0 {
		bad("EDIT_CUTOFF_MIN must not be negative")
	}
	if b.MaxOccurrences < 1 {
		bad("MAX_OCCURRENCES must be at least 1")
	}
	if b.ImportMaxBytes < megabyte || b.ImportMaxRows < 1 || b.ImportMaxWeeks < 1 {
		bad("IMPORT_MAX_MB, IMPORT_MAX_ROWS and IMPORT_MAX_WEEKS must each be at least 1")
	}
	// Eight is the floor for a password anyone could defend; the brief's 10 is the default.
	if b.MinPasswordLength < 8 {
		bad("MIN_PASSWORD_LENGTH must be at least 8")
	}
	if b.TempPasswordLength < 8 {
		bad("TEMP_PASSWORD_LENGTH must be at least 8")
	}
	// The last check-in reminder fires at start+late-N; N beyond the late window would land before start_at.
	if b.ReminderCheckinLast < 0 || b.ReminderCheckinLast > b.CheckinLate {
		bad("REMINDER_CHECKIN_LAST_MIN must be between 0 and CHECKIN_LATE_MIN")
	}
	if b.ReminderCheckout <= 0 {
		bad("REMINDER_CHECKOUT_MIN must be at least 1")
	}
	// Zero is a valid way to switch auto-approval off: nobody has fewer than 0 points.
	if b.AutoApproveMaxPoints < 0 {
		bad("AUTO_APPROVE_MAX_POINTS must not be negative")
	}
	if b.MaxActiveBookings < 1 || b.MaxActiveBookingsOrg < 1 {
		bad("MAX_ACTIVE_BOOKINGS and MAX_ACTIVE_BOOKINGS_ORG must each be at least 1")
	}
	if b.MaxLetterBytes < megabyte || b.MaxUploadBytes < megabyte {
		bad("MAX_LETTER_MB and MAX_UPLOAD_MB must each be at least 1")
	}
	if b.LetterRetention < 24*time.Hour || b.PhotoRetention < 24*time.Hour {
		bad("LETTER_RETENTION_DAYS and PHOTO_RETENTION_DAYS must each be at least 1")
	}
	if b.OrphanAttachmentAge < time.Hour {
		bad("ORPHAN_ATTACHMENT_HOURS must be at least 1")
	}
	if b.CheckinEarly < 0 || b.CheckinLate < 0 || b.CheckoutGrace < 0 {
		bad("CHECKIN_EARLY_MIN, CHECKIN_LATE_MIN and CHECKOUT_GRACE_MIN must not be negative")
	}
	if b.LateCancel < 0 {
		bad("LATE_CANCEL_HOURS must not be negative")
	}
	if b.ViolationWindow < 24*time.Hour || b.BlockDuration < 24*time.Hour {
		bad("VIOLATION_WINDOW_DAYS and BLOCK_DAYS must each be at least 1")
	}
	if b.BlockThresholdPoints < 1 {
		bad("BLOCK_THRESHOLD_POINTS must be at least 1")
	}
	// violations.points has CHECK (points > 0).
	if b.PointsNoShow < 1 || b.PointsNoCheckout < 1 || b.PointsLateCancel < 1 {
		bad("POINTS_NO_SHOW, POINTS_NO_CHECKOUT and POINTS_LATE_CANCEL must each be at least 1")
	}
	return errors.Join(errs...)
}
