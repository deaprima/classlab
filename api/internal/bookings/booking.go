// Package bookings runs the booking workflow: request, approve, reject, cancel, and the audit trail.
package bookings

import (
	"time"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/rooms"

	"github.com/google/uuid"
)

// Requester is the borrower as shown to approvers: enough to recognise them, nothing more.
type Requester struct {
	ID         uuid.UUID `json:"id"`
	Identifier string    `json:"identifier"`
	Name       string    `json:"name"`
}

type Booking struct {
	ID             uuid.UUID  `json:"id"`
	RoomID         uuid.UUID  `json:"room_id"`
	Room           rooms.Room `json:"room"`
	RequesterID    uuid.UUID  `json:"requester_id"`
	Requester      Requester  `json:"requester"`
	StartAt        time.Time  `json:"start_at"`
	EndAt          time.Time  `json:"end_at"`
	Purpose        string     `json:"purpose"`
	Participants   *int       `json:"participants,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
	Status         string     `json:"status"`
	DecidedBy      *uuid.UUID `json:"decided_by,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	DecisionReason *string    `json:"decision_reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Event struct {
	ID         uuid.UUID  `json:"id"`
	BookingID  uuid.UUID  `json:"booking_id"`
	ActorID    *uuid.UUID `json:"actor_id"` // null when the system made the change (job, auto-approval)
	FromStatus *string    `json:"from_status,omitempty"`
	ToStatus   string     `json:"to_status"`
	Reason     *string    `json:"reason,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func fromRow(r dbgen.GetBookingRow) Booking {
	b := Booking{
		ID:     r.ID,
		RoomID: r.RoomID,
		Room: rooms.FromRow(rooms.Row{
			ID: r.RoomID, OrgUnitID: r.RoomOrgUnitID, Code: r.RoomCode, Name: r.RoomName,
			Type: r.RoomType, Capacity: r.RoomCapacity, Facilities: r.RoomFacilities, IsActive: r.RoomIsActive,
			OpenTime: r.RoomOpenTime, CloseTime: r.RoomCloseTime,
		}),
		RequesterID: r.RequesterID,
		Requester:   Requester{ID: r.RequesterID, Identifier: r.RequesterIdentifier, Name: r.RequesterName},
		StartAt:     clock.ToWIB(r.StartAt),
		EndAt:       clock.ToWIB(r.EndAt),
		Purpose:     r.Purpose,
		Status:      string(r.Status),
		CreatedAt:   clock.ToWIB(r.CreatedAt),
	}
	if r.Participants.Valid {
		n := int(r.Participants.Int32)
		b.Participants = &n
	}
	if r.Notes.Valid {
		b.Notes = &r.Notes.String
	}
	if r.DecidedBy.Valid {
		id := uuid.UUID(r.DecidedBy.Bytes)
		b.DecidedBy = &id
	}
	if r.DecidedAt.Valid {
		t := clock.ToWIB(r.DecidedAt.Time)
		b.DecidedAt = &t
	}
	if r.DecisionReason.Valid {
		b.DecisionReason = &r.DecisionReason.String
	}
	return b
}

func eventFromRow(r dbgen.BookingEvent) Event {
	e := Event{
		ID: r.ID, BookingID: r.BookingID,
		ToStatus: string(r.ToStatus), CreatedAt: clock.ToWIB(r.CreatedAt),
	}
	if r.ActorID.Valid {
		id := uuid.UUID(r.ActorID.Bytes)
		e.ActorID = &id
	}
	if r.FromStatus.Valid {
		s := string(r.FromStatus.BookingStatus)
		e.FromStatus = &s
	}
	if r.Reason.Valid {
		e.Reason = &r.Reason.String
	}
	return e
}
