// Package rooms manages classroom and laboratory master data.
package rooms

import (
	"github.com/google/uuid"

	"classlab/api/internal/platform/db/dbgen"
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
}

// FromRow is exported because availability responses embed the same room shape.
func FromRow(r dbgen.Room) Room {
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
	}
}
