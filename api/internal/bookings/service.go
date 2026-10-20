// Package bookings runs the booking workflow: request (one date or several), automatic or manual approval,
// edit, cancel, and the append-only history of each booking.
package bookings

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"classlab/api/internal/attachments"
	"classlab/api/internal/httpx"
	"classlab/api/internal/notifications"
	"classlab/api/internal/overlap"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/reports"
	"classlab/api/internal/roomtypes"
	"classlab/api/internal/violations"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	pgUniqueViolation = "23505"

	// slotMinutes is BR-01's fixed granularity; the other limits come from the room type's policy.
	slotMinutes = 30

	maxPurposeRunes      = 200
	maxNotesRunes        = 1000
	maxReasonRunes       = 500
	minReasonRunes       = 5 // BR-04
	maxParticipants      = 10000
	maxIdempotencyKey    = 128
	maxLetterNumberRunes = 100

	conflictMessage = "Ruang sudah terpakai pada rentang waktu tersebut."

	// autoApprovedReason is the reason written on the system's approval event (BR-17).
	autoApprovedReason = "AUTO_APPROVED"
)

// Settings are the BR-11 to BR-28 values the booking workflow reads from configuration.
type Settings struct {
	PendingTTL           time.Duration // BR-22
	EditCutoff           time.Duration // BR-26
	LateCancel           time.Duration // BR-14
	MaxOccurrences       int           // BR-27
	AutoApproveMaxPoints int           // BR-17
	MaxActiveBookings    int           // BR-15, MAHASISWA
	MaxActiveBookingsOrg int           // BR-15, ORGANISASI
	PointsLateCancel     int           // BR-14
	CheckinEarly         time.Duration // BR-11
	CheckinLate          time.Duration // BR-11
	Violations           violations.Settings
}

type Service struct {
	pool     *db.Pool
	q        *dbgen.Queries
	clock    clock.Clock
	types    *roomtypes.Store
	notifier *notifications.Notifier
	files    *attachments.Service
	reports  *reports.Service
	settings Settings
}

func NewService(pool *db.Pool, clk clock.Clock, types *roomtypes.Store, notifier *notifications.Notifier, files *attachments.Service, reports *reports.Service, settings Settings) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), clock: clk, types: types, notifier: notifier, files: files, reports: reports, settings: settings}
}

// isRequester reports whether the role submits bookings: students and organisations.
func isRequester(actor httpx.Principal) bool {
	return actor.Role == httpx.RoleMahasiswa || actor.Role == httpx.RoleOrganisasi
}

// isManager reports whether the role can decide bookings at all. Which room types it may decide is
// checked against each room type's policy (BR-10), never against the role alone.
func isManager(actor httpx.Principal) bool {
	return actor.HasRole(httpx.RoleTU, httpx.RoleKalab, httpx.RoleAdmin)
}

// canRead is the set of roles allowed to read bookings at all; OB is not among them.
func canRead(actor httpx.Principal) bool {
	return isRequester(actor) || isManager(actor)
}

// visibleTo reports whether the actor may read a booking. A requester sees their own, TU and KALAB see
// the room types they approve, and ADMIN reads everything. Anything else answers 404, so a booking
// outside the actor's reach is indistinguishable from a missing one (BR-10).
func (s *Service) visibleTo(ctx context.Context, actor httpx.Principal, requesterID uuid.UUID, roomType string) error {
	switch {
	case actor.Role == httpx.RoleAdmin:
		return nil
	case isRequester(actor):
		if actor.UserID == requesterID {
			return nil
		}
		return httpx.ErrNotFound
	case isManager(actor):
		err := s.types.RequireApprover(ctx, actor, roomType)
		if errors.Is(err, httpx.ErrForbidden) {
			return httpx.ErrNotFound
		}
		return err
	default:
		return httpx.ErrForbidden
	}
}

// maxActive is the quota for a requester's role (BR-15).
func (s *Service) maxActive(actor httpx.Principal) int {
	if actor.Role == httpx.RoleOrganisasi {
		return s.settings.MaxActiveBookingsOrg
	}
	return s.settings.MaxActiveBookings
}

func (s *Service) mapWriteError(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		// The only unique constraint besides the primary key is (requester_id, idempotency_key): two
		// requests with the same key raced on different rooms, so they did not share a lock.
		return idempotencyReuse()
	}
	return overlap.MapWriteError(err, op, conflictMessage)
}

func invalidState(message string) error {
	return httpx.NewCustomError(http.StatusConflict, "INVALID_STATE", message, nil, httpx.ErrInvalidState)
}

func idempotencyReuse() error {
	return httpx.ValidationError(httpx.ErrorDetail{
		Field: "Idempotency-Key", Message: "Idempotency-Key sudah dipakai untuk permintaan yang berbeda.",
	})
}

// quotaExceeded is the 409 QUOTA_EXCEEDED of BR-15: the request is well-formed, the requester just holds
// too many live requests (a multi-date request counts as one).
func quotaExceeded(limit int) error {
	return httpx.NewCustomError(http.StatusConflict, "QUOTA_EXCEEDED",
		fmt.Sprintf("Anda sudah memiliki %d pengajuan aktif (menunggu, disetujui, atau berlangsung). Batalkan salah satunya atau tunggu hingga selesai sebelum mengajukan lagi.", limit),
		nil, httpx.ErrInvalidState)
}

// bookingBlocked is the 403 BOOKING_BLOCKED of BR-14; details carry when the block ends.
func bookingBlocked(until time.Time) error {
	return httpx.NewCustomError(http.StatusForbidden, "BOOKING_BLOCKED",
		"Anda sedang diblokir mengajukan booking karena poin pelanggaran. Booking yang sudah disetujui tetap berlaku.",
		[]httpx.ErrorDetail{{Field: "blocked_until", Message: clock.ToWIB(until).Format(time.RFC3339)}}, httpx.ErrForbidden)
}

// editWindowClosed is the 409 EDIT_WINDOW_CLOSED of BR-26.
func editWindowClosed(cutoff time.Duration) error {
	return httpx.NewCustomError(http.StatusConflict, "EDIT_WINDOW_CLOSED",
		fmt.Sprintf("Pengajuan hanya dapat diubah sampai %d menit sebelum mulai.", int(cutoff/time.Minute)),
		nil, httpx.ErrInvalidState)
}

func requireReason(v string) (string, error) {
	v = strings.TrimSpace(v)
	n := utf8.RuneCountInString(v)
	if n < minReasonRunes || n > maxReasonRunes {
		return "", httpx.ValidationError(httpx.ErrorDetail{
			Field: "reason", Message: fmt.Sprintf("Alasan wajib diisi (%d-%d karakter).", minReasonRunes, maxReasonRunes),
		})
	}
	return v, nil
}

func optionalReason(v string) (string, error) {
	v = strings.TrimSpace(v)
	if utf8.RuneCountInString(v) > maxReasonRunes {
		return "", httpx.ValidationError(httpx.ErrorDetail{Field: "reason", Message: fmt.Sprintf("Alasan maksimal %d karakter.", maxReasonRunes)})
	}
	return v, nil
}
