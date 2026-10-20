package bookings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/violations"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ListFilter struct {
	Status   *string
	RoomID   *uuid.UUID
	RoomType *string
	GroupID  *uuid.UUID
	From     *time.Time // bookings still running after this instant
	To       *time.Time // bookings starting before this instant
	Mine     bool
	Page     int
	PerPage  int
}

// Get returns one booking. An approver also gets a summary of the requester: violation points and course
// context, which is what they need to decide (BR-14, BR-24).
func (s *Service) Get(ctx context.Context, actor httpx.Principal, id uuid.UUID) (Booking, error) {
	if !canRead(actor) {
		return Booking{}, httpx.ErrForbidden
	}
	row, err := s.q.GetBooking(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, httpx.ErrNotFound
	}
	if err != nil {
		return Booking{}, fmt.Errorf("reading booking: %w", err)
	}
	if err := s.visibleTo(ctx, actor, row.RequesterID, string(row.RoomType)); err != nil {
		return Booking{}, err
	}
	b := fromRow(row)

	if isManager(actor) {
		summary, err := s.requesterSummary(ctx, row.RequesterID)
		if err != nil {
			return Booking{}, err
		}
		b.RequesterSummary = &summary
	}
	return b, nil
}

func (s *Service) requesterSummary(ctx context.Context, requesterID uuid.UUID) (RequesterSummary, error) {
	points, err := violations.ActivePoints(ctx, s.q, s.settings.Violations, requesterID, s.clock.Now())
	if err != nil {
		return RequesterSummary{}, err
	}
	rows, err := s.q.ListUserCourses(ctx, requesterID)
	if err != nil {
		return RequesterSummary{}, fmt.Errorf("listing courses: %w", err)
	}
	out := RequesterSummary{ActivePoints: points, Courses: make([]Course, len(rows))}
	for i, r := range rows {
		out.Courses[i] = Course{Semester: r.Semester, CourseCode: textPtr(r.CourseCode), CourseName: textPtr(r.CourseName), ClassName: textPtr(r.ClassName)}
	}
	return out, nil
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// Events returns a booking's trail. It is not paginated: a booking has a handful of events.
func (s *Service) Events(ctx context.Context, actor httpx.Principal, id uuid.UUID) ([]Event, error) {
	if !canRead(actor) {
		return nil, httpx.ErrForbidden
	}
	row, err := s.q.GetBooking(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reading booking: %w", err)
	}
	if err := s.visibleTo(ctx, actor, row.RequesterID, string(row.RoomType)); err != nil {
		return nil, err
	}
	rows, err := s.q.ListBookingEvents(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("listing booking events: %w", err)
	}
	out := make([]Event, len(rows))
	for i, r := range rows {
		out[i] = eventFromRow(r)
	}
	return out, nil
}

func (s *Service) List(ctx context.Context, actor httpx.Principal, f ListFilter) ([]Booking, int64, error) {
	if !canRead(actor) {
		return nil, 0, httpx.ErrForbidden
	}

	// Requesters are pinned to their own bookings whatever they ask for. TU and KALAB see the room types
	// they approve; ADMIN sees all (BR-10). Managers may narrow with ?mine=.
	var requester pgtype.UUID
	var roomTypes []string
	switch {
	case isRequester(actor) || f.Mine:
		requester = pgtype.UUID{Bytes: actor.UserID, Valid: true}
	case actor.Role != httpx.RoleAdmin:
		types, err := s.types.ApprovableTypes(ctx, actor.Role)
		if err != nil {
			return nil, 0, err
		}
		// types is non-nil even when empty: an empty array matches no room type, NULL would match all of them.
		roomTypes = types
	}
	var status dbgen.NullBookingStatus
	if f.Status != nil {
		status = dbgen.NullBookingStatus{BookingStatus: dbgen.BookingStatus(*f.Status), Valid: true}
	}
	var room, group pgtype.UUID
	if f.RoomID != nil {
		room = pgtype.UUID{Bytes: *f.RoomID, Valid: true}
	}
	if f.GroupID != nil {
		group = pgtype.UUID{Bytes: *f.GroupID, Valid: true}
	}
	var roomType pgtype.Text
	if f.RoomType != nil {
		roomType = pgtype.Text{String: *f.RoomType, Valid: true}
	}
	var from, to pgtype.Timestamptz
	if f.From != nil {
		from = pgtype.Timestamptz{Time: f.From.UTC(), Valid: true}
	}
	if f.To != nil {
		to = pgtype.Timestamptz{Time: f.To.UTC(), Valid: true}
	}
	// The approvers' queue lists what expires soonest first (BR-22).
	queue := isManager(actor) && f.Status != nil && *f.Status == string(dbgen.BookingStatusMENUNGGU)

	rows, err := s.q.ListBookings(ctx, dbgen.ListBookingsParams{
		RequesterID: requester, Status: status, RoomID: room, GroupID: group, RangeStart: from, RangeEnd: to,
		RoomTypes: roomTypes, RoomType: roomType, QueueOrder: queue,
		PageLimit: int32(f.PerPage), PageOffset: int32((f.Page - 1) * f.PerPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing bookings: %w", err)
	}
	total, err := s.q.CountBookings(ctx, dbgen.CountBookingsParams{
		RequesterID: requester, Status: status, RoomID: room, GroupID: group, RangeStart: from, RangeEnd: to,
		RoomTypes: roomTypes, RoomType: roomType,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("counting bookings: %w", err)
	}

	out := make([]Booking, len(rows))
	for i, r := range rows {
		out[i] = fromRow(dbgen.GetBookingRow(r))
	}
	return out, total, nil
}
