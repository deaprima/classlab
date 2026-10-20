package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"classlab/api/internal/httpx"
	"classlab/api/internal/overlap"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/roomtypes"
	"classlab/api/internal/violations"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Occurrence is one date of a multi-date request.
type Occurrence struct {
	StartAt time.Time
	EndAt   time.Time
}

type CreateInput struct {
	RoomID uuid.UUID
	// StartAt and EndAt make a single booking; Occurrences make a group of 1 to MaxOccurrences (BR-27).
	// HasOccurrences tells "occurrences was sent" from "was not", whatever its length.
	StartAt            time.Time
	EndAt              time.Time
	Occurrences        []Occurrence
	HasOccurrences     bool
	Purpose            string
	Participants       *int
	Notes              *string
	Details            json.RawMessage
	LetterAttachmentID *uuid.UUID
	LetterNumber       *string
	IdempotencyKey     string
}

// CreateResult holds a Booking for a single date and a GroupResult for a multi-date request.
// Replayed is true when an Idempotency-Key matched an earlier identical request and nothing new was written.
type CreateResult struct {
	Booking  *Booking
	Group    *GroupResult
	Replayed bool
}

// window is one date of a request, numbered from 1, in UTC.
type window struct {
	no         int
	start, end time.Time
}

func (in CreateInput) windows() []window {
	if !in.HasOccurrences {
		return []window{{no: 1, start: in.StartAt.UTC(), end: in.EndAt.UTC()}}
	}
	out := make([]window, len(in.Occurrences))
	for i, o := range in.Occurrences {
		out[i] = window{no: i + 1, start: o.StartAt.UTC(), end: o.EndAt.UTC()}
	}
	return out
}

// occurrenceTag is the number put on problems: only a multi-date request has dates to tell apart.
func (in CreateInput) occurrenceTag(w window) int {
	if in.HasOccurrences {
		return w.no
	}
	return 0
}

