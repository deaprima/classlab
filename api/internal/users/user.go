// Package users manages accounts (Kaprodi only) and exposes the shared User representation.
package users

import (
	"time"

	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
)

// User is the API representation of an account. It never carries the password hash.
type User struct {
	ID         uuid.UUID `json:"id"`
	OrgUnitID  uuid.UUID `json:"org_unit_id"`
	Identifier string    `json:"identifier"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

// userRow has the same shape as the sqlc row types of every user query that omits password_hash,
// so each of them converts with a plain type conversion.
type userRow struct {
	ID         uuid.UUID
	OrgUnitID  uuid.UUID
	Identifier string
	Name       string
	Email      string
	Role       dbgen.UserRole
	IsActive   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (r userRow) toUser() User {
	return User{
		ID:         r.ID,
		OrgUnitID:  r.OrgUnitID,
		Identifier: r.Identifier,
		Name:       r.Name,
		Email:      r.Email,
		Role:       string(r.Role),
		IsActive:   r.IsActive,
		CreatedAt:  clock.ToWIB(r.CreatedAt),
	}
}
