package bookings

import (
	"encoding/json"
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

// Letter is the cover letter of a booking (BR-18).
type Letter struct {
	AttachmentID uuid.UUID `json:"attachment_id"`
	LetterNumber *string   `json:"letter_number"`
}

// Course is the requester's course context from the student import (BR-24).
type Course struct {
	Semester   string  `json:"semester"`
	CourseCode *string `json:"course_code"`
	CourseName *string `json:"course_name"`
	ClassName  *string `json:"class_name"`
}

// RequesterSummary is what an approver sees about the person asking: violation points and course context.
type RequesterSummary struct {
	ActivePoints int      `json:"active_points"`
	Courses      []Course `json:"courses"`
}

type Booking struct {
	ID             uuid.UUID       `json:"id"`
	RoomID         uuid.UUID       `json:"room_id"`
	Room           rooms.Room      `json:"room"`
	RequesterID    uuid.UUID       `json:"requester_id"`
	Requester      Requester       `json:"requester"`
	StartAt        time.Time       `json:"start_at"`
	EndAt          time.Time       `json:"end_at"`
	Purpose        string          `json:"purpose"`
	Participants   *int            `json:"participants,omitempty"`
	Notes          *string         `json:"notes,omitempty"`
	Details        json.RawMessage `json:"details"`
	Status         string          `json:"status"`
	ApprovalMode   string          `json:"approval_mode"`
	ManualReason   *string         `json:"manual_reason"`
	DecidedBy      *uuid.UUID      `json:"decided_by,omitempty"`
	DecidedAt      *time.Time      `json:"decided_at,omitempty"`
	DecisionReason *string         `json:"decision_reason,omitempty"`
	// GroupID, OccurrenceNo and OccurrenceCount describe a multi-date request (BR-27). A booking made with
	// a single date has no group and counts as one occurrence.
	GroupID         *uuid.UUID `json:"group_id"`
	OccurrenceNo    *int       `json:"occurrence_no"`
	OccurrenceCount int        `json:"occurrence_count"`
	ExpiresAt       *time.Time `json:"expires_at"`
	EditedAt        *time.Time `json:"edited_at"`
	Letter          *Letter    `json:"letter"`
	CreatedAt       time.Time  `json:"created_at"`
	// RequesterSummary is only on the detail an approver reads.
	RequesterSummary *RequesterSummary `json:"requester_summary,omitempty"`
}

// OccurrenceRef is one date of a multi-date request in a create response.
type OccurrenceRef struct {
	ID           uuid.UUID `json:"id"`
	OccurrenceNo int       `json:"occurrence_no"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
	Status       string    `json:"status"`
}

// GroupResult is the response to a multi-date request (BR-27).
type GroupResult struct {
	GroupID      uuid.UUID       `json:"group_id"`
	Status       string          `json:"status"`
	ApprovalMode string          `json:"approval_mode"`
	ManualReason *string         `json:"manual_reason"`
	ExpiresAt    *time.Time      `json:"expires_at"`
	Bookings     []OccurrenceRef `json:"bookings"`
}

type Event struct {
	ID         uuid.UUID       `json:"id"`
	BookingID  uuid.UUID       `json:"booking_id"`
	ActorID    *uuid.UUID      `json:"actor_id"` // null when the system made the change (job, auto-approval)
	Kind       string          `json:"kind"`     // STATUS or EDITED
	FromStatus *string         `json:"from_status,omitempty"`
	ToStatus   string          `json:"to_status"`
	Reason     *string         `json:"reason,omitempty"`
	Changes    json.RawMessage `json:"changes,omitempty"` // EDITED: { field: { from, to } }
	CreatedAt  time.Time       `json:"created_at"`
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
		RequesterID:     r.RequesterID,
		Requester:       Requester{ID: r.RequesterID, Identifier: r.RequesterIdentifier, Name: r.RequesterName},
		StartAt:         clock.ToWIB(r.StartAt),
		EndAt:           clock.ToWIB(r.EndAt),
		Purpose:         r.Purpose,
		Details:         json.RawMessage(r.Details),
		Status:          string(r.Status),
		ApprovalMode:    string(r.ApprovalMode),
		OccurrenceCount: max(1, int(r.OccurrenceCount)),
		CreatedAt:       clock.ToWIB(r.CreatedAt),
	}
	if len(b.Details) == 0 {
		b.Details = json.RawMessage("{}")
	}
	if r.Participants.Valid {
		n := int(r.Participants.Int32)
		b.Participants = &n
	}
	if r.Notes.Valid {
		b.Notes = &r.Notes.String
	}
	if r.ManualReason.Valid {
		s := string(r.ManualReason.ManualReason)
		b.ManualReason = &s
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
	if r.GroupID.Valid {
		id := uuid.UUID(r.GroupID.Bytes)
		b.GroupID = &id
	}
	if r.OccurrenceNo.Valid {
		n := int(r.OccurrenceNo.Int32)
		b.OccurrenceNo = &n
	}
	if r.ExpiresAt.Valid {
		t := clock.ToWIB(r.ExpiresAt.Time)
		b.ExpiresAt = &t
	}
	if r.EditedAt.Valid {
		t := clock.ToWIB(r.EditedAt.Time)
		b.EditedAt = &t
	}
	if r.LetterAttachmentID.Valid {
		l := Letter{AttachmentID: uuid.UUID(r.LetterAttachmentID.Bytes)}
		if r.LetterNumber.Valid {
			l.LetterNumber = &r.LetterNumber.String
		}
		b.Letter = &l
	}
	return b
}

func eventFromRow(r dbgen.BookingEvent) Event {
	e := Event{
		ID: r.ID, BookingID: r.BookingID, Kind: r.Kind,
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
	if len(r.Changes) > 0 {
		e.Changes = json.RawMessage(r.Changes)
	}
	return e
}