// Create files a request (brief 4.1, 4.4). Everything happens in one transaction: the requester's quota
// lock, then the room row lock, then the checks (type policy, block, letter, capacity, quota, collisions)
// and finally the rows, their history and the notification. A multi-date request is all or nothing: one
// date that fails stops the whole request. The exclusion constraint stays behind all of it as the last guard.
func (s *Service) Create(ctx context.Context, actor httpx.Principal, in CreateInput) (CreateResult, error) {
	if !isRequester(actor) {
		return CreateResult{}, httpx.ErrForbidden
	}

	in.Purpose = strings.TrimSpace(in.Purpose)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	in.Notes = trimmedOrNil(in.Notes)
	in.LetterNumber = trimmedOrNil(in.LetterNumber)

	var problems []httpx.ErrorDetail
	if in.HasOccurrences && (!in.StartAt.IsZero() || !in.EndAt.IsZero()) {
		problems = append(problems, httpx.ErrorDetail{Field: "occurrences", Message: "Gunakan start_at dan end_at, atau occurrences, tidak keduanya."})
	}
	if in.HasOccurrences && (len(in.Occurrences) < 1 || len(in.Occurrences) > s.settings.MaxOccurrences) {
		problems = append(problems, httpx.ErrorDetail{Field: "occurrences", Message: fmt.Sprintf("occurrences harus berisi 1 sampai %d tanggal.", s.settings.MaxOccurrences)})
	}
	if utf8.RuneCountInString(in.IdempotencyKey) > maxIdempotencyKey {
		problems = append(problems, httpx.ErrorDetail{Field: "Idempotency-Key", Message: fmt.Sprintf("Idempotency-Key maksimal %d karakter.", maxIdempotencyKey)})
	}
	if len(problems) > 0 {
		return CreateResult{}, httpx.ValidationError(problems...)
	}
	wins := in.windows()

	// A retry must get the original answer even after the slot has moved into the past, so this runs before
	// validation. It is repeated under the room lock to catch two concurrent retries.
	if res, ok, err := s.findReplay(ctx, s.q, actor, in, wins); err != nil || ok {
		return res, err
	}

	now := s.clock.Now()
	if in.RoomID == uuid.Nil {
		problems = append(problems, httpx.ErrorDetail{Field: "room_id", Message: "room_id wajib diisi."})
	}
	problems = append(problems, validateCommon(in.Purpose, in.Participants, in.Notes, in.LetterNumber)...)
	for _, w := range wins {
		problems = append(problems, tagList(validateWindow(w.start, w.end, now), in.occurrenceTag(w))...)
	}
	problems = append(problems, overlapsWithinRequest(wins, in.HasOccurrences)...)
	if len(problems) > 0 {
		return CreateResult{}, httpx.ValidationError(problems...)
	}

	var result CreateResult
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)

		// The requester lock comes first and is held to commit, so two requests of one requester for
		// different rooms are counted one after the other. Locks are always taken requester, then room.
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

		if res, ok, err := s.findReplay(ctx, q, actor, in, wins); err != nil || ok {
			result = res
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
		requester, err := q.GetUserByID(ctx, actor.UserID)
		if err != nil {
			return fmt.Errorf("loading requester: %w", err)
		}
		if requester.BookingBlockedUntil.Valid && requester.BookingBlockedUntil.Time.After(now) {
			return bookingBlocked(requester.BookingBlockedUntil.Time)
		}
		if !room.IsActive {
			return httpx.ErrRoomInactive
		}

		if err := s.checkRequest(ctx, q, actor, in, wins, policy, room, now); err != nil {
			return err
		}
		details, _ := normalizeDetails(string(room.Type), in.Details) // already validated by checkRequest

		active, err := q.CountActiveRequests(ctx, dbgen.CountActiveRequestsParams{RequesterID: actor.UserID, EndAt: now})
		if err != nil {
			return fmt.Errorf("counting active requests: %w", err)
		}
		if limit := s.maxActive(actor); int(active) >= limit {
			return quotaExceeded(limit)
		}

		var collisions []httpx.ErrorDetail
		for _, w := range wins {
			found, err := overlap.Find(ctx, q, room.ID, []overlap.Window{{Start: w.start, End: w.end}}, uuid.Nil, uuid.Nil)
			if err != nil {
				return err
			}
			collisions = append(collisions, tagList(found, in.occurrenceTag(w))...)
		}
		if len(collisions) > 0 {
			msg := conflictMessage
			if in.HasOccurrences && len(wins) > 1 {
				msg = "Satu atau lebih tanggal bentrok; tidak ada pengajuan yang dibuat."
			}
			return overlap.Error(msg, collisions)
		}

		points, err := violations.ActivePoints(ctx, q, s.settings.Violations, actor.UserID, now)
		if err != nil {
			return err
		}
		decision, err := DecideApproval(ApprovalInput{
			Requester: actor.Role, Policy: policy, ActivePoints: points, AutoApproveMaxPoints: s.settings.AutoApproveMaxPoints,
		})
		if err != nil {
			return httpx.ErrRoomTypeNotAllowed
		}

		rows, err := s.insertRows(ctx, q, actor, in, wins, room, details, decision, now)
		if err != nil {
			return err
		}
		result, err = s.resultFor(ctx, q, in.HasOccurrences, rows)
		if err != nil {
			return err
		}
		if decision.Mode == ModeManual {
			first, err := q.GetBooking(ctx, rows[0].ID)
			if err != nil {
				return fmt.Errorf("reading created booking: %w", err)
			}
			return s.notifyCreated(ctx, q, fromRow(first), len(rows), now)
		}
		return nil
	})
	if err != nil {
		return CreateResult{}, s.mapWriteError(err, "creating booking")
	}
	return result, nil
}

// checkRequest applies the checks that need the room and its policy: each date against the type's limits
// and the room's hours (BR-01, BR-20, BR-29), the head count (BR-21), the details (BR-20) and the letter
// (BR-18). All problems are reported together; the hours error comes alone because it says which window applies.
func (s *Service) checkRequest(ctx context.Context, q *dbgen.Queries, actor httpx.Principal, in CreateInput, wins []window,
	policy roomtypes.Policy, room dbgen.LockRoomRow, now time.Time) error {

	var problems []httpx.ErrorDetail
	for _, w := range wins {
		if err := validatePolicyWindow(policy, room.OpenTime, room.CloseTime, w.start, w.end, now); err != nil {
			err = tagDetails(err, in.occurrenceTag(w))
			var ce *httpx.CustomDomainError
			if errors.As(err, &ce) && ce.Code == "OUTSIDE_OPERATING_HOURS" {
				return err
			}
			if errors.As(err, &ce) {
				problems = append(problems, ce.Details...)
				continue
			}
			return err
		}
	}

	// A head count above the room's capacity cannot be seated (BR-21); an auditorium must give one.
	if in.Participants != nil && int32(*in.Participants) > room.Capacity {
		problems = append(problems, httpx.ErrorDetail{Field: "participants", Message: fmt.Sprintf("Jumlah peserta melebihi kapasitas ruang (%d).", room.Capacity)})
	}
	if in.Participants == nil && specFor(string(room.Type)).participantsRequired {
		problems = append(problems, httpx.ErrorDetail{Field: "participants", Message: "Jumlah peserta wajib diisi untuk tipe ruang ini."})
	}
	if _, bad := normalizeDetails(string(room.Type), in.Details); len(bad) > 0 {
		problems = append(problems, bad...)
	}
	letterProblems, err := s.checkLetter(ctx, q, actor, policy, in.LetterAttachmentID)
	if err != nil {
		return err
	}
	problems = append(problems, letterProblems...)

	if len(problems) > 0 {
		return httpx.ValidationError(problems...)
	}
	return nil
}

