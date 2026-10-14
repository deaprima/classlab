package schedules

import (
	"fmt"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/overlap"
	"classlab/api/internal/platform/clock"
)

// MaxOccurrences caps one repeat request at 26 weekly sessions, i.e. roughly one semester (BR-09).
const MaxOccurrences = 26

const dateLayout = "2006-01-02"

// Window is one concrete session, as UTC instants.
type Window = overlap.Window

// Occurrences expands a weekly repeat. The first session is the given start/end; each next one is
// seven calendar days later in WIB, and the last is the latest one that starts on or before `until`
// (a date in WIB). Without `until` there is exactly one session.
func Occurrences(start, end time.Time, until string) ([]Window, *httpx.ErrorDetail) {
	first := Window{Start: start.UTC(), End: end.UTC()}
	if until == "" {
		return []Window{first}, nil
	}

	untilDay, err := time.ParseInLocation(dateLayout, until, clock.WIB)
	if err != nil {
		return nil, &httpx.ErrorDetail{Field: "repeat.until", Message: "repeat.until harus berformat YYYY-MM-DD."}
	}
	startWIB := clock.ToWIB(start)
	startDay := time.Date(startWIB.Year(), startWIB.Month(), startWIB.Day(), 0, 0, 0, 0, clock.WIB)
	if untilDay.Before(startDay) {
		return nil, &httpx.ErrorDetail{Field: "repeat.until", Message: "repeat.until tidak boleh sebelum tanggal mulai."}
	}

	// Jakarta has no DST, so adding whole days to a WIB time is the same wall-clock time every week.
	var out []Window
	for k := 0; ; k++ {
		day := startDay.AddDate(0, 0, 7*k)
		if day.After(untilDay) {
			break
		}
		if len(out) == MaxOccurrences {
			return nil, &httpx.ErrorDetail{
				Field:   "repeat.until",
				Message: fmt.Sprintf("Jadwal berulang maksimal %d minggu.", MaxOccurrences),
			}
		}
		out = append(out, Window{
			Start: first.Start.AddDate(0, 0, 7*k),
			End:   first.End.AddDate(0, 0, 7*k),
		})
	}
	return out, nil
}
