package bookings

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"classlab/api/internal/config"
	"classlab/api/internal/httpx"
	"classlab/api/internal/overlap"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/roomtypes"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	pgUniqueViolation = "23505"

	// slotMinutes is BR-01's fixed granularity; the other limits come from configuration.
	slotMinutes = 30

	maxPurposeRunes   = 200
	maxNotesRunes     = 1000
	maxReasonRunes    = 500
	minReasonRunes    = 5 // BR-04
	maxParticipants   = 10000
	maxIdempotencyKey = 128

	conflictMessage = "Ruang sudah terpakai pada rentang waktu tersebut."
)

type Service struct {
	pool  *db.Pool
	q     *dbgen.Queries
	clock clock.Clock
	rules config.BookingRules
	types *roomtypes.Store
}

func NewService(pool *db.Pool, clk clock.Clock, rules config.BookingRules, types *roomtypes.Store) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), clock: clk, rules: rules, types: types}
}

type CreateInput struct {
	RoomID         uuid.UUID
	StartAt        time.Time
	EndAt          time.Time
	Purpose        string
	Participants   *int
	Notes          *string
	IdempotencyKey string
}

type ListFilter struct {
	Status  *string
	RoomID  *uuid.UUID
	From    *time.Time // bookings still running after this instant
	To      *time.Time // bookings starting before this instant
	Mine    bool
	Page    int
	PerPage int
}