// checkLetter enforces BR-18: a letter is mandatory for organisations and for room types that require one,
// and whenever one is named it must be the requester's own, a letter, and not yet deleted.
func (s *Service) checkLetter(ctx context.Context, q *dbgen.Queries, actor httpx.Principal, policy roomtypes.Policy, id *uuid.UUID) ([]httpx.ErrorDetail, error) {
	required := actor.Role == httpx.RoleOrganisasi || policy.LetterRequired
	if id == nil {
		if required {
			return []httpx.ErrorDetail{{Field: "letter_attachment_id", Message: "Surat tugas atau surat kegiatan wajib dilampirkan."}}, nil
		}
		return nil, nil
	}
	row, err := q.GetLetterForRequester(ctx, *id)
	if errors.Is(err, pgx.ErrNoRows) {
		return []httpx.ErrorDetail{{Field: "letter_attachment_id", Message: "Surat tidak ditemukan."}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading letter: %w", err)
	}
	switch {
	case row.UploadedBy != actor.UserID:
		// Someone else's file is reported exactly like a missing one.
		return []httpx.ErrorDetail{{Field: "letter_attachment_id", Message: "Surat tidak ditemukan."}}, nil
	case row.Kind != dbgen.AttachmentKindLETTER:
		return []httpx.ErrorDetail{{Field: "letter_attachment_id", Message: "Lampiran ini bukan surat."}}, nil
	case row.PurgedAt.Valid:
		return []httpx.ErrorDetail{{Field: "letter_attachment_id", Message: "Surat sudah dihapus; unggah kembali."}}, nil
	}
	return nil, nil
}

// insertRows writes one row per date, with a shared group id when there are several (BR-27), plus the
// history. A request decided by the system gets two events, MENUNGGU by the requester and DISETUJUI by the
// system (BR-17); the Idempotency-Key is kept on the first row only.
func (s *Service) insertRows(ctx context.Context, q *dbgen.Queries, actor httpx.Principal, in CreateInput, wins []window,
	room dbgen.LockRoomRow, details []byte, decision Decision, now time.Time) ([]dbgen.Booking, error) {

	var groupID pgtype.UUID
	if in.HasOccurrences {
		groupID = pgtype.UUID{Bytes: uuid.New(), Valid: true}
	}

	status := dbgen.BookingStatusMENUNGGU
	var decidedAt pgtype.Timestamptz
	var manual dbgen.NullManualReason
	if decision.Mode == ModeAuto {
		status = dbgen.BookingStatusDISETUJUI
		decidedAt = pgtype.Timestamptz{Time: now, Valid: true}
	} else {
		manual = dbgen.NullManualReason{ManualReason: dbgen.ManualReason(decision.Reason), Valid: true}
	}

	// A request that waits expires 24 hours after it was made, or when it would have started, whichever is
	// first; every row of a group shares the value, counted from the earliest date (BR-22).
	var expires pgtype.Timestamptz
	if status == dbgen.BookingStatusMENUNGGU {
		firstStart := wins[0].start
		for _, w := range wins {
			if w.start.Before(firstStart) {
				firstStart = w.start
			}
		}
		expires = pgtype.Timestamptz{Time: earliest(now.Add(s.settings.PendingTTL), firstStart), Valid: true}
	}

	rows := make([]dbgen.Booking, 0, len(wins))
	for i, w := range wins {
		params := dbgen.CreateBookingParams{
			RoomID: room.ID, RequesterID: actor.UserID, StartAt: w.start, EndAt: w.end, Purpose: in.Purpose,
			Details: details, GroupID: groupID, ExpiresAt: expires, Status: status, DecidedAt: decidedAt,
			ApprovalMode: dbgen.ApprovalMode(decision.Mode), ManualReason: manual, CreatedAt: now,
		}
		if in.HasOccurrences {
			params.OccurrenceNo = pgtype.Int4{Int32: int32(w.no), Valid: true}
		}
		if in.Participants != nil {
			params.Participants = pgtype.Int4{Int32: int32(*in.Participants), Valid: true}
		}
		if in.Notes != nil {
			params.Notes = pgtype.Text{String: *in.Notes, Valid: true}
		}
		if in.LetterAttachmentID != nil {
			params.LetterAttachmentID = pgtype.UUID{Bytes: *in.LetterAttachmentID, Valid: true}
		}
		if in.LetterNumber != nil {
			params.LetterNumber = pgtype.Text{String: *in.LetterNumber, Valid: true}
		}
		if i == 0 && in.IdempotencyKey != "" {
			params.IdempotencyKey = pgtype.Text{String: in.IdempotencyKey, Valid: true}
		}
		row, err := q.CreateBooking(ctx, params)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)

		if err := q.CreateBookingEvent(ctx, dbgen.CreateBookingEventParams{
			BookingID: row.ID, ActorID: pgtype.UUID{Bytes: actor.UserID, Valid: true},
			ToStatus: dbgen.BookingStatusMENUNGGU, Kind: "STATUS", CreatedAt: now,
		}); err != nil {
			return nil, fmt.Errorf("recording booking event: %w", err)
		}
		if decision.Mode == ModeAuto {
			if err := q.CreateBookingEvent(ctx, dbgen.CreateBookingEventParams{
				BookingID:  row.ID, // no actor: the system decided
				FromStatus: dbgen.NullBookingStatus{BookingStatus: dbgen.BookingStatusMENUNGGU, Valid: true},
				ToStatus:   dbgen.BookingStatusDISETUJUI, Reason: pgtype.Text{String: autoApprovedReason, Valid: true},
				Kind: "STATUS", CreatedAt: now,
			}); err != nil {
				return nil, fmt.Errorf("recording auto-approval event: %w", err)
			}
		}
	}
	return rows, nil
}

