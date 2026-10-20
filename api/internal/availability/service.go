// Package availability answers "is this room free?" by combining schedules and bookings.
// Status is computed per request and never stored (brief 4.2).
package availability

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/rooms"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// maxRange bounds how much one request may scan; a month is enough for any calendar view.
const maxRange = 31 * 24 * time.Hour

const (
	StatusTersedia  = "TERSEDIA"
	StatusDigunakan = "DIGUNAKAN"
	StatusDiajukan  = "DIAJUKAN"
)

type Conflict struct {
	Kind    string    `json:"kind"`
	Title   string    `json:"title"`
	Status  string    `json:"status,omitempty"` // BOOKING only: MENUNGGU or DISETUJUI
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
}

type RoomAvailability struct {
	Room      rooms.Room `json:"room"`
	Status    string     `json:"status"`
	Conflicts []Conflict `json:"conflicts"`
}

type TimelineItem struct {
	ID      uuid.UUID `json:"id"`
	Kind    string    `json:"kind"`
	Title   string    `json:"title"`
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
}

type Filter struct {
	Start       time.Time
	End         time.Time
	Type        *string
	CapacityMin *int
}

type Service struct {
	q *dbgen.Queries
}

func NewService(pool *db.Pool) *Service {
	return &Service{q: dbgen.New(pool)}
}

// Check reports the status of every bookable room (active, not deleted) for [Start, End).
// Priority is DIGUNAKAN > DIAJUKAN > TERSEDIA: an academic schedule or approved booking beats a
// pending request, which in turn beats a free slot.
func (s *Service) Check(ctx context.Context, f Filter) ([]RoomAvailability, error) {
	if p := validateRange("start_at", "end_at", f.Start, f.End); len(p) > 0 {
		return nil, httpx.ValidationError(p...)
	}

	var typ dbgen.NullRoomType
	if f.Type != nil {
		typ = dbgen.NullRoomType{RoomType: dbgen.RoomType(*f.Type), Valid: true}
	}
	var capMin pgtype.Int4
	if f.CapacityMin != nil {
		capMin = pgtype.Int4{Int32: int32(*f.CapacityMin), Valid: true}
	}

	roomRows, err := s.q.ListBookableRooms(ctx, dbgen.ListBookableRoomsParams{Type: typ, CapacityMin: capMin})
	if err != nil {
		return nil, fmt.Errorf("listing bookable rooms: %w", err)
	}

	ids := make([]uuid.UUID, len(roomRows))
	for i, r := range roomRows {
		ids[i] = r.ID
	}

	// Two queries for all rooms together, not two per room.
	scheds, err := s.q.ListSchedulesOverlapping(ctx, dbgen.ListSchedulesOverlappingParams{
		RoomIds: ids, RangeStart: f.Start, RangeEnd: f.End,
	})
	if err != nil {
		return nil, fmt.Errorf("loading schedules: %w", err)
	}
	books, err := s.q.ListBookingsOverlapping(ctx, dbgen.ListBookingsOverlappingParams{
		RoomIds:    ids,
		Statuses:   []string{string(dbgen.BookingStatusMENUNGGU), string(dbgen.BookingStatusDISETUJUI)},
		RangeStart: f.Start, RangeEnd: f.End,
	})
	if err != nil {
		return nil, fmt.Errorf("loading bookings: %w", err)
	}

	out := make([]RoomAvailability, len(roomRows))
	index := make(map[uuid.UUID]int, len(roomRows))
	for i, r := range roomRows {
		out[i] = RoomAvailability{Room: rooms.FromRow(rooms.Row(r)), Status: StatusTersedia, Conflicts: []Conflict{}}
		index[r.ID] = i
	}
	for _, c := range scheds {
		ra := &out[index[c.RoomID]]
		ra.Status = StatusDigunakan
		ra.Conflicts = append(ra.Conflicts, Conflict{
			Kind: "SCHEDULE", Title: c.Title, StartAt: clock.ToWIB(c.StartAt), EndAt: clock.ToWIB(c.EndAt),
		})
	}
	for _, c := range books {
		ra := &out[index[c.RoomID]]
		switch c.Status {
		case dbgen.BookingStatusDISETUJUI:
			ra.Status = StatusDigunakan
		case dbgen.BookingStatusMENUNGGU:
			if ra.Status == StatusTersedia {
				ra.Status = StatusDiajukan
			}
		}
		ra.Conflicts = append(ra.Conflicts, Conflict{
			Kind: "BOOKING", Title: c.Purpose, Status: string(c.Status),
			StartAt: clock.ToWIB(c.StartAt), EndAt: clock.ToWIB(c.EndAt),
		})
	}
	for i := range out {
		sortConflicts(out[i].Conflicts)
	}
	return out, nil
}

// Timeline merges a room's academic schedules with its approved bookings (BR-05). Approved
// bookings are an overlay: they never change room_schedules. Pending bookings are not shown here.
func (s *Service) Timeline(ctx context.Context, roomID uuid.UUID, from, to time.Time) ([]TimelineItem, error) {
	if p := validateRange("from", "to", from, to); len(p) > 0 {
		return nil, httpx.ValidationError(p...)
	}
	if _, err := s.q.GetRoom(ctx, roomID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.ErrNotFound
		}
		return nil, fmt.Errorf("getting room: %w", err)
	}

	scheds, err := s.q.ListSchedulesOverlapping(ctx, dbgen.ListSchedulesOverlappingParams{
		RoomIds: []uuid.UUID{roomID}, RangeStart: from, RangeEnd: to,
	})
	if err != nil {
		return nil, fmt.Errorf("loading schedules: %w", err)
	}
	books, err := s.q.ListBookingsOverlapping(ctx, dbgen.ListBookingsOverlappingParams{
		RoomIds:    []uuid.UUID{roomID},
		Statuses:   []string{string(dbgen.BookingStatusDISETUJUI)},
		RangeStart: from, RangeEnd: to,
	})
	if err != nil {
		return nil, fmt.Errorf("loading bookings: %w", err)
	}

	items := make([]TimelineItem, 0, len(scheds)+len(books))
	for _, c := range scheds {
		items = append(items, TimelineItem{ID: c.ID, Kind: "SCHEDULE", Title: c.Title, StartAt: clock.ToWIB(c.StartAt), EndAt: clock.ToWIB(c.EndAt)})
	}
	for _, c := range books {
		items = append(items, TimelineItem{ID: c.ID, Kind: "BOOKING", Title: c.Purpose, StartAt: clock.ToWIB(c.StartAt), EndAt: clock.ToWIB(c.EndAt)})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].StartAt.Equal(items[j].StartAt) {
			return items[i].StartAt.Before(items[j].StartAt)
		}
		return items[i].ID.String() < items[j].ID.String()
	})
	return items, nil
}

func sortConflicts(c []Conflict) {
	sort.SliceStable(c, func(i, j int) bool { return c[i].StartAt.Before(c[j].StartAt) })
}

func validateRange(startName, endName string, start, end time.Time) []httpx.ErrorDetail {
	if !start.Before(end) {
		return []httpx.ErrorDetail{{Field: endName, Message: endName + " harus setelah " + startName + "."}}
	}
	if end.Sub(start) > maxRange {
		return []httpx.ErrorDetail{{Field: endName, Message: "Rentang waktu maksimal 31 hari."}}
	}
	return nil
}
