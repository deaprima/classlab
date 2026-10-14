// Package schedules manages academic and manual room schedules (room_schedules).
package schedules

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	pgExclusionViolation = "23P01"

	maxTitleRunes   = 200
	maxConflictList = 50
)

type Schedule struct {
	ID        uuid.UUID `json:"id"`
	RoomID    uuid.UUID `json:"room_id"`
	Title     string    `json:"title"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	Source    string    `json:"source"`
	CreatedBy uuid.UUID `json:"created_by"`
}

func fromRow(r dbgen.RoomSchedule) Schedule {
	return Schedule{
		ID: r.ID, RoomID: r.RoomID, Title: r.Title,
		StartAt: clock.ToWIB(r.StartAt), EndAt: clock.ToWIB(r.EndAt),
		Source: string(r.Source), CreatedBy: r.CreatedBy,
	}
}

type Repeat struct {
	Freq  string
	Until string
}

type CreateInput struct {
	RoomID  uuid.UUID
	Title   string
	StartAt time.Time
	EndAt   time.Time
	Source  string // optional; defaults to MANUAL
	Repeat  *Repeat
}

// UpdateInput holds only the fields the caller sent; nil means "leave unchanged".
type UpdateInput struct {
	Title   *string
	StartAt *time.Time
	EndAt   *time.Time
}

type Service struct {
	pool  *db.Pool
	q     *dbgen.Queries
	clock clock.Clock
}

func NewService(pool *db.Pool, clk clock.Clock) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), clock: clk}
}

func canManage(actor httpx.Principal) bool {
	return actor.HasRole(httpx.RoleTUKalab, httpx.RoleKaprodi)
}

// Create inserts one schedule, or one per week when Repeat is set (BR-09). Everything happens in a
// single transaction under the room lock: if any session collides, nothing is written and the
// response lists every collision.
func (s *Service) Create(ctx context.Context, actor httpx.Principal, in CreateInput) ([]Schedule, error) {
	if !canManage(actor) {
		return nil, httpx.ErrForbidden
	}

	in.Title = strings.TrimSpace(in.Title)
	source := dbgen.ScheduleSourceMANUAL
	var problems []httpx.ErrorDetail
	problems = append(problems, validateTitle(in.Title)...)
	problems = append(problems, validateWindow(in.StartAt, in.EndAt)...)
	if in.RoomID == uuid.Nil {
		problems = append(problems, httpx.ErrorDetail{Field: "room_id", Message: "room_id wajib diisi."})
	}
	switch in.Source {
	case "":
	case string(dbgen.ScheduleSourceAKADEMIK), string(dbgen.ScheduleSourceMANUAL):
		source = dbgen.ScheduleSource(in.Source)
	default:
		problems = append(problems, httpx.ErrorDetail{Field: "source", Message: "source harus AKADEMIK atau MANUAL."})
	}

	until := ""
	if in.Repeat != nil {
		if in.Repeat.Freq != "WEEKLY" {
			problems = append(problems, httpx.ErrorDetail{Field: "repeat.freq", Message: "repeat.freq harus WEEKLY."})
		}
		if in.Repeat.Until == "" {
			problems = append(problems, httpx.ErrorDetail{Field: "repeat.until", Message: "repeat.until wajib diisi."})
		}
		until = in.Repeat.Until
	}
	if len(problems) > 0 {
		return nil, httpx.ValidationError(problems...)
	}

	windows, problem := Occurrences(in.StartAt, in.EndAt, until)
	if problem != nil {
		return nil, httpx.ValidationError(*problem)
	}

	var created []Schedule
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if _, err := q.LockRoom(ctx, in.RoomID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.ValidationError(httpx.ErrorDetail{Field: "room_id", Message: "Ruang tidak ditemukan."})
			}
			return fmt.Errorf("locking room: %w", err)
		}

		conflicts, err := findConflicts(ctx, q, in.RoomID, windows, uuid.Nil)
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			return conflictError(conflicts)
		}

		now := s.clock.Now()
		for _, w := range windows {
			row, err := q.CreateSchedule(ctx, dbgen.CreateScheduleParams{
				RoomID: in.RoomID, Title: in.Title, StartAt: w.Start, EndAt: w.End,
				Source: source, CreatedBy: actor.UserID, CreatedAt: now,
			})
			if err != nil {
				return err
			}
			created = append(created, fromRow(row))
		}
		return nil
	})
	if err != nil {
		return nil, mapWriteError(err, "creating schedule")
	}
	return created, nil
}

func (s *Service) Update(ctx context.Context, actor httpx.Principal, id uuid.UUID, in UpdateInput) (Schedule, error) {
	if !canManage(actor) {
		return Schedule{}, httpx.ErrForbidden
	}
	if in.Title == nil && in.StartAt == nil && in.EndAt == nil {
		return Schedule{}, httpx.ValidationError(httpx.ErrorDetail{Message: "Tidak ada field yang diubah."})
	}

	var problems []httpx.ErrorDetail
	var title *string
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		problems = append(problems, validateTitle(t)...)
		title = &t
	}
	if len(problems) > 0 {
		return Schedule{}, httpx.ValidationError(problems...)
	}

	var updated Schedule
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)

		// The room is only known from the schedule itself, so read it, take the room lock, then
		// read again: the first read may be stale by the time the lock is granted.
		cur, err := q.GetSchedule(ctx, id)
		if err != nil {
			return err
		}
		if _, err := q.LockRoom(ctx, cur.RoomID); err != nil {
			return err
		}
		if cur, err = q.GetSchedule(ctx, id); err != nil {
			return err
		}

		start, end := cur.StartAt, cur.EndAt
		if in.StartAt != nil {
			start = in.StartAt.UTC()
		}
		if in.EndAt != nil {
			end = in.EndAt.UTC()
		}
		if p := validateWindow(start, end); len(p) > 0 {
			return httpx.ValidationError(p...)
		}

		conflicts, err := findConflicts(ctx, q, cur.RoomID, []Window{{Start: start, End: end}}, id)
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			return conflictError(conflicts)
		}

		row, err := q.UpdateSchedule(ctx, dbgen.UpdateScheduleParams{
			ID: id, Title: pgtype.Text{String: deref(title), Valid: title != nil},
			StartAt: start, EndAt: end, UpdatedAt: s.clock.Now(),
		})
		if err != nil {
			return err
		}
		updated = fromRow(row)
		return nil
	})
	if err != nil {
		return Schedule{}, mapWriteError(err, "updating schedule")
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, actor httpx.Principal, id uuid.UUID) error {
	if !canManage(actor) {
		return httpx.ErrForbidden
	}
	n, err := s.q.DeleteSchedule(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting schedule: %w", err)
	}
	if n == 0 {
		return httpx.ErrNotFound
	}
	return nil
}

// findConflicts lists existing schedules and live bookings (MENUNGGU holds the slot, BR-03) that
// overlap any window. Intervals are half-open, so back-to-back sessions do not collide (BR-02).
// excludeID is the schedule being edited, which must not conflict with itself.
func findConflicts(ctx context.Context, q *dbgen.Queries, roomID uuid.UUID, windows []Window, excludeID uuid.UUID) ([]httpx.ErrorDetail, error) {
	var out []httpx.ErrorDetail
	seen := map[uuid.UUID]bool{}

	for _, w := range windows {
		scheds, err := q.ListSchedulesOverlapping(ctx, dbgen.ListSchedulesOverlappingParams{
			RoomIds: []uuid.UUID{roomID}, RangeStart: w.Start, RangeEnd: w.End,
		})
		if err != nil {
			return nil, fmt.Errorf("checking schedule overlaps: %w", err)
		}
		for _, c := range scheds {
			if c.ID == excludeID || seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			out = append(out, httpx.ErrorDetail{
				Kind:    "SCHEDULE",
				Message: fmt.Sprintf("Bentrok dengan jadwal %q.", c.Title),
				StartAt: clock.ToWIB(c.StartAt).Format(time.RFC3339),
				EndAt:   clock.ToWIB(c.EndAt).Format(time.RFC3339),
			})
		}

		books, err := q.ListBookingsOverlapping(ctx, dbgen.ListBookingsOverlappingParams{
			RoomIds:    []uuid.UUID{roomID},
			Statuses:   []string{string(dbgen.BookingStatusMENUNGGU), string(dbgen.BookingStatusDISETUJUI)},
			RangeStart: w.Start, RangeEnd: w.End,
		})
		if err != nil {
			return nil, fmt.Errorf("checking booking overlaps: %w", err)
		}
		for _, c := range books {
			if seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			out = append(out, httpx.ErrorDetail{
				Kind:    "BOOKING",
				Status:  string(c.Status),
				Message: fmt.Sprintf("Bentrok dengan booking %s.", strings.ToLower(string(c.Status))),
				StartAt: clock.ToWIB(c.StartAt).Format(time.RFC3339),
				EndAt:   clock.ToWIB(c.EndAt).Format(time.RFC3339),
			})
		}
	}

	if len(out) > maxConflictList {
		out = out[:maxConflictList]
	}
	return out, nil
}

func conflictError(details []httpx.ErrorDetail) error {
	return httpx.NewCustomError(http.StatusConflict, "BOOKING_CONFLICT",
		"Jadwal bentrok dengan jadwal atau booking yang sudah ada.", details, httpx.ErrBookingConflict)
}

// mapWriteError turns storage errors into API errors. The exclusion constraint is the last line of
// defence (brief 4.4); the room lock should make it unreachable, but a race must still be a 409.
func mapWriteError(err error, op string) error {
	var domain *httpx.CustomDomainError
	if errors.As(err, &domain) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgExclusionViolation {
		return conflictError(nil)
	}
	return fmt.Errorf("%s: %w", op, err)
}

func validateTitle(v string) []httpx.ErrorDetail {
	if n := utf8.RuneCountInString(v); n < 1 || n > maxTitleRunes {
		return []httpx.ErrorDetail{{Field: "title", Message: fmt.Sprintf("Judul wajib diisi (maksimal %d karakter).", maxTitleRunes)}}
	}
	return nil
}

// validateWindow requires start < end on one calendar day in WIB. A session crossing midnight is
// not a lecture, and keeping to one day makes weekly expansion unambiguous.
func validateWindow(start, end time.Time) []httpx.ErrorDetail {
	if start.IsZero() || end.IsZero() {
		return []httpx.ErrorDetail{{Field: "start_at", Message: "start_at dan end_at wajib diisi."}}
	}
	if !start.Before(end) {
		return []httpx.ErrorDetail{{Field: "end_at", Message: "end_at harus setelah start_at."}}
	}
	s, e := clock.ToWIB(start), clock.ToWIB(end)
	if s.Year() != e.Year() || s.YearDay() != e.YearDay() {
		return []httpx.ErrorDetail{{Field: "end_at", Message: "Jadwal harus berada pada hari yang sama (WIB)."}}
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
