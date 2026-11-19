package rooms

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"time"

	"classlab/api/internal/audit"
	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// QR is what a manager prints and sticks on the door (BR-11).
type QR struct {
	URL       string    `json:"url"`
	QRToken   string    `json:"qr_token"`
	RotatedAt time.Time `json:"rotated_at"`
}

// qrTokenBytes gives 256 bits, written as 64 hex characters like the tokens the migration created.
const qrTokenBytes = 32

func newQRToken() (string, error) {
	b := make([]byte, qrTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating qr token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// checkinURL is the address the QR encodes: <frontend>/check-in?room=<code>&t=<token>.
func (s *Service) checkinURL(code, token string) string {
	q := url.Values{"room": {code}, "t": {token}}
	return s.publicBaseURL + "/check-in?" + q.Encode()
}

// QR returns the printable QR of a room. Only those who manage the room's type may see the token.
func (s *Service) QR(ctx context.Context, actor httpx.Principal, id uuid.UUID) (QR, error) {
	row, err := s.q.GetRoomQR(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return QR{}, httpx.ErrNotFound
	}
	if err != nil {
		return QR{}, fmt.Errorf("reading room qr: %w", err)
	}
	if err := s.types.RequireRoomManager(ctx, actor, string(row.Type)); err != nil {
		return QR{}, err
	}
	return QR{URL: s.checkinURL(row.Code, row.QrToken), QRToken: row.QrToken, RotatedAt: clock.ToWIB(row.QrRotatedAt)}, nil
}

// RotateQR replaces the token; the old one stops working at once because check-in compares against the
// stored value. The audit entry records that it happened, never a token (BR-25).
func (s *Service) RotateQR(ctx context.Context, actor httpx.Principal, id uuid.UUID) (QR, error) {
	token, err := newQRToken()
	if err != nil {
		return QR{}, err
	}

	var out QR
	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		// Same lock as check-in reads under, so a rotation cannot slip between a token check and its commit.
		locked, err := q.LockRoomQR(ctx, id)
		if err != nil {
			return err
		}
		if err := s.types.RequireRoomManager(ctx, actor, string(locked.Type)); err != nil {
			return err
		}
		row, err := q.RotateRoomQR(ctx, dbgen.RotateRoomQRParams{ID: id, QrToken: token, Now: now})
		if err != nil {
			return fmt.Errorf("rotating room qr: %w", err)
		}
		out = QR{URL: s.checkinURL(row.Code, row.QrToken), QRToken: row.QrToken, RotatedAt: clock.ToWIB(row.QrRotatedAt)}
		return audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionRoomQRRotated, EntityType: audit.EntityRoom, EntityID: id,
			After: map[string]any{"code": row.Code, "qr_rotated_at": row.QrRotatedAt}, At: now,
		})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return QR{}, httpx.ErrNotFound
	}
	if err != nil {
		if errors.Is(err, httpx.ErrForbidden) {
			return QR{}, err
		}
		return QR{}, fmt.Errorf("rotating qr: %w", err)
	}
	return out, nil
}
