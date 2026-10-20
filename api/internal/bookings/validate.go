package bookings

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/roomtypes"

	"github.com/jackc/pgx/v5/pgtype"
)

// validateWindow applies the checks of BR-01 that do not depend on the room: ordering, one day, the
// 30-minute grid and not in the past. Everything is judged in WIB, the zone users read the clock in.
func validateWindow(start, end, now time.Time) []httpx.ErrorDetail {
	if start.IsZero() || end.IsZero() {
		var out []httpx.ErrorDetail
		if start.IsZero() {
			out = append(out, httpx.ErrorDetail{Field: "start_at", Message: "start_at wajib diisi."})
		}
		if end.IsZero() {
			out = append(out, httpx.ErrorDetail{Field: "end_at", Message: "end_at wajib diisi."})
		}
		return out
	}
	if !start.Before(end) {
		return []httpx.ErrorDetail{{Field: "end_at", Message: "end_at harus setelah start_at."}}
	}
	st, en := clock.ToWIB(start), clock.ToWIB(end)
	if st.Year() != en.Year() || st.YearDay() != en.YearDay() {
		return []httpx.ErrorDetail{{Field: "end_at", Message: "Booking harus berada pada hari yang sama (WIB)."}}
	}

	var out []httpx.ErrorDetail
	if !onSlot(st) {
		out = append(out, httpx.ErrorDetail{Field: "start_at", Message: fmt.Sprintf("start_at harus kelipatan %d menit.", slotMinutes)})
	}
	if !onSlot(en) {
		out = append(out, httpx.ErrorDetail{Field: "end_at", Message: fmt.Sprintf("end_at harus kelipatan %d menit.", slotMinutes)})
	}
	if start.Before(now) {
		out = append(out, httpx.ErrorDetail{Field: "start_at", Message: "start_at tidak boleh di masa lalu."})
	}
	return out
}

// validatePolicyWindow applies what depends on the room: the duration and booking-horizon limits of its
// type (BR-01, BR-20) and its operating hours, which are the room's own when set and the type's default
// otherwise (BR-29). Hours get their own error code so a client can tell "closed at that time" from a
// malformed request.
func validatePolicyWindow(p roomtypes.Policy, roomOpen, roomClose pgtype.Time, start, end, now time.Time) error {
	st, en := clock.ToWIB(start), clock.ToWIB(end)

	open, closing := p.OpenMinute, p.CloseMinute
	if o, ok := roomtypes.MinutesOf(roomOpen); ok {
		if c, ok := roomtypes.MinutesOf(roomClose); ok {
			open, closing = o, c
		}
	}
	if minuteOfDay(st) < open || minuteOfDay(en) > closing {
		return outsideOperatingHours(open, closing)
	}

	var out []httpx.ErrorDetail
	if d := end.Sub(start); d < p.MinDuration {
		out = append(out, httpx.ErrorDetail{Field: "end_at", Message: "Durasi minimal " + formatDuration(p.MinDuration) + "."})
	} else if d > p.MaxDuration {
		out = append(out, httpx.ErrorDetail{Field: "end_at", Message: "Durasi maksimal " + formatDuration(p.MaxDuration) + "."})
	}
	today := clock.ToWIB(now)
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, clock.WIB)
	startDay := time.Date(st.Year(), st.Month(), st.Day(), 0, 0, 0, 0, clock.WIB)
	if startDay.Sub(todayStart) > time.Duration(p.MaxAdvanceDays)*24*time.Hour {
		out = append(out, httpx.ErrorDetail{Field: "start_at", Message: fmt.Sprintf("Booking maksimal %d hari ke depan.", p.MaxAdvanceDays)})
	}
	if len(out) > 0 {
		return httpx.ValidationError(out...)
	}
	return nil
}

// outsideOperatingHours is the 422 OUTSIDE_OPERATING_HOURS response; details carry the effective window.
func outsideOperatingHours(open, closing int) error {
	return httpx.NewCustomError(http.StatusUnprocessableEntity, "OUTSIDE_OPERATING_HOURS",
		fmt.Sprintf("Waktu di luar jam operasional ruang ini (%s-%s WIB).", formatMinute(open), formatMinute(closing)),
		[]httpx.ErrorDetail{
			{Field: "open_time", Message: formatMinute(open)},
			{Field: "close_time", Message: formatMinute(closing)},
		}, httpx.ErrValidation)
}

func onSlot(t time.Time) bool {
	return t.Minute()%slotMinutes == 0 && t.Second() == 0 && t.Nanosecond() == 0
}

func minuteOfDay(t time.Time) int { return t.Hour()*60 + t.Minute() }

func formatMinute(m int) string { return fmt.Sprintf("%02d:%02d", m/60, m%60) }

func formatDuration(d time.Duration) string {
	if d%time.Hour == 0 {
		return fmt.Sprintf("%d jam", int(d/time.Hour))
	}
	return fmt.Sprintf("%d menit", int(d/time.Minute))
}

// tagDetails sets the occurrence number on every detail of a validation problem, so a client can tell which
// date of a multi-date request it belongs to (BR-27). Errors that are not domain errors pass through.
func tagDetails(err error, n int) error {
	var ce *httpx.CustomDomainError
	if n == 0 || !errors.As(err, &ce) {
		return err
	}
	tagged := *ce
	tagged.Details = tagList(ce.Details, n)
	return &tagged
}

func tagList(details []httpx.ErrorDetail, n int) []httpx.ErrorDetail {
	if n == 0 {
		return details
	}
	out := make([]httpx.ErrorDetail, len(details))
	for i, d := range details {
		d.OccurrenceNo = n
		out[i] = d
	}
	return out
}