// Create files a booking request (brief 4.1, 4.4). The room row is locked first, then the slot is
// checked against schedules and live bookings, then the row is written; the exclusion constraint
// stays behind all of that as the last guard. replayed is true when an Idempotency-Key matched an
// earlier identical request, in which case nothing new is written.
func (s *Service) Create(ctx context.Context, actor httpx.Principal, in CreateInput) (created Booking, replayed bool, err error) {
	if !isRequester(actor) {
		return Booking{}, false, httpx.ErrForbidden
	}

	in.Purpose = strings.TrimSpace(in.Purpose)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	if in.Notes != nil {
		n := strings.TrimSpace(*in.Notes)
		in.Notes = &n
		if n == "" {
			in.Notes = nil
		}
	}
	if utf8.RuneCountInString(in.IdempotencyKey) > maxIdempotencyKey {
		return Booking{}, false, httpx.ValidationError(httpx.ErrorDetail{
			Field: "Idempotency-Key", Message: fmt.Sprintf("Idempotency-Key maksimal %d karakter.", maxIdempotencyKey),
		})
	}

	// A retry must get the original answer even after the slot has moved into the past, so this
	// runs before validation. It is repeated under the room lock to catch two concurrent retries.
	if b, ok, err := s.findReplay(ctx, s.q, actor, in); err != nil || ok {
		return b, ok, err
	}

	now := s.clock.Now()
	var problems []httpx.ErrorDetail
	if in.RoomID == uuid.Nil {
		problems = append(problems, httpx.ErrorDetail{Field: "room_id", Message: "room_id wajib diisi."})
	}
	if n := utf8.RuneCountInString(in.Purpose); n < 1 || n > maxPurposeRunes {
		problems = append(problems, httpx.ErrorDetail{Field: "purpose", Message: fmt.Sprintf("Keperluan wajib diisi (maksimal %d karakter).", maxPurposeRunes)})
	}
	if in.Participants != nil && (*in.Participants < 1 || *in.Participants > maxParticipants) {
		problems = append(problems, httpx.ErrorDetail{Field: "participants", Message: fmt.Sprintf("Jumlah peserta harus antara 1 dan %d.", maxParticipants)})
	}
	if in.Notes != nil && utf8.RuneCountInString(*in.Notes) > maxNotesRunes {
		problems = append(problems, httpx.ErrorDetail{Field: "notes", Message: fmt.Sprintf("Catatan maksimal %d karakter.", maxNotesRunes)})
	}
	problems = append(problems, s.validateWindow(in.StartAt, in.EndAt, now)...)
	if len(problems) > 0 {
		return Booking{}, false, httpx.ValidationError(problems...)
	}
	start, end := in.StartAt.UTC(), in.EndAt.UTC()

	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)

		// The student lock comes first and is held to commit, so two requests of one student for different
		// rooms are counted one after the other. Locks are always taken student, then room.
		if err := q.LockRequesterQuota(ctx, actor.UserID.String()); err != nil {
			return fmt.Errorf("locking booking quota: %w", err)
		}

		room, err := q.LockRoom(ctx, in.RoomID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.ValidationError(httpx.ErrorDetail{Field: "room_id", Message: "Ruang tidak ditemukan."})
			}
			return fmt.Errorf("locking room: %w", err)
		}

		if b, ok, err := s.findReplay(ctx, q, actor, in); err != nil || ok {
			created, replayed = b, ok
			return err
		}
		// Who may request which type is policy data (BR-17, BR-20), not a role list in code.
		policy, err := s.types.Get(ctx, string(room.Type))
		if err != nil {
			return fmt.Errorf("loading room type policy: %w", err)
		}
		if !policy.CanRequest(actor.Role) {
			return httpx.ErrRoomTypeNotAllowed
		}
		active, err := q.CountActiveBookingsForRequester(ctx, dbgen.CountActiveBookingsForRequesterParams{RequesterID: actor.UserID, EndAt: now})
		if err != nil {
			return fmt.Errorf("counting active bookings: %w", err)
		}
		if active >= int64(s.rules.MaxActivePerUser) {
			return quotaExceeded(s.rules.MaxActivePerUser)
		}
		if !room.IsActive {
			return httpx.ErrRoomInactive
		}

		conflicts, err := overlap.Find(ctx, q, room.ID, []overlap.Window{{Start: start, End: end}}, uuid.Nil, uuid.Nil)
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			return overlap.Error(conflictMessage, conflicts)
		}

		params := dbgen.CreateBookingParams{
			RoomID: room.ID, RequesterID: actor.UserID, StartAt: start, EndAt: end,
			Purpose: in.Purpose, CreatedAt: now,
		}
		if in.Participants != nil {
			params.Participants = pgtype.Int4{Int32: int32(*in.Participants), Valid: true}
		}
		if in.Notes != nil {
			params.Notes = pgtype.Text{String: *in.Notes, Valid: true}
		}
		if in.IdempotencyKey != "" {
			params.IdempotencyKey = pgtype.Text{String: in.IdempotencyKey, Valid: true}
		}
		row, err := q.CreateBooking(ctx, params)
		if err != nil {
			return err
		}

		if err := q.CreateBookingEvent(ctx, dbgen.CreateBookingEventParams{
			BookingID: row.ID, ActorID: pgtype.UUID{Bytes: actor.UserID, Valid: true}, ToStatus: dbgen.BookingStatusMENUNGGU, CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("recording booking event: %w", err)
		}

		view, err := q.GetBooking(ctx, row.ID)
		if err != nil {
			return fmt.Errorf("reading created booking: %w", err)
		}
		created = fromRow(view)
		return notifyCreated(ctx, q, created, now)
	})
	if err != nil {
		return Booking{}, false, s.mapWriteError(err, "creating booking")
	}
	return created, replayed, nil
}

