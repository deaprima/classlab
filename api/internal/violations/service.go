package violations

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"classlab/api/internal/audit"
	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	minReasonRunes = 5 // BR-04, as for other decisions that need a reason
	maxReasonRunes = 500
)

// Violation is one entry of a requester's record (BR-14).
type Violation struct {
	ID        uuid.UUID `json:"id"`
	BookingID uuid.UUID `json:"booking_id"`
	Kind      string    `json:"kind"`
	Points    int       `json:"points"`
	// Active means the points still count: not annulled and inside the look-back window.
	Active         bool       `json:"active"`
	AnnulledAt     *time.Time `json:"annulled_at"`
	AnnulledBy     *uuid.UUID `json:"annulled_by"`
	AnnulReason    *string    `json:"annul_reason"`
	CreatedAt      time.Time  `json:"created_at"`
	RoomCode       *string    `json:"room_code,omitempty"`
	BookingStartAt *time.Time `json:"booking_start_at,omitempty"`
}

// History is a requester's violations with the standing that follows from them.
type History struct {
	Data                []Violation
	Total               int64
	ActivePoints        int
	BookingBlockedUntil *time.Time
}

type Service struct {
	pool   *db.Pool
	q      *dbgen.Queries
	clock  clock.Clock
	window time.Duration
}

func NewService(pool *db.Pool, clk clock.Clock, window time.Duration) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), clock: clk, window: window}
}

func fromRow(v dbgen.Violation, now time.Time, window time.Duration) Violation {
	out := Violation{
		ID: v.ID, BookingID: v.BookingID, Kind: string(v.Kind), Points: int(v.Points),
		Active:    !v.AnnulledAt.Valid && v.CreatedAt.After(now.Add(-window)),
		CreatedAt: clock.ToWIB(v.CreatedAt),
	}
	if v.AnnulledAt.Valid {
		t := clock.ToWIB(v.AnnulledAt.Time)
		out.AnnulledAt = &t
	}
	if v.AnnulledBy.Valid {
		id := uuid.UUID(v.AnnulledBy.Bytes)
		out.AnnulledBy = &id
	}
	if v.AnnulReason.Valid {
		out.AnnulReason = &v.AnnulReason.String
	}
	return out
}

// List returns the violations of one requester. TU, KALAB and ADMIN may read them (brief 6.2); an account
// that is not a requester has none, so it answers 404 like a missing user.
func (s *Service) List(ctx context.Context, actor httpx.Principal, userID uuid.UUID, page, perPage int) (History, error) {
	if !actor.HasRole(httpx.RoleTU, httpx.RoleKalab, httpx.RoleAdmin) {
		return History{}, httpx.ErrForbidden
	}
	user, err := s.q.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && user.Role != dbgen.UserRoleMAHASISWA && user.Role != dbgen.UserRoleORGANISASI) {
		return History{}, httpx.ErrNotFound
	}
	if err != nil {
		return History{}, fmt.Errorf("reading user: %w", err)
	}

	now := s.clock.Now()
	rows, err := s.q.ListUserViolations(ctx, dbgen.ListUserViolationsParams{
		UserID: userID, PageLimit: int32(perPage), PageOffset: int32((page - 1) * perPage),
	})
	if err != nil {
		return History{}, fmt.Errorf("listing violations: %w", err)
	}
	total, err := s.q.CountUserViolations(ctx, userID)
	if err != nil {
		return History{}, fmt.Errorf("counting violations: %w", err)
	}
	points, err := ActivePoints(ctx, s.q, Settings{Window: s.window}, userID, now)
	if err != nil {
		return History{}, err
	}

	h := History{Data: make([]Violation, len(rows)), Total: total, ActivePoints: points}
	for i, r := range rows {
		h.Data[i] = fromRow(dbgen.Violation{
			ID: r.ID, UserID: r.UserID, BookingID: r.BookingID, Kind: r.Kind, Points: r.Points,
			AnnulledAt: r.AnnulledAt, AnnulledBy: r.AnnulledBy, AnnulReason: r.AnnulReason, CreatedAt: r.CreatedAt,
		}, now, s.window)
		code, start := r.RoomCode, clock.ToWIB(r.BookingStartAt)
		h.Data[i].RoomCode, h.Data[i].BookingStartAt = &code, &start
	}
	if user.BookingBlockedUntil.Valid && user.BookingBlockedUntil.Time.After(now) {
		t := clock.ToWIB(user.BookingBlockedUntil.Time)
		h.BookingBlockedUntil = &t
	}
	return h, nil
}

// Annul cancels the points of one violation (ADMIN, with a reason). It does not lift an existing block;
// that is done on the user (PATCH /users/{id}, clear_booking_block). The row is locked so two annulments of
// the same violation cannot both succeed.
func (s *Service) Annul(ctx context.Context, actor httpx.Principal, id uuid.UUID, reason string) (Violation, error) {
	if actor.Role != httpx.RoleAdmin {
		return Violation{}, httpx.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(reason); n < minReasonRunes || n > maxReasonRunes {
		return Violation{}, httpx.ValidationError(httpx.ErrorDetail{
			Field: "reason", Message: fmt.Sprintf("Alasan wajib diisi (%d-%d karakter).", minReasonRunes, maxReasonRunes),
		})
	}

	var out Violation
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		cur, err := q.LockViolation(ctx, id)
		if err != nil {
			return err
		}
		if cur.AnnulledAt.Valid {
			return httpx.NewCustomError(http.StatusConflict, "INVALID_STATE", "Pelanggaran ini sudah dibatalkan.", nil, httpx.ErrInvalidState)
		}
		row, err := q.AnnulViolation(ctx, dbgen.AnnulViolationParams{
			ID: id, Now: pgtype.Timestamptz{Time: now, Valid: true},
			AnnulledBy: pgtype.UUID{Bytes: actor.UserID, Valid: true}, Reason: pgtype.Text{String: reason, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("annulling violation: %w", err)
		}
		out = fromRow(row, now, s.window)
		return audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionViolationAnnulled, EntityType: audit.EntityViolation, EntityID: id,
			Before: map[string]any{"user_id": cur.UserID, "kind": cur.Kind, "points": cur.Points, "annulled": false},
			After:  map[string]any{"user_id": cur.UserID, "kind": cur.Kind, "points": cur.Points, "annulled": true, "reason": reason},
			At:     now,
		})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Violation{}, httpx.ErrNotFound
	}
	if err != nil {
		var domain *httpx.CustomDomainError
		if errors.As(err, &domain) {
			return Violation{}, err
		}
		return Violation{}, fmt.Errorf("annulling violation: %w", err)
	}
	return out, nil
}
