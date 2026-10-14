// Package notifications serves a user's in-app notifications: the list, read markers and a live stream.
// Other modules create notifications in their own transactions (see db/queries/notifications.sql).
package notifications

import (
	"time"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
)

type Notification struct {
	ID        uuid.UUID  `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	RefType   *string    `json:"ref_type,omitempty"`
	RefID     *uuid.UUID `json:"ref_id,omitempty"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func fromRow(r dbgen.Notification) Notification {
	n := Notification{
		ID: r.ID, Type: r.Type, Title: r.Title, Body: r.Body,
		CreatedAt: clock.ToWIB(r.CreatedAt),
	}
	if r.RefType.Valid {
		n.RefType = &r.RefType.String
	}
	if r.RefID.Valid {
		id := uuid.UUID(r.RefID.Bytes)
		n.RefID = &id
	}
	if r.ReadAt.Valid {
		t := clock.ToWIB(r.ReadAt.Time)
		n.ReadAt = &t
	}
	return n
}
