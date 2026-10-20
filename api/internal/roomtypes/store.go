package roomtypes

import (
	"context"
	"errors"
	"fmt"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/jackc/pgx/v5"
)

// ErrUnknownType means no policy row exists for the requested room type.
var ErrUnknownType = errors.New("unknown room type")

// Store reads policies on every call. The table has five rows and is read by primary key, and reading
// fresh means an administrator's change applies to the very next request without a cache to invalidate.
type Store struct {
	q *dbgen.Queries
}

// NewStore takes the pool or a transaction, whichever the caller wants the reads to run in.
func NewStore(db dbgen.DBTX) *Store {
	return &Store{q: dbgen.New(db)}
}

func (s *Store) Get(ctx context.Context, roomType string) (Policy, error) {
	// A value outside the enum would make Postgres raise an error instead of returning no row.
	if !dbgen.RoomType(roomType).Valid() {
		return Policy{}, ErrUnknownType
	}
	row, err := s.q.GetRoomTypePolicy(ctx, dbgen.RoomType(roomType))
	if errors.Is(err, pgx.ErrNoRows) {
		return Policy{}, ErrUnknownType
	}
	if err != nil {
		return Policy{}, fmt.Errorf("getting room type policy: %w", err)
	}
	return fromRow(policyRow(row)), nil
}

func (s *Store) All(ctx context.Context) ([]Policy, error) {
	rows, err := s.q.ListRoomTypePolicies(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing room type policies: %w", err)
	}
	out := make([]Policy, len(rows))
	for i, r := range rows {
		out[i] = fromRow(policyRow(r))
	}
	return out, nil
}

// RequireApprover returns ErrForbidden unless the actor's role approves this room type.
func (s *Store) RequireApprover(ctx context.Context, actor httpx.Principal, roomType string) error {
	return s.require(ctx, roomType, func(p Policy) bool { return p.CanApprove(actor.Role) })
}

// RequireRoomManager returns ErrForbidden unless the actor may manage rooms and schedules of this type.
func (s *Store) RequireRoomManager(ctx context.Context, actor httpx.Principal, roomType string) error {
	return s.require(ctx, roomType, func(p Policy) bool { return p.CanManageRooms(actor.Role) })
}

func (s *Store) require(ctx context.Context, roomType string, allowed func(Policy) bool) error {
	p, err := s.Get(ctx, roomType)
	if errors.Is(err, ErrUnknownType) {
		// A type with no policy cannot be authorised for anyone.
		return httpx.ErrForbidden
	}
	if err != nil {
		return err
	}
	if !allowed(p) {
		return httpx.ErrForbidden
	}
	return nil
}

// ApprovableTypes lists the room types role approves, for scoping lists to what a manager may see.
func (s *Store) ApprovableTypes(ctx context.Context, role httpx.Role) ([]string, error) {
	return s.typesWhere(ctx, func(p Policy) bool { return p.CanApprove(role) })
}

// ManageableTypes lists the room types role may manage rooms and schedules for.
func (s *Store) ManageableTypes(ctx context.Context, role httpx.Role) ([]string, error) {
	return s.typesWhere(ctx, func(p Policy) bool { return p.CanManageRooms(role) })
}

func (s *Store) typesWhere(ctx context.Context, keep func(Policy) bool) ([]string, error) {
	all, err := s.All(ctx)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, p := range all {
		if keep(p) {
			out = append(out, p.Type)
		}
	}
	return out, nil
}
