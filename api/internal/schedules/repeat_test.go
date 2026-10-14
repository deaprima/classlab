package schedules

import (
	"testing"
	"time"

	"classlab/api/internal/platform/clock"
)

func wib(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, clock.WIB)
}

func TestOccurrencesWithoutRepeatIsOneSession(t *testing.T) {
	start, end := wib(2026, 10, 20, 8, 0), wib(2026, 10, 20, 10, 0)
	got, problem := Occurrences(start, end, "")
	if problem != nil || len(got) != 1 || !got[0].Start.Equal(start) || !got[0].End.Equal(end) {
		t.Fatalf("got %v, %v", got, problem)
	}
}

func TestOccurrencesAreWeeklyAndUntilIsInclusive(t *testing.T) {
	start, end := wib(2026, 10, 20, 13, 0), wib(2026, 10, 20, 15, 0)

	cases := []struct {
		until string
		want  int
	}{
		{"2026-10-20", 1}, // same day as the first session
		{"2026-10-26", 1}, // the day before the second session
		{"2026-10-27", 2}, // the day of the second session
		{"2026-11-10", 4},
		{"2027-04-13", 26},
	}
	for _, tc := range cases {
		got, problem := Occurrences(start, end, tc.until)
		if problem != nil {
			t.Errorf("until %s: %v", tc.until, problem.Message)
			continue
		}
		if len(got) != tc.want {
			t.Errorf("until %s: %d sessions; want %d", tc.until, len(got), tc.want)
		}
		for i, w := range got {
			if wantStart := start.AddDate(0, 0, 7*i); !w.Start.Equal(wantStart) {
				t.Errorf("until %s: session %d starts %v; want %v", tc.until, i, w.Start, wantStart)
			}
			if w.End.Sub(w.Start) != 2*time.Hour {
				t.Errorf("until %s: session %d lasts %v; want 2h", tc.until, i, w.End.Sub(w.Start))
			}
		}
	}
}

func TestOccurrencesKeepWallClockTimeAcrossMonthsAndYears(t *testing.T) {
	start, end := wib(2026, 12, 29, 23, 0), wib(2026, 12, 29, 23, 30)
	got, problem := Occurrences(start, end, "2027-01-12")
	if problem != nil || len(got) != 3 {
		t.Fatalf("got %d sessions, %v", len(got), problem)
	}
	last := clock.ToWIB(got[2].Start)
	if last.Year() != 2027 || last.Month() != time.January || last.Day() != 12 || last.Hour() != 23 {
		t.Errorf("third session = %v; want 12 Jan 2027 23:00 WIB", last)
	}
}

// The "until" date is a WIB date: 22:00 UTC on the 19th is already the 20th in Jakarta.
func TestOccurrencesTreatUntilAsAWIBDate(t *testing.T) {
	start := time.Date(2026, 10, 19, 22, 0, 0, 0, time.UTC) // 20 Oct 05:00 WIB
	got, problem := Occurrences(start, start.Add(time.Hour), "2026-10-27")
	if problem != nil || len(got) != 2 {
		t.Fatalf("got %d sessions, %v; want 2 (20 and 27 Oct WIB)", len(got), problem)
	}
}

func TestOccurrencesRejectBadInput(t *testing.T) {
	start, end := wib(2026, 10, 20, 8, 0), wib(2026, 10, 20, 9, 0)

	for name, until := range map[string]string{
		"not a date":       "20-10-2026",
		"garbage":          "besok",
		"before the start": "2026-10-19",
	} {
		if _, problem := Occurrences(start, end, until); problem == nil || problem.Field != "repeat.until" {
			t.Errorf("%s: problem = %v; want one on repeat.until", name, problem)
		}
	}

	// 27 sessions: one more than the limit.
	if _, problem := Occurrences(start, end, "2027-04-20"); problem == nil || problem.Field != "repeat.until" {
		t.Errorf("27 sessions: problem = %v; want one on repeat.until", problem)
	}
}
