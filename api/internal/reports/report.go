// Package reports handles damage reports for rooms: creation with an optional photo, and the
// BARU -> DITERIMA -> DIPROSES -> SELESAI lifecycle with its append-only history (BR-08).
package reports

import (
	"fmt"
	"time"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/rooms"

	"github.com/google/uuid"
)

// photoPathFormat is where a report's photo is served. photo_path in responses is this URL path, not
// a file system path: the stored file name never leaves the server.
const photoPathFormat = "/api/v1/reports/%s/photo"

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
	PhotoPath   *string    `json:"photo_path,omitempty"`
	Status      string     `json:"status"`
	HandledBy   *uuid.UUID `json:"handled_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
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
		Room: rooms.FromRow(dbgen.Room{
			ID: r.RoomID, OrgUnitID: r.RoomOrgUnitID, Code: r.RoomCode, Name: r.RoomName,
			Type: r.RoomType, Capacity: r.RoomCapacity, Facilities: r.RoomFacilities, IsActive: r.RoomIsActive,
		}),
		ReporterID:  r.ReporterID,
		Reporter:    Reporter{ID: r.ReporterID, Identifier: r.ReporterIdentifier, Name: r.ReporterName},
		Description: r.Description,
		Status:      string(r.Status),
		CreatedAt:   clock.ToWIB(r.CreatedAt),
	}
	if r.PhotoPath.Valid {
		p := photoURL(r.ID)
		rep.PhotoPath = &p
	}
	if r.HandledBy.Valid {
		id := uuid.UUID(r.HandledBy.Bytes)
		rep.HandledBy = &id
	}
	return rep
}

func photoURL(reportID uuid.UUID) string { return fmt.Sprintf(photoPathFormat, reportID) }

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
