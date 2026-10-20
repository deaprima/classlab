package imports

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/roomtypes"
)

// scheduleRow is one validated line of a schedule CSV (BR-23): a weekly class that runs from
// SemesterStart to SemesterEnd, minus the skipped dates.
type scheduleRow struct {
	Line          int
	RoomCode      string
	Title         string
	Weekday       time.Weekday // time.Monday ... time.Sunday
	StartMinute   int          // minutes after midnight WIB
	EndMinute     int
	SemesterStart time.Time // a date, midnight WIB
	SemesterEnd   time.Time
	SkipDates     map[string]bool // ISO dates
}

const dateLayout = "2006-01-02"

// parseScheduleRow reads one CSV line. It reports every problem it finds in the line, not just the first.
func parseScheduleRow(r Row, maxWeeks int) (scheduleRow, []RowError) {
	var problems []RowError
	bad := func(col, msg string) { problems = append(problems, RowError{Line: r.Line, Column: col, Message: msg}) }

	row := scheduleRow{Line: r.Line, RoomCode: strings.ToUpper(r.Cells["room_code"]), Title: r.Cells["title"], SkipDates: map[string]bool{}}
	if row.RoomCode == "" {
		bad("room_code", "room_code wajib diisi.")
	}
	if n := len([]rune(row.Title)); n < 1 || n > 200 {
		bad("title", "title wajib diisi (maksimal 200 karakter).")
	}

	dowOK := false
	dow, err := strconv.Atoi(r.Cells["day_of_week"])
	if err != nil || dow < 1 || dow > 7 {
		bad("day_of_week", "day_of_week harus 1 (Senin) sampai 7 (Minggu).")
	} else {
		dowOK = true
		row.Weekday = time.Weekday(dow % 7) // 1..6 map to Monday..Saturday, 7 to Sunday
	}

	start, startOK := roomtypes.ParseClock(r.Cells["start_time"])
	end, endOK := roomtypes.ParseClock(r.Cells["end_time"])
	if !startOK {
		bad("start_time", "start_time harus berformat HH:MM.")
	}
	if !endOK {
		bad("end_time", "end_time harus berformat HH:MM.")
	}
	if startOK && endOK {
		// A lecture ends the day it starts, and 24:00 is a valid end but not a valid start.
		if start >= end || start >= 24*60 {
			bad("end_time", "end_time harus setelah start_time pada hari yang sama.")
		}
		row.StartMinute, row.EndMinute = start, end
	}

	semStart, errS := parseDate(r.Cells["semester_start"])
	semEnd, errE := parseDate(r.Cells["semester_end"])
	if errS != nil {
		bad("semester_start", "semester_start harus berformat YYYY-MM-DD.")
	}
	if errE != nil {
		bad("semester_end", "semester_end harus berformat YYYY-MM-DD.")
	}
	if errS == nil && errE == nil {
		if semEnd.Before(semStart) {
			bad("semester_end", "semester_end tidak boleh sebelum semester_start.")
		}
		row.SemesterStart, row.SemesterEnd = semStart, semEnd
		if dowOK && !semEnd.Before(semStart) && row.meetingsInRange() > maxWeeks {
			bad("semester_end", fmt.Sprintf("Rentang semester memuat lebih dari %d pertemuan mingguan.", maxWeeks))
		}
	}

	for _, d := range strings.Fields(r.Cells["skip_dates"]) {
		if _, err := parseDate(d); err != nil {
			bad("skip_dates", fmt.Sprintf("Tanggal %q harus berformat YYYY-MM-DD, dipisah spasi.", d))
			continue
		}
		row.SkipDates[d] = true
	}
	return row, problems
}

// parseDate reads YYYY-MM-DD as midnight WIB, refusing anything the time package would normalise (a 31st
// of February must fail, not roll into March).
func parseDate(v string) (time.Time, error) {
	t, err := time.ParseInLocation(dateLayout, v, clock.WIB)
	if err != nil {
		return time.Time{}, err
	}
	if t.Format(dateLayout) != v {
		return time.Time{}, fmt.Errorf("not a calendar date: %q", v)
	}
	return t, nil
}

// meetingsInRange counts the weekday's occurrences between the semester's first and last day, before
// skipped dates and before dropping the past. That count is what IMPORT_MAX_WEEKS limits per line.
func (r scheduleRow) meetingsInRange() int {
	n := 0
	for day := r.firstDay(); !day.After(r.SemesterEnd); day = day.AddDate(0, 0, 7) {
		n++
	}
	return n
}

// firstDay is the first date on or after SemesterStart that falls on the row's weekday.
func (r scheduleRow) firstDay() time.Time {
	day := r.SemesterStart
	for day.Weekday() != r.Weekday {
		day = day.AddDate(0, 0, 1)
	}
	return day
}

// occurrence is one concrete class meeting.
type occurrence struct {
	Start time.Time // UTC instants
	End   time.Time
}

// expand lists the meetings of a row: the chosen weekday of every week between the semester's first and
// last day, without the skipped dates. Meetings that have already started are left out: an import
// describes what is still to come, and the past is not rewritten (a re-import after a rollback would
// otherwise collide with the meetings that stayed behind).
func (r scheduleRow) expand(now time.Time) []occurrence {
	var out []occurrence
	for day := r.firstDay(); !day.After(r.SemesterEnd); day = day.AddDate(0, 0, 7) {
		if r.SkipDates[day.Format(dateLayout)] {
			continue
		}
		start := day.Add(time.Duration(r.StartMinute) * time.Minute)
		end := day.Add(time.Duration(r.EndMinute) * time.Minute)
		if start.Before(now) {
			continue
		}
		out = append(out, occurrence{Start: start.UTC(), End: end.UTC()})
	}
	return out
}
