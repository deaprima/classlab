// Package schedules manages academic and manual room schedules (room_schedules).
package schedules

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"classlab/api/internal/audit"
	"classlab/api/internal/httpx"
	"classlab/api/internal/overlap"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/roomtypes"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	maxTitleRunes = 200
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

// scheduleRow is the set of room_schedules columns the queries return; each sqlc row type with these
// fields converts to it directly.
type scheduleRow struct {
	ID        uuid.UUID
	RoomID    uuid.UUID
	Title     string
	StartAt   time.Time
	EndAt     time.Time
	Source    dbgen.ScheduleSource
	CreatedBy uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

func fromRow(r scheduleRow) Schedule {
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
	types *roomtypes.Store
}

func NewService(pool *db.Pool, clk clock.Clock, types *roomtypes.Store) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), clock: clk, types: types}
}

// Create inserts one schedule, or one per week when Repeat is set (BR-09). Everything happens in a
// single transaction under the room lock: if any session collides, nothing is written and the
// response lists every collision.
func (s *Service) Create(ctx context.Context, actor httpx.Principal, in CreateInput) ([]Schedule, error) {
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
		room, err := q.LockRoom(ctx, in.RoomID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.ValidationError(httpx.ErrorDetail{Field: "room_id", Message: "Ruang tidak ditemukan."})
			}
			return fmt.Errorf("locking room: %w", err)
		}
		// Schedules follow the same authority as rooms: the approvers of the room's type and administrators.
		if err := s.types.RequireRoomManager(ctx, actor, string(room.Type)); err != nil {
			return err
		}

		conflicts, err := overlap.Find(ctx, q, in.RoomID, windows, uuid.Nil, uuid.Nil)
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
			created = append(created, fromRow(scheduleRow(row)))
		}
		// One line for the whole request; a weekly series is one decision, not 26.
		return audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionScheduleCreated, EntityType: audit.EntitySchedule,
			EntityID: created[0].ID, After: created, At: now,
		})
	})
	if err != nil {
		return nil, mapWriteError(err, "creating schedule")
	}
	return created, nil
}

func (s *Service) Update(ctx context.Context, actor httpx.Principal, id uuid.UUID, in UpdateInput) (Schedule, error) {
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
		room, err := q.LockRoom(ctx, cur.RoomID)
		if err != nil {
			return err
		}
		if err := s.types.RequireRoomManager(ctx, actor, string(room.Type)); err != nil {
			return err
		}
		if cur, err = q.GetSchedule(ctx, id); err != nil {
			return err
		}
		before := fromRow(scheduleRow(cur))

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

		conflicts, err := overlap.Find(ctx, q, cur.RoomID, []Window{{Start: start, End: end}}, id, uuid.Nil)
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			return conflictError(conflicts)
		}

		now := s.clock.Now()
		row, err := q.UpdateSchedule(ctx, dbgen.UpdateScheduleParams{
			ID: id, Title: pgtype.Text{String: deref(title), Valid: title != nil},
			StartAt: start, EndAt: end, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		updated = fromRow(scheduleRow(row))
		return audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionScheduleUpdated, EntityType: audit.EntitySchedule,
			EntityID: id, Before: before, After: updated, At: now,
		})
	})
	if err != nil {
		return Schedule{}, mapWriteError(err, "updating schedule")
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, actor httpx.Principal, id uuid.UUID) error {
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)

		cur, err := q.GetSchedule(ctx, id)
		if err != nil {
			return err
		}
		room, err := q.LockRoom(ctx, cur.RoomID)
		if err != nil {
			return err
		}
		if err := s.types.RequireRoomManager(ctx, actor, string(room.Type)); err != nil {
			return err
		}
		n, err := q.DeleteSchedule(ctx, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return pgx.ErrNoRows
		}
		return audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionScheduleDeleted, EntityType: audit.EntitySchedule,
			EntityID: id, Before: fromRow(scheduleRow(cur)), At: s.clock.Now(),
		})
	})
	if err != nil {
		return mapWriteError(err, "deleting schedule")
	}
	return nil
}

func conflictError(details []httpx.ErrorDetail) error {
	return overlap.Error("Jadwal bentrok dengan jadwal atau booking yang sudah ada.", details)
}

func mapWriteError(err error, op string) error {
	return overlap.MapWriteError(err, op, "Jadwal bentrok dengan jadwal atau booking yang sudah ada.")
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
