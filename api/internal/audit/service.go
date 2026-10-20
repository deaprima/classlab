package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Log is the API representation of one audit line.
type Log struct {
	ID              uuid.UUID       `json:"id"`
	ActorID         *uuid.UUID      `json:"actor_id"`
	ActorRole       *string         `json:"actor_role"`
	ActorIdentifier *string         `json:"actor_identifier"`
	ActorName       *string         `json:"actor_name"`
	Action          string          `json:"action"`
	EntityType      string          `json:"entity_type"`
	EntityID        *uuid.UUID      `json:"entity_id"`
	Before          json.RawMessage `json:"before"`
	After           json.RawMessage `json:"after"`
	IP              *string         `json:"ip"`
	UserAgent       *string         `json:"user_agent"`
	RequestID       *string         `json:"request_id"`
	CreatedAt       time.Time       `json:"created_at"`
}

type ListFilter struct {
	ActorID    *uuid.UUID
	Action     string
	EntityType string
	EntityID   *uuid.UUID
	From       *time.Time
	To         *time.Time
	Page       int
	PerPage    int
}

type Service struct {
	q *dbgen.Queries
}

func NewService(pool *db.Pool) *Service {
	return &Service{q: dbgen.New(pool)}
}

// List is open to administrators only; other roles can write audit lines through their actions but
// never read them (brief section 3).
func (s *Service) List(ctx context.Context, actor httpx.Principal, f ListFilter) ([]Log, int64, error) {
	if actor.Role != httpx.RoleAdmin {
		return nil, 0, httpx.ErrForbidden
	}

	var (
		actorID  pgtype.UUID
		entityID pgtype.UUID
		action   pgtype.Text
		entity   pgtype.Text
		from, to pgtype.Timestamptz
	)
	if f.ActorID != nil {
		actorID = pgtype.UUID{Bytes: *f.ActorID, Valid: true}
	}
	if f.EntityID != nil {
		entityID = pgtype.UUID{Bytes: *f.EntityID, Valid: true}
	}
	if f.Action != "" {
		action = pgtype.Text{String: f.Action, Valid: true}
	}
	if f.EntityType != "" {
		entity = pgtype.Text{String: f.EntityType, Valid: true}
	}
	if f.From != nil {
		from = pgtype.Timestamptz{Time: *f.From, Valid: true}
	}
	if f.To != nil {
		to = pgtype.Timestamptz{Time: *f.To, Valid: true}
	}

	total, err := s.q.CountAuditLogs(ctx, dbgen.CountAuditLogsParams{
		ActorID: actorID, Action: action, EntityType: entity, EntityID: entityID, FromAt: from, ToAt: to,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("counting audit logs: %w", err)
	}
	rows, err := s.q.ListAuditLogs(ctx, dbgen.ListAuditLogsParams{
		ActorID: actorID, Action: action, EntityType: entity, EntityID: entityID, FromAt: from, ToAt: to,
		PageLimit: int32(f.PerPage), PageOffset: int32((f.Page - 1) * f.PerPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing audit logs: %w", err)
	}

	out := make([]Log, len(rows))
	for i, r := range rows {
		out[i] = fromRow(r)
	}
	return out, total, nil
}

func fromRow(r dbgen.ListAuditLogsRow) Log {
	l := Log{
		ID: r.ID, Action: r.Action, EntityType: r.EntityType,
		Before: r.Before, After: r.After,
		CreatedAt: clock.ToWIB(r.CreatedAt),
	}
	if r.ActorID.Valid {
		id := uuid.UUID(r.ActorID.Bytes)
		l.ActorID = &id
	}
	if r.ActorRole.Valid {
		role := string(r.ActorRole.UserRole)
		l.ActorRole = &role
	}
	if r.ActorIdentifier.Valid {
		l.ActorIdentifier = &r.ActorIdentifier.String
	}
	if r.ActorName.Valid {
		l.ActorName = &r.ActorName.String
	}
	if r.EntityID.Valid {
		id := uuid.UUID(r.EntityID.Bytes)
		l.EntityID = &id
	}
	if r.Ip != nil {
		ip := r.Ip.String()
		l.IP = &ip
	}
	if r.UserAgent.Valid {
		l.UserAgent = &r.UserAgent.String
	}
	if r.RequestID.Valid {
		l.RequestID = &r.RequestID.String
	}
	return l
}
