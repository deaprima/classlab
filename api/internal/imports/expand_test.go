package imports

import (
	"slices"
	"testing"
	"time"

	"classlab/api/internal/platform/clock"
)

func csvRow(line int, cells map[string]string) Row { return Row{Line: line, Cells: cells} }

func goodCells() map[string]string {
	return map[string]string{
		"room_code": "r-301", "title": "Basis Data A", "day_of_week": "1", "start_time": "07:00", "end_time": "09:30",
		"semester_start": "2026-08-24", "semester_end": "2026-12-18", "skip_dates": "2026-10-12 2026-11-02",
	}
}

func wibAt(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, clock.WIB)
}

func TestExpandListsTheWeekdayOfEveryWeekWithoutSkippedDates(t *testing.T) {
	row, problems := parseScheduleRow(csvRow(2, goodCells()), 30)
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	if row.RoomCode != "R-301" {
		t.Errorf("room code = %q, want it upper-cased", row.RoomCode)
	}

	occ := row.expand(wibAt(2026, 1, 1, 0, 0))
	// Mondays from 24 Aug to 14 Dec: 17 of them, minus the two skipped (12 Oct and 2 Nov).
	if len(occ) != 17-2 {
		t.Fatalf("occurrences = %d, want 15", len(occ))
	}
	first, last := occ[0], occ[len(occ)-1]
	if !first.Start.Equal(wibAt(2026, 8, 24, 7, 0)) || !first.End.Equal(wibAt(2026, 8, 24, 9, 30)) {
		t.Errorf("first = %v - %v", first.Start.In(clock.WIB), first.End.In(clock.WIB))
	}
	if !last.Start.Equal(wibAt(2026, 12, 14, 7, 0)) {
		t.Errorf("last = %v", last.Start.In(clock.WIB))
	}
	for _, o := range occ {
		if o.Start.In(clock.WIB).Weekday() != time.Monday {
			t.Errorf("%v is not a Monday", o.Start.In(clock.WIB))
		}
		for _, skipped := range []string{"2026-10-12", "2026-11-02"} {
			if o.Start.In(clock.WIB).Format(dateLayout) == skipped {
				t.Errorf("skipped date %s was generated", skipped)
			}
		}
	}
}

func TestExpandStartsOnTheFirstMatchingWeekdayAfterSemesterStart(t *testing.T) {
	cells := goodCells()
	cells["day_of_week"] = "3" // Wednesday
	cells["skip_dates"] = ""
	row, _ := parseScheduleRow(csvRow(2, cells), 30)
	occ := row.expand(wibAt(2026, 1, 1, 0, 0))
	if !occ[0].Start.Equal(wibAt(2026, 8, 26, 7, 0)) {
		t.Errorf("first Wednesday = %v, want 26 Aug", occ[0].Start.In(clock.WIB))
	}
}

func TestExpandMapsDayNumbersFromMondayToSunday(t *testing.T) {
	want := map[string]time.Weekday{
		"1": time.Monday, "2": time.Tuesday, "3": time.Wednesday, "4": time.Thursday,
		"5": time.Friday, "6": time.Saturday, "7": time.Sunday,
	}
	for dow, wd := range want {
		cells := goodCells()
		cells["day_of_week"], cells["skip_dates"] = dow, ""
		row, problems := parseScheduleRow(csvRow(2, cells), 30)
		if len(problems) != 0 {
			t.Fatalf("day %s: %v", dow, problems)
		}
		if got := row.expand(wibAt(2026, 1, 1, 0, 0))[0].Start.In(clock.WIB).Weekday(); got != wd {
			t.Errorf("day_of_week %s generated a %v, want %v", dow, got, wd)
		}
	}
}