// findReplay returns the booking an earlier request with the same Idempotency-Key created. Reusing a
// key for a different request is a client bug, reported as a validation error rather than silently
// returning an unrelated booking.
func (s *Service) findReplay(ctx context.Context, q *dbgen.Queries, actor httpx.Principal, in CreateInput) (Booking, bool, error) {
	if in.IdempotencyKey == "" {
		return Booking{}, false, nil
	}
	row, err := q.GetBookingByIdempotencyKey(ctx, dbgen.GetBookingByIdempotencyKeyParams{
		RequesterID: actor.UserID, IdempotencyKey: pgtype.Text{String: in.IdempotencyKey, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, false, nil
	}
	if err != nil {
		return Booking{}, false, fmt.Errorf("looking up idempotency key: %w", err)
	}
	if !sameRequest(row, in) {
		return Booking{}, false, idempotencyReuse()
	}
	view, err := q.GetBooking(ctx, row.ID)
	if err != nil {
		return Booking{}, false, fmt.Errorf("reading replayed booking: %w", err)
	}
	return fromRow(view), true, nil
}

func sameRequest(row dbgen.Booking, in CreateInput) bool {
	if row.RoomID != in.RoomID || !row.StartAt.Equal(in.StartAt) || !row.EndAt.Equal(in.EndAt) || row.Purpose != in.Purpose {
		return false
	}
	if row.Participants.Valid != (in.Participants != nil) || (in.Participants != nil && int(row.Participants.Int32) != *in.Participants) {
		return false
	}
	return row.Notes.Valid == (in.Notes != nil) && (in.Notes == nil || row.Notes.String == *in.Notes)
}

func idempotencyReuse() error {
	return httpx.ValidationError(httpx.ErrorDetail{
		Field: "Idempotency-Key", Message: "Idempotency-Key sudah dipakai untuk permintaan yang berbeda.",
	})
}

// validateWindow applies BR-01. Everything is judged in WIB, the zone users read the clock in.
func (s *Service) validateWindow(start, end, now time.Time) []httpx.ErrorDetail {
	if start.IsZero() || end.IsZero() {
		var out []httpx.ErrorDetail
		if start.IsZero() {
			out = append(out, httpx.ErrorDetail{Field: "start_at", Message: "start_at wajib diisi."})
		}
		if end.IsZero() {
			out = append(out, httpx.ErrorDetail{Field: "end_at", Message: "end_at wajib diisi."})
		}
		return out
	}
	if !start.Before(end) {
		return []httpx.ErrorDetail{{Field: "end_at", Message: "end_at harus setelah start_at."}}
	}
	st, en := clock.ToWIB(start), clock.ToWIB(end)
	if st.Year() != en.Year() || st.YearDay() != en.YearDay() {
		return []httpx.ErrorDetail{{Field: "end_at", Message: "Booking harus berada pada hari yang sama (WIB)."}}
	}

	var out []httpx.ErrorDetail
	if !onSlot(st) {
		out = append(out, httpx.ErrorDetail{Field: "start_at", Message: fmt.Sprintf("start_at harus kelipatan %d menit.", slotMinutes)})
	}
	if !onSlot(en) {
		out = append(out, httpx.ErrorDetail{Field: "end_at", Message: fmt.Sprintf("end_at harus kelipatan %d menit.", slotMinutes)})
	}
	if start.Before(now) {
		out = append(out, httpx.ErrorDetail{Field: "start_at", Message: "start_at tidak boleh di masa lalu."})
	}
	if d := end.Sub(start); d < s.rules.MinDuration {
		out = append(out, httpx.ErrorDetail{Field: "end_at", Message: "Durasi minimal " + formatDuration(s.rules.MinDuration) + "."})
	} else if d > s.rules.MaxDuration {
		out = append(out, httpx.ErrorDetail{Field: "end_at", Message: "Durasi maksimal " + formatDuration(s.rules.MaxDuration) + "."})
	}
	if minuteOfDay(st) < s.rules.OpenMinute {
		out = append(out, httpx.ErrorDetail{Field: "start_at", Message: "Jam operasional dimulai pukul " + formatMinute(s.rules.OpenMinute) + " WIB."})
	}
	if minuteOfDay(en) > s.rules.CloseMinute {
		out = append(out, httpx.ErrorDetail{Field: "end_at", Message: "Jam operasional berakhir pukul " + formatMinute(s.rules.CloseMinute) + " WIB."})
	}
	today := clock.ToWIB(now)
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, clock.WIB)
	startDay := time.Date(st.Year(), st.Month(), st.Day(), 0, 0, 0, 0, clock.WIB)
	if startDay.Sub(todayStart) > time.Duration(s.rules.MaxFutureDays)*24*time.Hour {
		out = append(out, httpx.ErrorDetail{Field: "start_at", Message: fmt.Sprintf("Booking maksimal %d hari ke depan.", s.rules.MaxFutureDays)})
	}
	return out
}

func onSlot(t time.Time) bool {
	return t.Minute()%slotMinutes == 0 && t.Second() == 0 && t.Nanosecond() == 0
}

func minuteOfDay(t time.Time) int { return t.Hour()*60 + t.Minute() }

func formatMinute(m int) string { return fmt.Sprintf("%02d:%02d", m/60, m%60) }

func formatDuration(d time.Duration) string {
	if d%time.Hour == 0 {
		return fmt.Sprintf("%d jam", int(d/time.Hour))
	}
	return fmt.Sprintf("%d menit", int(d/time.Minute))
}

// / isRequester reports whether the role submits bookings: students and organisations.
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

func (s *Service) Get(ctx context.Context, actor httpx.Principal, id uuid.UUID) (Booking, error) {
	if !canRead(actor) {
		return Booking{}, httpx.ErrForbidden
	}
	row, err := s.q.GetBooking(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, httpx.ErrNotFound
	}
	if err != nil {
		return Booking{}, fmt.Errorf("reading booking: %w", err)
	}
	if err := s.visibleTo(ctx, actor, row.RequesterID, string(row.RoomType)); err != nil {
		return Booking{}, err
	}
	return fromRow(row), nil
}

// Events returns a booking's audit trail. It is not paginated: a booking has at most three events.
func (s *Service) Events(ctx context.Context, actor httpx.Principal, id uuid.UUID) ([]Event, error) {
	if !canRead(actor) {
		return nil, httpx.ErrForbidden
	}
	row, err := s.q.GetBooking(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("reading booking: %w", err)
	}
	if err := s.visibleTo(ctx, actor, row.RequesterID, string(row.RoomType)); err != nil {
		return nil, err
	}
	rows, err := s.q.ListBookingEvents(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("listing booking events: %w", err)
	}
	out := make([]Event, len(rows))
	for i, r := range rows {
		out[i] = eventFromRow(r)
	}
	return out, nil
}

func (s *Service) List(ctx context.Context, actor httpx.Principal, f ListFilter) ([]Booking, int64, error) {
	if !canRead(actor) {
		return nil, 0, httpx.ErrForbidden
	}

	// Requesters are pinned to their own bookings whatever they ask for. TU and KALAB see the room types
	// they approve; ADMIN sees all (BR-10). Managers may narrow with ?mine=.
	var requester pgtype.UUID
	var roomTypes []string
	switch {
	case isRequester(actor) || f.Mine:
		requester = pgtype.UUID{Bytes: actor.UserID, Valid: true}
	case actor.Role != httpx.RoleAdmin:
		types, err := s.types.ApprovableTypes(ctx, actor.Role)
		if err != nil {
			return nil, 0, err
		}
		// types is non-nil even when empty: an empty array matches no room type, NULL would match all of them.
		roomTypes = types
	}
	var status dbgen.NullBookingStatus
	if f.Status != nil {
		status = dbgen.NullBookingStatus{BookingStatus: dbgen.BookingStatus(*f.Status), Valid: true}
	}
	var room pgtype.UUID
	if f.RoomID != nil {
		room = pgtype.UUID{Bytes: *f.RoomID, Valid: true}
	}
	var from, to pgtype.Timestamptz
	if f.From != nil {
		from = pgtype.Timestamptz{Time: f.From.UTC(), Valid: true}
	}
	if f.To != nil {
		to = pgtype.Timestamptz{Time: f.To.UTC(), Valid: true}
	}

	rows, err := s.q.ListBookings(ctx, dbgen.ListBookingsParams{
		RequesterID: requester, Status: status, RoomID: room, RangeStart: from, RangeEnd: to, RoomTypes: roomTypes,
		PageLimit: int32(f.PerPage), PageOffset: int32((f.Page - 1) * f.PerPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing bookings: %w", err)
	}
	total, err := s.q.CountBookings(ctx, dbgen.CountBookingsParams{
		RequesterID: requester, Status: status, RoomID: room, RangeStart: from, RangeEnd: to, RoomTypes: roomTypes,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("counting bookings: %w", err)
	}

	out := make([]Booking, len(rows))
	for i, r := range rows {
		out[i] = fromRow(dbgen.GetBookingRow(r))
	}
	return out, total, nil
}

// Approve moves MENUNGGU to DISETUJUI after re-checking the slot inside the transaction (brief 4.4).
// Only an approver of the room's type may do it (BR-10).
func (s *Service) Approve(ctx context.Context, actor httpx.Principal, id uuid.UUID) (Booking, error) {
	if !isManager(actor) {
		return Booking{}, httpx.ErrForbidden
	}
	return s.decide(ctx, actor, id, decision{
		to:        dbgen.BookingStatusDISETUJUI,
		recheck:   true,
		asManager: true,
		check: func(cur dbgen.Booking, room dbgen.LockRoomRow, now time.Time) error {
			if cur.Status != dbgen.BookingStatusMENUNGGU {
				return invalidState(fmt.Sprintf("Booking berstatus %s tidak dapat disetujui.", cur.Status))
			}
			if !room.IsActive {
				return httpx.ErrRoomInactive
			}
			if !cur.EndAt.After(now) {
				return invalidState("Waktu booking sudah lewat sehingga tidak dapat disetujui.")
			}
			return nil
		},
	})
}

// Reject moves MENUNGGU to DITOLAK; the reason is mandatory (BR-04).
func (s *Service) Reject(ctx context.Context, actor httpx.Principal, id uuid.UUID, reason string) (Booking, error) {
	if !isManager(actor) {
		return Booking{}, httpx.ErrForbidden
	}
	reason, err := requireReason(reason)
	if err != nil {
		return Booking{}, err
	}
	return s.decide(ctx, actor, id, decision{
		to:        dbgen.BookingStatusDITOLAK,
		reason:    reason,
		asManager: true,
		check: func(cur dbgen.Booking, _ dbgen.LockRoomRow, _ time.Time) error {
			if cur.Status != dbgen.BookingStatusMENUNGGU {
				return invalidState(fmt.Sprintf("Booking berstatus %s tidak dapat ditolak.", cur.Status))
			}
			return nil
		},
	})
}

// Cancel is allowed to the owner while the booking is MENUNGGU, and to an approver of the room's type
// once it is DISETUJUI (with a reason, BR-04). Any other combination is an invalid transition (brief 4.1).
func (s *Service) Cancel(ctx context.Context, actor httpx.Principal, id uuid.UUID, reason string) (Booking, error) {
	var err error
	switch {
	case isManager(actor):
		reason, err = requireReason(reason)
	case isRequester(actor):
		reason, err = optionalReason(reason)
	default:
		return Booking{}, httpx.ErrForbidden
	}
	if err != nil {
		return Booking{}, err
	}

	return s.decide(ctx, actor, id, decision{
		to:        dbgen.BookingStatusDIBATALKAN,
		reason:    reason,
		asManager: isManager(actor),
		check: func(cur dbgen.Booking, _ dbgen.LockRoomRow, _ time.Time) error {
			switch {
			case isRequester(actor) && cur.Status != dbgen.BookingStatusMENUNGGU:
				return invalidState("Pengajuan yang sudah diputuskan hanya dapat dibatalkan oleh pengelola.")
			case isManager(actor) && cur.Status != dbgen.BookingStatusDISETUJUI:
				return invalidState("Pengelola hanya dapat membatalkan booking yang sudah disetujui; tolak pengajuan yang masih menunggu.")
			}
			return nil
		},
	})
}

type decision struct {
	to        dbgen.BookingStatus
	reason    string // stored as decision_reason when not empty
	recheck   bool   // re-run the overlap check, for approvals
	asManager bool   // the actor must approve this booking's room type
	check     func(cur dbgen.Booking, room dbgen.LockRoomRow, now time.Time) error
}

// decide is the one place a booking changes status: lock the room, re-read the booking under the
// lock, validate the transition, then write the new status, its audit event and the notification in
// the same transaction, so none of them can exist without the others.
//
// Every writer of booking status takes the room lock first, so the re-read cannot go stale between
// the check and the update and no separate row lock is needed.
func (s *Service) decide(ctx context.Context, actor httpx.Principal, id uuid.UUID, d decision) (Booking, error) {
	// An unlocked peek finds the room to lock and lets a requester who does not own the booking get a
	// 404 without ever contending for the lock. A manager's authority is checked under the lock, against
	// the room's current type.
	peek, err := s.q.GetBooking(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, httpx.ErrNotFound
	}
	if err != nil {
		return Booking{}, fmt.Errorf("reading booking: %w", err)
	}
	if isRequester(actor) && peek.RequesterID != actor.UserID {
		return Booking{}, httpx.ErrNotFound
	}

	var result Booking
	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		room, err := q.LockRoom(ctx, peek.RoomID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.ErrNotFound
			}
			return fmt.Errorf("locking room: %w", err)
		}
		if d.asManager {
			if err := s.types.RequireApprover(ctx, actor, string(room.Type)); err != nil {
				return err
			}
		}
		cur, err := q.LoadBooking(ctx, id)
		if err != nil {
			return fmt.Errorf("re-reading booking: %w", err)
		}

		if cur.Status == dbgen.BookingStatusDITOLAK || cur.Status == dbgen.BookingStatusDIBATALKAN {
			return invalidState(fmt.Sprintf("Booking berstatus %s sudah final.", cur.Status))
		}
		if err := d.check(cur, room, now); err != nil {
			return err
		}
		if d.recheck {
			conflicts, err := overlap.Find(ctx, q, room.ID, []overlap.Window{{Start: cur.StartAt, End: cur.EndAt}}, uuid.Nil, cur.ID)
			if err != nil {
				return err
			}
			if len(conflicts) > 0 {
				return overlap.Error(conflictMessage, conflicts)
			}
		}

		reason := pgtype.Text{String: d.reason, Valid: d.reason != ""}
		if _, err := q.DecideBooking(ctx, dbgen.DecideBookingParams{
			ID: id, Status: d.to, DecisionReason: reason,
			DecidedBy: pgtype.UUID{Bytes: actor.UserID, Valid: true},
			Now:       pgtype.Timestamptz{Time: now, Valid: true},
		}); err != nil {
			return fmt.Errorf("updating booking status: %w", err)
		}
		if err := q.CreateBookingEvent(ctx, dbgen.CreateBookingEventParams{
			BookingID: id, ActorID: pgtype.UUID{Bytes: actor.UserID, Valid: true}, ToStatus: d.to, CreatedAt: now, Reason: reason,
			FromStatus: dbgen.NullBookingStatus{BookingStatus: cur.Status, Valid: true},
		}); err != nil {
			return fmt.Errorf("recording booking event: %w", err)
		}

		view, err := q.GetBooking(ctx, id)
		if err != nil {
			return fmt.Errorf("reading updated booking: %w", err)
		}
		result = fromRow(view)
		return notifyDecided(ctx, q, result, actor, d.reason, now)
	})
	if err != nil {
		return Booking{}, s.mapWriteError(err, "updating booking")
	}
	return result, nil
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

// quotaExceeded is a 422 with no field: the request is well-formed, the student just holds too many.
func quotaExceeded(limit int) error {
	return httpx.ValidationError(httpx.ErrorDetail{
		Message: fmt.Sprintf("Anda sudah memiliki %d pengajuan aktif (menunggu atau disetujui). Batalkan salah satunya atau tunggu hingga selesai sebelum mengajukan lagi.", limit),
	})
}
