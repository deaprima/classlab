// Package roomtypes reads the per-room-type policy (brief BR-20): who approves, who may request,
// hours, durations and whether a letter is needed. Behaviour that differs between room types lives in
// that data, never in code that branches on the type.
package roomtypes

import (
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/db/dbgen"
)

// Policy is one row of room_type_policies.
type Policy struct {
	Type           string
	Label          string
	ApproverRoles  []httpx.Role
	RequesterRoles []httpx.Role
	ApprovalPolicy string // AUTO_IF_ELIGIBLE or ALWAYS_MANUAL
	LetterRequired bool
	OpenMinute     int // minutes after midnight WIB
	CloseMinute    int
	MinDuration    time.Duration
	MaxDuration    time.Duration
	MaxAdvanceDays int
	UpdatedAt      time.Time
}

// CanApprove reports whether role may approve, reject and cancel bookings of this type, handle its
// reports and see its room QR codes (BR-10).
func (p Policy) CanApprove(role httpx.Role) bool {
	return slices.Contains(p.ApproverRoles, role)
}

// CanManageRooms reports whether role may create and change rooms and manual schedules of this type.
// Administrators manage every type even when they are not approvers of it (BR-10).
func (p Policy) CanManageRooms(role httpx.Role) bool {
	return role == httpx.RoleAdmin || p.CanApprove(role)
}

// CanRequest reports whether role may submit bookings for this type.
func (p Policy) CanRequest(role httpx.Role) bool {
	return slices.Contains(p.RequesterRoles, role)
}

// policyRow is the shape both policy queries return; each sqlc row type converts to it directly.
type policyRow struct {
	RoomType       dbgen.RoomType
	Label          string
	ApproverRoles  []string
	RequesterRoles []string
	ApprovalPolicy dbgen.ApprovalPolicy
	LetterRequired bool
	OpenTime       pgtype.Time
	CloseTime      pgtype.Time
	MinDurationMin int32
	MaxDurationMin int32
	MaxAdvanceDays int32
	UpdatedAt      time.Time
}

func fromRow(r policyRow) Policy {
	return Policy{
		Type:           string(r.RoomType),
		Label:          r.Label,
		ApproverRoles:  toRoles(r.ApproverRoles),
		RequesterRoles: toRoles(r.RequesterRoles),
		ApprovalPolicy: string(r.ApprovalPolicy),
		LetterRequired: r.LetterRequired,
		OpenMinute:     minutesOfDay(r.OpenTime.Microseconds),
		CloseMinute:    minutesOfDay(r.CloseTime.Microseconds),
		MinDuration:    time.Duration(r.MinDurationMin) * time.Minute,
		MaxDuration:    time.Duration(r.MaxDurationMin) * time.Minute,
		MaxAdvanceDays: int(r.MaxAdvanceDays),
		UpdatedAt:      r.UpdatedAt,
	}
}

func toRoles(in []string) []httpx.Role {
	out := make([]httpx.Role, len(in))
	for i, r := range in {
		out[i] = httpx.Role(r)
	}
	return out
}

// minutesOfDay converts a Postgres time (microseconds since midnight) to minutes.
func minutesOfDay(micros int64) int {
	return int(micros / int64(time.Minute/time.Microsecond))
}