func TestExpandLeavesOutMeetingsThatHaveAlreadyStarted(t *testing.T) {
	cells := goodCells()
	cells["skip_dates"] = ""
	row, _ := parseScheduleRow(csvRow(2, cells), 30)

	// Exactly at the start of the 7 Sep meeting: that one has started, the next is still to come.
	occ := row.expand(wibAt(2026, 9, 7, 7, 0))
	if len(occ) == 0 || !occ[0].Start.Equal(wibAt(2026, 9, 7, 7, 0)) {
		t.Errorf("a meeting starting exactly now should be kept: first = %v", occ[0].Start.In(clock.WIB))
	}
	occ = row.expand(wibAt(2026, 9, 7, 7, 1))
	if !occ[0].Start.Equal(wibAt(2026, 9, 14, 7, 0)) {
		t.Errorf("a minute later the first is %v, want 14 Sep", occ[0].Start.In(clock.WIB))
	}
	if got := row.expand(wibAt(2027, 1, 1, 0, 0)); len(got) != 0 {
		t.Errorf("after the semester: %d occurrences, want none", len(got))
	}
}

func TestParseScheduleRowReportsEveryProblemInALine(t *testing.T) {
	cases := map[string]struct {
		change  map[string]string
		columns []string
	}{
		"no room code":          {map[string]string{"room_code": ""}, []string{"room_code"}},
		"no title":              {map[string]string{"title": ""}, []string{"title"}},
		"day 0":                 {map[string]string{"day_of_week": "0"}, []string{"day_of_week"}},
		"day 8":                 {map[string]string{"day_of_week": "8"}, []string{"day_of_week"}},
		"day not a number":      {map[string]string{"day_of_week": "senin"}, []string{"day_of_week"}},
		"bad start":             {map[string]string{"start_time": "7"}, []string{"start_time"}},
		"bad end":               {map[string]string{"end_time": "25:00"}, []string{"end_time"}},
		"end before start":      {map[string]string{"start_time": "10:00", "end_time": "09:00"}, []string{"end_time"}},
		"zero length":           {map[string]string{"start_time": "10:00", "end_time": "10:00"}, []string{"end_time"}},
		"starts at midnight 24": {map[string]string{"start_time": "24:00", "end_time": "24:00"}, []string{"end_time"}},
		"impossible date":       {map[string]string{"semester_start": "2026-02-30"}, []string{"semester_start"}},
		"wrong date format":     {map[string]string{"semester_end": "18/12/2026"}, []string{"semester_end"}},
		"end before start date": {map[string]string{"semester_end": "2026-08-01"}, []string{"semester_end"}},
		"over 30 weeks":         {map[string]string{"semester_end": "2027-06-30"}, []string{"semester_end"}},
		"bad skip date":         {map[string]string{"skip_dates": "2026-10-12 besok"}, []string{"skip_dates"}},
		"two problems":          {map[string]string{"title": "", "day_of_week": "9"}, []string{"title", "day_of_week"}},
	}
	for name, c := range cases {
		cells := goodCells()
		for k, v := range c.change {
			cells[k] = v
		}
		_, problems := parseScheduleRow(csvRow(7, cells), 30)
		var got []string
		for _, p := range problems {
			if p.Line != 7 {
				t.Errorf("%s: problem on line %d, want 7", name, p.Line)
			}
			got = append(got, p.Column)
		}
		slices.Sort(got)
		want := slices.Clone(c.columns)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: columns %v, want %v", name, got, want)
		}
	}
}

func TestParseScheduleRowAcceptsMidnightClosingAndExactlyMaxWeeks(t *testing.T) {
	cells := goodCells()
	cells["start_time"], cells["end_time"] = "20:00", "24:00"
	cells["semester_start"], cells["semester_end"] = "2026-08-24", "2027-03-15" // the 30th Monday
	cells["skip_dates"] = ""
	row, problems := parseScheduleRow(csvRow(2, cells), 30)
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	occ := row.expand(wibAt(2026, 1, 1, 0, 0))
	if len(occ) != 30 {
		t.Errorf("occurrences = %d, want 30", len(occ))
	}
	if !occ[0].End.Equal(wibAt(2026, 8, 25, 0, 0)) {
		t.Errorf("a 24:00 end is midnight the next day: %v", occ[0].End.In(clock.WIB))
	}
}
