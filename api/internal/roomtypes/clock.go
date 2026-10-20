package roomtypes

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// MinutesPerSlot is the granularity of operating hours and durations (BR-01, BR-29).
const MinutesPerSlot = 30

const minutesPerDay = 24 * 60

// ParseClock reads "HH:MM" as minutes after midnight. "24:00" is accepted so a day can close at midnight.
func ParseClock(v string) (int, bool) {
	hh, mm, ok := strings.Cut(v, ":")
	if !ok || len(hh) != 2 || len(mm) != 2 {
		return 0, false
	}
	h, errH := strconv.Atoi(hh)
	m, errM := strconv.Atoi(mm)
	if errH != nil || errM != nil || h < 0 || h > 24 || m < 0 || m > 59 || (h == 24 && m != 0) {
		return 0, false
	}
	return h*60 + m, true
}

// FormatClock renders minutes after midnight as "HH:MM".
func FormatClock(minutes int) string { return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60) }

// PGTime converts minutes after midnight to a Postgres time value.
func PGTime(minutes int) pgtype.Time {
	return pgtype.Time{Microseconds: int64(minutes) * int64(time.Minute/time.Microsecond), Valid: true}
}

// MinutesOf converts a Postgres time to minutes after midnight; a NULL time yields (0, false).
func MinutesOf(t pgtype.Time) (int, bool) {
	if !t.Valid {
		return 0, false
	}
	return minutesOfDay(t.Microseconds), true
}

// ValidHours reports whether open and close form a usable window: on the slot grid, open before close.
func ValidHours(open, close int) bool {
	return open >= 0 && close <= minutesPerDay && open < close &&
		open%MinutesPerSlot == 0 && close%MinutesPerSlot == 0
}
