// Package audit writes the append-only administrative log (brief BR-25). Record runs inside the same
// transaction as the change it describes, so a rolled-back change leaves no log line and a committed
// one always has its line.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/platform/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Redacted replaces the value of any sensitive field before it is stored.
const Redacted = "[REDACTED]"

// Entry describes one administrative action.
type Entry struct {
	Actor      *httpx.Principal // nil when the system acts
	Action     string           // a constant from actions.go
	EntityType string
	EntityID   uuid.UUID // uuid.Nil when the action has no single entity
	Before     any       // state before the change, or nil
	After      any       // state after the change, or nil
	At         time.Time // from the injected clock
}

// Record stores entry through tx. Pass the transaction that performs the change itself.
func Record(ctx context.Context, tx dbgen.DBTX, e Entry) error {
	before, err := redact(e.Before)
	if err != nil {
		return fmt.Errorf("preparing audit before-state: %w", err)
	}
	after, err := redact(e.After)
	if err != nil {
		return fmt.Errorf("preparing audit after-state: %w", err)
	}

	params := dbgen.InsertAuditLogParams{
		Action:     e.Action,
		EntityType: e.EntityType,
		Before:     before,
		After:      after,
		CreatedAt:  e.At,
	}
	if e.Actor != nil {
		params.ActorID = pgtype.UUID{Bytes: e.Actor.UserID, Valid: true}
		params.ActorRole = dbgen.NullUserRole{UserRole: dbgen.UserRole(e.Actor.Role), Valid: true}
	}
	if e.EntityID != uuid.Nil {
		params.EntityID = pgtype.UUID{Bytes: e.EntityID, Valid: true}
	}
	ci := httpx.ClientInfoFrom(ctx)
	if ip, err := netip.ParseAddr(ci.IP); err == nil {
		params.Ip = &ip
	}
	if ci.UserAgent != "" {
		params.UserAgent = pgtype.Text{String: ci.UserAgent, Valid: true}
	}
	if id := logger.RequestIDFromContext(ctx); id != "" {
		params.RequestID = pgtype.Text{String: id, Valid: true}
	}

	if err := dbgen.New(tx).InsertAuditLog(ctx, params); err != nil {
		return fmt.Errorf("writing audit log %s: %w", e.Action, err)
	}
	return nil
}

// redact turns v into JSON with every sensitive field masked. A nil v stays NULL in the database.
func redact(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshalling: %w", err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("re-reading: %w", err)
	}
	out, err := json.Marshal(mask(tree))
	if err != nil {
		return nil, fmt.Errorf("marshalling redacted: %w", err)
	}
	return out, nil
}

// sensitiveFragments are matched against a lower-cased key, so password_hash, temporary_password,
// qr_token, refresh_token and token_hash are all caught without listing each one.
var sensitiveFragments = []string{"password", "passwd", "secret", "token", "credential"}

func sensitive(key string) bool {
	k := strings.ToLower(key)
	for _, f := range sensitiveFragments {
		if strings.Contains(k, f) {
			return true
		}
	}
	return false
}

func mask(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if sensitive(k) {
				t[k] = Redacted
				continue
			}
			t[k] = mask(val)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = mask(val)
		}
		return t
	default:
		return v
	}
}
