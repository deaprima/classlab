package notifications

import (
	"context"
	"errors"
	"fmt"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Every method acts on the caller's own notifications only; there is no way to name another user's.
type Service struct {
	q     *dbgen.Queries
	clock clock.Clock
}

func NewService(pool *db.Pool, clk clock.Clock) *Service {
	return &Service{q: dbgen.New(pool), clock: clk}
}

func (s *Service) List(ctx context.Context, actor httpx.Principal, unreadOnly bool, page, perPage int) ([]Notification, int64, error) {
	rows, err := s.q.ListNotifications(ctx, dbgen.ListNotificationsParams{
		UserID: actor.UserID, UnreadOnly: unreadOnly,
		PageLimit: int32(perPage), PageOffset: int32((page - 1) * perPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing notifications: %w", err)
	}
	total, err := s.q.CountNotifications(ctx, dbgen.CountNotificationsParams{UserID: actor.UserID, UnreadOnly: unreadOnly})
	if err != nil {
		return nil, 0, fmt.Errorf("counting notifications: %w", err)
	}
	out := make([]Notification, len(rows))
	for i, r := range rows {
		out[i] = fromRow(r)
	}
	return out, total, nil
}

// MarkRead is idempotent: marking a read notification again keeps its first read_at.
// Someone else's notification is reported as missing.
func (s *Service) MarkRead(ctx context.Context, actor httpx.Principal, id uuid.UUID) (Notification, error) {
	row, err := s.q.MarkNotificationRead(ctx, dbgen.MarkNotificationReadParams{
		ID: id, UserID: actor.UserID, ReadAt: pgtype.Timestamptz{Time: s.clock.Now(), Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Notification{}, httpx.ErrNotFound
	}
	if err != nil {
		return Notification{}, fmt.Errorf("marking notification read: %w", err)
	}
	return fromRow(row), nil
}

// MarkAllRead returns how many notifications changed.
func (s *Service) MarkAllRead(ctx context.Context, actor httpx.Principal) (int64, error) {
	n, err := s.q.MarkAllNotificationsRead(ctx, dbgen.MarkAllNotificationsReadParams{
		UserID: actor.UserID, ReadAt: pgtype.Timestamptz{Time: s.clock.Now(), Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("marking all notifications read: %w", err)
	}
	return n, nil
}

// Get loads one notification of the user, for the stream to push.
func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Notification, error) {
	row, err := s.q.GetNotificationForUser(ctx, dbgen.GetNotificationForUserParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Notification{}, httpx.ErrNotFound
	}
	if err != nil {
		return Notification{}, fmt.Errorf("reading notification: %w", err)
	}
	return fromRow(row), nil
}

func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	n, err := s.q.CountNotifications(ctx, dbgen.CountNotificationsParams{UserID: userID, UnreadOnly: true})
	if err != nil {
		return 0, fmt.Errorf("counting unread notifications: %w", err)
	}
	return n, nil
}