// resultFor shapes the response: the booking itself for one date, a group summary for a multi-date request.
func (s *Service) resultFor(ctx context.Context, q *dbgen.Queries, grouped bool, rows []dbgen.Booking) (CreateResult, error) {
	if !grouped {
		view, err := q.GetBooking(ctx, rows[0].ID)
		if err != nil {
			return CreateResult{}, fmt.Errorf("reading created booking: %w", err)
		}
		b := fromRow(view)
		return CreateResult{Booking: &b}, nil
	}
	first := rows[0]
	g := GroupResult{
		GroupID: uuid.UUID(first.GroupID.Bytes), Status: string(first.Status), ApprovalMode: string(first.ApprovalMode),
		Bookings: make([]OccurrenceRef, len(rows)),
	}
	if first.ManualReason.Valid {
		r := string(first.ManualReason.ManualReason)
		g.ManualReason = &r
	}
	if first.ExpiresAt.Valid {
		t := clock.ToWIB(first.ExpiresAt.Time)
		g.ExpiresAt = &t
	}
	for i, r := range rows {
		g.Bookings[i] = OccurrenceRef{
			ID: r.ID, OccurrenceNo: int(r.OccurrenceNo.Int32), StartAt: clock.ToWIB(r.StartAt), EndAt: clock.ToWIB(r.EndAt), Status: string(r.Status),
		}
	}
	return CreateResult{Group: &g}, nil
}

