// Package reports handles damage reports for rooms: creation with an optional photo, and the
// BARU -> DITERIMA -> DIPROSES -> SELESAI lifecycle with its append-only history (BR-08).
package reports

import (
	"time"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/rooms"

	"github.com/google/uuid"
)

// Reporter is the person who filed the report, as shown to handlers.
type Reporter struct {
	ID         uuid.UUID `json:"id"`
	Identifier string    `json:"identifier"`
	Name       string    `json:"name"`
}

type Report struct {
	ID          uuid.UUID  `json:"id"`
	RoomID      uuid.UUID  `json:"room_id"`
	Room        rooms.Room `json:"room"`
	ReporterID  uuid.UUID  `json:"reporter_id"`
	Reporter    Reporter   `json:"reporter"`
	Description string     `json:"description"`
	// AttachmentID is the photo, opened through GET /attachments/{id} by those allowed to see the report.
	AttachmentID *uuid.UUID `json:"attachment_id"`
	Status       string     `json:"status"`
	HandledBy    *uuid.UUID `json:"handled_by,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Detail is a report together with its history, for GET /reports/{id}.
type Detail struct {
	Report
	Events []Event `json:"events"`
}

type Event struct {
	ID         uuid.UUID `json:"id"`
	ReportID   uuid.UUID `json:"report_id"`
	ActorID    uuid.UUID `json:"actor_id"`
	FromStatus *string   `json:"from_status,omitempty"`
	ToStatus   string    `json:"to_status"`
	Note       *string   `json:"note,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func fromRow(r dbgen.GetReportRow) Report {
	rep := Report{
		ID:     r.ID,
		RoomID: r.RoomID,
		Room: rooms.FromRow(rooms.Row{
			ID: r.RoomID, OrgUnitID: r.RoomOrgUnitID, Code: r.RoomCode, Name: r.RoomName,
			Type: r.RoomType, Capacity: r.RoomCapacity, Facilities: r.RoomFacilities, IsActive: r.RoomIsActive,
			OpenTime: r.RoomOpenTime, CloseTime: r.RoomCloseTime,
		}),
		ReporterID:  r.ReporterID,
		Reporter:    Reporter{ID: r.ReporterID, Identifier: r.ReporterIdentifier, Name: r.ReporterName},
		Description: r.Description,
		Status:      string(r.Status),
		CreatedAt:   clock.ToWIB(r.CreatedAt),
	}
	if r.AttachmentID.Valid {
		id := uuid.UUID(r.AttachmentID.Bytes)
		rep.AttachmentID = &id
	}
	if r.HandledBy.Valid {
		id := uuid.UUID(r.HandledBy.Bytes)
		rep.HandledBy = &id
	}
	return rep
}

func eventFromRow(r dbgen.ReportEvent) Event {
	e := Event{
		ID: r.ID, ReportID: r.ReportID, ActorID: r.ActorID,
		ToStatus: string(r.ToStatus), CreatedAt: clock.ToWIB(r.CreatedAt),
	}
	if r.FromStatus.Valid {
		s := string(r.FromStatus.ReportStatus)
		e.FromStatus = &s
	}
	if r.Note.Valid {
		e.Note = &r.Note.String
	}
	return e
}
