package notifications

import (
	"context"
	"fmt"
	"time"

	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Message is what a notification says, independent of how it reaches anyone.
type Message struct {
	Type    string // one of the Type constants
	Title   string
	Body    string
	RefType string // one of the Ref constants
	RefID   uuid.UUID
}

// Medium delivers messages. Business code never talks to a medium directly, only to the Notifier, so
// adding Web Push (Phase 8) means adding a Medium and listing it in app wiring, with no change to the
// code that decides who is told what.
//
// Both methods run inside the caller's transaction (q is bound to it). The in-app medium writes rows
// there, so a notification commits or rolls back with the change that caused it (agent.md section 7, #7).
// A medium that reaches outside the database must not send from here, because the transaction can still
// roll back; it should record the message for sending after commit.
type Medium interface {
	// ToUser delivers m to one user.
	ToUser(ctx context.Context, q *dbgen.Queries, userID uuid.UUID, m Message, at time.Time) error
	// ToApprovers delivers m to every active user whose role approves roomType (BR-10), except exclude
	// when it is not uuid.Nil (people are not told about what they did themselves).
	ToApprovers(ctx context.Context, q *dbgen.Queries, roomType string, m Message, exclude uuid.UUID, at time.Time) error
}

// Notifier hands each message to every configured medium. The first failing medium aborts the call,
// which fails the caller's transaction: a half-delivered notification is worse than a retried request.
type Notifier struct {
	media []Medium
}

func NewNotifier(media ...Medium) *Notifier {
	return &Notifier{media: media}
}

func (n *Notifier) ToUser(ctx context.Context, q *dbgen.Queries, userID uuid.UUID, m Message, at time.Time) error {
	for _, c := range n.media {
		if err := c.ToUser(ctx, q, userID, m, at); err != nil {
			return fmt.Errorf("notifying user: %w", err)
		}
	}
	return nil
}

func (n *Notifier) ToApprovers(ctx context.Context, q *dbgen.Queries, roomType string, m Message, exclude uuid.UUID, at time.Time) error {
	for _, c := range n.media {
		if err := c.ToApprovers(ctx, q, roomType, m, exclude, at); err != nil {
			return fmt.Errorf("notifying approvers: %w", err)
		}
	}
	return nil
}

// InApp stores notifications in the notifications table. The live stream hears about each row when the
// transaction commits (see db/queries/notifications.sql).
type InApp struct{}

func (InApp) ToUser(ctx context.Context, q *dbgen.Queries, userID uuid.UUID, m Message, at time.Time) error {
	return q.CreateNotification(ctx, dbgen.CreateNotificationParams{
		UserID: userID, Type: m.Type, Title: m.Title, Body: m.Body,
		RefType:   pgtype.Text{String: m.RefType, Valid: m.RefType != ""},
		RefID:     pgtype.UUID{Bytes: m.RefID, Valid: m.RefID != uuid.Nil},
		CreatedAt: at,
	})
}

func (InApp) ToApprovers(ctx context.Context, q *dbgen.Queries, roomType string, m Message, exclude uuid.UUID, at time.Time) error {
	return q.CreateNotificationForApprovers(ctx, dbgen.CreateNotificationForApproversParams{
		RoomType: dbgen.RoomType(roomType), Type: m.Type, Title: m.Title, Body: m.Body,
		RefType:       pgtype.Text{String: m.RefType, Valid: m.RefType != ""},
		RefID:         pgtype.UUID{Bytes: m.RefID, Valid: m.RefID != uuid.Nil},
		CreatedAt:     at,
		ExcludeUserID: pgtype.UUID{Bytes: exclude, Valid: exclude != uuid.Nil},
	})
}
