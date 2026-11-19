// Package rooms manages room master data (classrooms, laboratories and special-purpose rooms).
package rooms

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/roomtypes"
)

// Room is the API representation of a room.
type Room struct {
	ID         uuid.UUID `json:"id"`
	OrgUnitID  uuid.UUID `json:"org_unit_id"`
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Capacity   int       `json:"capacity"`
	Facilities []string  `json:"facilities"`
	IsActive   bool      `json:"is_active"`
	// OpenTime and CloseTime are this room's own operating hours (BR-29), "HH:MM" in WIB, or null when the
	// room follows the default hours of its type.
	OpenTime  *string `json:"open_time"`
	CloseTime *string `json:"close_time"`
}

// Row is the set of room columns every room query returns. qr_token is deliberately not among them: it
// leaves the database only through the QR endpoints, and only to the room's managers (brief BR-11).
// Each sqlc row type with these fields converts to it directly.
type Row struct {
	ID         uuid.UUID
	OrgUnitID  uuid.UUID
	Code       string
	Name       string
	Type       dbgen.RoomType
	Capacity   int32
	Facilities []string
	IsActive   bool
	DeletedAt  pgtype.Timestamptz
	CreatedAt  time.Time
	UpdatedAt  time.Time
	OpenTime   pgtype.Time
	CloseTime  pgtype.Time
}

// FromRow is exported because availability responses embed the same room shape.
func FromRow(r Row) Room {
	facilities := r.Facilities
	if facilities == nil {
		facilities = []string{}
	}
	return Room{
		ID:         r.ID,
		OrgUnitID:  r.OrgUnitID,
		Code:       r.Code,
		Name:       r.Name,
		Type:       string(r.Type),
		Capacity:   int(r.Capacity),
		Facilities: facilities,
		IsActive:   r.IsActive,
		OpenTime:   clockText(r.OpenTime),
		CloseTime:  clockText(r.CloseTime),
	}
}

// NewQRToken returns 32 random bytes as base64url, the secret printed in a room's door QR code.
func NewQRToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating QR token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// clockText renders a Postgres time as "HH:MM", or nil for NULL.
func clockText(t pgtype.Time) *string {
	m, ok := roomtypes.MinutesOf(t)
	if !ok {
		return nil
	}
	s := roomtypes.FormatClock(m)
	return &s
}