// findReplay returns what an earlier request with the same Idempotency-Key created. Reusing a key for a
// different request is a client bug, reported as a validation error rather than silently returning an
// unrelated booking.
func (s *Service) findReplay(ctx context.Context, q *dbgen.Queries, actor httpx.Principal, in CreateInput, wins []window) (CreateResult, bool, error) {
	if in.IdempotencyKey == "" {
		return CreateResult{}, false, nil
	}
	first, err := q.GetBookingByIdempotencyKey(ctx, dbgen.GetBookingByIdempotencyKeyParams{
		RequesterID: actor.UserID, IdempotencyKey: pgtype.Text{String: in.IdempotencyKey, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CreateResult{}, false, nil
	}
	if err != nil {
		return CreateResult{}, false, fmt.Errorf("looking up idempotency key: %w", err)
	}

	rows := []dbgen.Booking{first}
	if first.GroupID.Valid {
		if rows, err = q.ListGroupBookings(ctx, first.GroupID); err != nil {
			return CreateResult{}, false, fmt.Errorf("loading replayed group: %w", err)
		}
	}
	if !sameRequest(rows, in, wins) {
		return CreateResult{}, false, idempotencyReuse()
	}
	res, err := s.resultFor(ctx, q, first.GroupID.Valid, rows)
	if err != nil {
		return CreateResult{}, false, err
	}
	res.Replayed = true
	return res, true, nil
}

// sameRequest compares a stored request with a new one field by field.
func sameRequest(rows []dbgen.Booking, in CreateInput, wins []window) bool {
	if len(rows) != len(wins) || rows[0].GroupID.Valid != in.HasOccurrences {
		return false
	}
	for i, row := range rows {
		w := wins[i]
		if row.RoomID != in.RoomID || !row.StartAt.Equal(w.start) || !row.EndAt.Equal(w.end) || row.Purpose != in.Purpose {
			return false
		}
		if row.Participants.Valid != (in.Participants != nil) || (in.Participants != nil && int(row.Participants.Int32) != *in.Participants) {
			return false
		}
		if row.Notes.Valid != (in.Notes != nil) || (in.Notes != nil && row.Notes.String != *in.Notes) {
			return false
		}
		if row.LetterAttachmentID.Valid != (in.LetterAttachmentID != nil) ||
			(in.LetterAttachmentID != nil && uuid.UUID(row.LetterAttachmentID.Bytes) != *in.LetterAttachmentID) {
			return false
		}
		if row.LetterNumber.Valid != (in.LetterNumber != nil) || (in.LetterNumber != nil && row.LetterNumber.String != *in.LetterNumber) {
			return false
		}
		if !sameJSON(row.Details, in.Details) {
			return false
		}
	}
	return true
}

// sameJSON compares two JSON values by meaning, so key order and spacing do not matter; a missing value is {}.
func sameJSON(stored []byte, sent json.RawMessage) bool {
	decode := func(b []byte) any {
		var v any
		if len(strings.TrimSpace(string(b))) == 0 || json.Unmarshal(b, &v) != nil || v == nil {
			return map[string]any{}
		}
		return v
	}
	return reflect.DeepEqual(decode(stored), decode(sent))
}

// validateCommon applies the field rules shared by creating and editing a booking.
func validateCommon(purpose string, participants *int, notes, letterNumber *string) []httpx.ErrorDetail {
	var problems []httpx.ErrorDetail
	if n := utf8.RuneCountInString(purpose); n < 1 || n > maxPurposeRunes {
		problems = append(problems, httpx.ErrorDetail{Field: "purpose", Message: fmt.Sprintf("Keperluan wajib diisi (maksimal %d karakter).", maxPurposeRunes)})
	}
	if participants != nil && (*participants < 1 || *participants > maxParticipants) {
		problems = append(problems, httpx.ErrorDetail{Field: "participants", Message: fmt.Sprintf("Jumlah peserta harus antara 1 dan %d.", maxParticipants)})
	}
	if notes != nil && utf8.RuneCountInString(*notes) > maxNotesRunes {
		problems = append(problems, httpx.ErrorDetail{Field: "notes", Message: fmt.Sprintf("Catatan maksimal %d karakter.", maxNotesRunes)})
	}
	if letterNumber != nil && utf8.RuneCountInString(*letterNumber) > maxLetterNumberRunes {
		problems = append(problems, httpx.ErrorDetail{Field: "letter_number", Message: fmt.Sprintf("Nomor surat maksimal %d karakter.", maxLetterNumberRunes)})
	}
	return problems
}

// overlapsWithinRequest refuses dates of one request that overlap each other (BR-27).
func overlapsWithinRequest(wins []window, grouped bool) []httpx.ErrorDetail {
	if !grouped || len(wins) < 2 {
		return nil
	}
	sorted := make([]window, len(wins))
	copy(sorted, wins)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].start.Before(sorted[j].start) })
	var problems []httpx.ErrorDetail
	for i := 1; i < len(sorted); i++ {
		if sorted[i].start.Before(sorted[i-1].end) {
			problems = append(problems, httpx.ErrorDetail{
				Field: "start_at", OccurrenceNo: sorted[i].no,
				Message: fmt.Sprintf("Tanggal ini tumpang tindih dengan tanggal ke-%d dalam pengajuan yang sama.", sorted[i-1].no),
			})
		}
	}
	return problems
}

func trimmedOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

func earliest(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
