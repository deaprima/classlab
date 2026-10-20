// Package users manages accounts and exposes the shared User representation. Administrators manage every
// account; TU may manage MAHASISWA accounts only (brief section 3, BR-24).
package users

import (
	"time"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// User is the API representation of an account. It never carries the password hash.
type User struct {
	ID                  uuid.UUID  `json:"id"`
	OrgUnitID           uuid.UUID  `json:"org_unit_id"`
	Identifier          string     `json:"identifier"`
	Name                string     `json:"name"`
	Email               *string    `json:"email"`
	Role                string     `json:"role"`
	IsActive            bool       `json:"is_active"`
	BookingBlockedUntil *time.Time `json:"booking_blocked_until"`
	PicName             *string    `json:"pic_name"`
	PicPhone            *string    `json:"pic_phone"`
	MustChangePassword  bool       `json:"must_change_password"`
	CreatedAt           time.Time  `json:"created_at"`
	// ActivePoints is only set on /auth/me, for roles that can book (BR-14).
	ActivePoints *int `json:"active_points,omitempty"`
}

// userRow has the same shape as the sqlc row types of every user query that omits password_hash,
// so each of them converts with a plain type conversion.
type userRow struct {
	ID                  uuid.UUID
	OrgUnitID           uuid.UUID
	Identifier          string
	Name                string
	Email               pgtype.Text
	Role                dbgen.UserRole
	IsActive            bool
	BookingBlockedUntil pgtype.Timestamptz
	PicName             pgtype.Text
	PicPhone            pgtype.Text
	MustChangePassword  bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (r userRow) toUser() User {
	return User{
		ID:                  r.ID,
		OrgUnitID:           r.OrgUnitID,
		Identifier:          r.Identifier,
		Name:                r.Name,
		Email:               textPtr(r.Email),
		Role:                string(r.Role),
		IsActive:            r.IsActive,
		BookingBlockedUntil: timePtr(r.BookingBlockedUntil),
		PicName:             textPtr(r.PicName),
		PicPhone:            textPtr(r.PicPhone),
		MustChangePassword:  r.MustChangePassword,
		CreatedAt:           clock.ToWIB(r.CreatedAt),
	}
}

// FromLoginRow builds the User returned by a successful login.
func FromLoginRow(r dbgen.GetUserForLoginRow) User {
	return userRow{
		ID: r.ID, OrgUnitID: r.OrgUnitID, Identifier: r.Identifier, Name: r.Name, Email: r.Email,
		Role: r.Role, IsActive: r.IsActive, BookingBlockedUntil: r.BookingBlockedUntil,
		PicName: r.PicName, PicPhone: r.PicPhone, MustChangePassword: r.MustChangePassword,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}.toUser()
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := clock.ToWIB(t.Time)
	return &v
}
