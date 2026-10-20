package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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

// EditInput holds only the fields the caller sent; nil means "leave unchanged". The room cannot be changed:
// cancel and ask again (BR-26). An empty notes or letter_number clears it.
type EditInput struct {
	StartAt            *time.Time
	EndAt              *time.Time
	Purpose            *string
	Participants       *int
	Notes              *string
	Details            *json.RawMessage
	LetterAttachmentID *uuid.UUID
	LetterNumber       *string
}

func (in EditInput) empty() bool {
	return in.StartAt == nil && in.EndAt == nil && in.Purpose == nil && in.Participants == nil && in.Notes == nil &&
		in.Details == nil && in.LetterAttachmentID == nil && in.LetterNumber == nil
}

// change is one field of an edit, recorded in the history as { from, to }.
type change struct {
	From any `json:"from"`
	To   any `json:"to"`
}

// Edit lets the owner change a waiting or approved date up to EDIT_CUTOFF before it starts (BR-26). The
// change goes through the same checks as creating (type limits, hours, capacity, letter, collisions with
// everything but the booking itself) and then decides what the booking becomes:
//
//   - waiting: it keeps waiting and its expiry is not extended;
//   - approved by the system: the approval decision is made again, so a booking that no longer qualifies
//     goes back to waiting for a person;
//   - approved by a person: a material change (time, head count, purpose, details, letter) sends it back to
//     waiting for approval again, with a fresh expiry; a change to the notes alone changes nothing else.
//
// Every edit adds an EDITED event with what changed. In a multi-date request an edit touches one date only.
func (s *Service) Edit(ctx context.Context, actor httpx.Principal, id uuid.UUID, in EditInput) (Booking, error) {
	if !isRequester(actor) {
		return Booking{}, httpx.ErrForbidden
	}
	if in.empty() {
		return Booking{}, httpx.ValidationError(httpx.ErrorDetail{Message: "Tidak ada field yang diubah."})
	}
	peek, err := s.peek(ctx, actor, id)
	if err != nil {
		return Booking{}, err
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
		cur, err := q.LoadBooking(ctx, id)
		if err != nil {
			return fmt.Errorf("re-reading booking: %w", err)
		}
		if cur.Status != dbgen.BookingStatusMENUNGGU && cur.Status != dbgen.BookingStatusDISETUJUI {
			return invalidState(fmt.Sprintf("Booking berstatus %s tidak dapat diubah.", cur.Status))
		}
		// "Up to 60 minutes before it starts": the start that applies now, not the one being asked for.
		if now.After(cur.StartAt.Add(-s.settings.EditCutoff)) {
			return editWindowClosed(s.settings.EditCutoff)
		}

		policy, err := s.types.Get(ctx, string(room.Type))
		if err != nil {
			return fmt.Errorf("loading room type policy: %w", err)
		}

		next := editedValues(cur, in)
		var problems []httpx.ErrorDetail
		problems = append(problems, validateCommon(next.purpose, next.participants, next.notes, next.letterNumber)...)
		problems = append(problems, validateWindow(next.start, next.end, now)...)
		if len(problems) > 0 {
			return httpx.ValidationError(problems...)
		}
		// The type's limits and the room's hours apply to the new window just as they did when it was made.
		inForCheck := CreateInput{
			Participants: next.participants, Details: next.details, LetterAttachmentID: next.letterID, LetterNumber: next.letterNumber,
		}
		if err := s.checkRequest(ctx, q, actor, inForCheck, []window{{no: 1, start: next.start, end: next.end}}, policy, room, now); err != nil {
			return err
		}
		// A booking never collides with itself.
		conflicts, err := overlap.Find(ctx, q, room.ID, []overlap.Window{{Start: next.start, End: next.end}}, uuid.Nil, cur.ID)
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			return overlap.Error(conflictMessage, conflicts)
		}

		changes := diff(cur, next)
		if len(changes) == 0 {
			view, err := q.GetBooking(ctx, id)
			if err != nil {
				return fmt.Errorf("reading booking: %w", err)
			}
			result = fromRow(view)
			return nil
		}
		material := isMaterial(changes)

		state, err := s.stateAfterEdit(ctx, q, actor, cur, policy, material, next.start, now)
		if err != nil {
			return err
		}

		row, err := q.UpdateBookingEdit(ctx, dbgen.UpdateBookingEditParams{
			ID: id, StartAt: next.start, EndAt: next.end, Purpose: next.purpose,
			Participants: intPtr(next.participants), Notes: textFor(next.notes), Details: next.details,
			LetterAttachmentID: uuidFor(next.letterID), LetterNumber: textFor(next.letterNumber),
			Status: state.status, ApprovalMode: state.mode, ManualReason: state.reason, ExpiresAt: state.expiresAt,
			DecidedBy: state.decidedBy, DecidedAt: state.decidedAt, DecisionReason: state.decisionReason,
			Now: pgtype.Timestamptz{Time: now, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("updating booking: %w", err)
		}

		raw, err := json.Marshal(changes)
		if err != nil {
			return fmt.Errorf("encoding changes: %w", err)
		}
		if err := q.CreateBookingEvent(ctx, dbgen.CreateBookingEventParams{
			BookingID: id, ActorID: pgtype.UUID{Bytes: actor.UserID, Valid: true},
			ToStatus: row.Status, Kind: "EDITED", Changes: raw, CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("recording edit: %w", err)
		}
		if row.Status != cur.Status {
			if err := q.CreateBookingEvent(ctx, dbgen.CreateBookingEventParams{
				BookingID: id, ActorID: pgtype.UUID{Bytes: actor.UserID, Valid: true},
				FromStatus: dbgen.NullBookingStatus{BookingStatus: cur.Status, Valid: true}, ToStatus: row.Status,
				Reason: pgtype.Text{String: state.statusReason, Valid: true}, Kind: "STATUS", CreatedAt: now,
			}); err != nil {
				return fmt.Errorf("recording status change: %w", err)
			}
		}

		view, err := q.GetBooking(ctx, id)
		if err != nil {
			return fmt.Errorf("reading updated booking: %w", err)
		}
		result = fromRow(view)
		// Approvers hear about a request that waits again, not about a change they need not act on.
		if material && row.Status == dbgen.BookingStatusMENUNGGU {
			return notifyEdited(ctx, q, result, now)
		}
		return nil
	})
	if err != nil {
		return Booking{}, s.mapWriteError(err, "updating booking")
	}
	return result, nil
}

// editedValues is the booking as it would be after applying the request.
type editedBooking struct {
	start, end   time.Time
	purpose      string
	participants *int
	notes        *string
	details      []byte
	letterID     *uuid.UUID
	letterNumber *string
}

func editedValues(cur dbgen.Booking, in EditInput) editedBooking {
	next := editedBooking{
		start: cur.StartAt, end: cur.EndAt, purpose: cur.Purpose, details: cur.Details,
	}
	if cur.Participants.Valid {
		n := int(cur.Participants.Int32)
		next.participants = &n
	}
	if cur.Notes.Valid {
		next.notes = &cur.Notes.String
	}
	if cur.LetterAttachmentID.Valid {
		id := uuid.UUID(cur.LetterAttachmentID.Bytes)
		next.letterID = &id
	}
	if cur.LetterNumber.Valid {
		next.letterNumber = &cur.LetterNumber.String
	}

	if in.StartAt != nil {
		next.start = in.StartAt.UTC()
	}
	if in.EndAt != nil {
		next.end = in.EndAt.UTC()
	}
	if in.Purpose != nil {
		next.purpose = strings.TrimSpace(*in.Purpose)
	}
	if in.Participants != nil {
		next.participants = in.Participants
	}
	if in.Notes != nil {
		next.notes = trimmedOrNil(in.Notes)
	}
	if in.Details != nil {
		next.details = []byte(*in.Details)
	}
	if in.LetterAttachmentID != nil {
		next.letterID = in.LetterAttachmentID
	}
	if in.LetterNumber != nil {
		next.letterNumber = trimmedOrNil(in.LetterNumber)
	}
	return next
}

// diff lists what the edit changes, as { field: { from, to } }.
func diff(cur dbgen.Booking, next editedBooking) map[string]change {
	out := map[string]change{}
	if !cur.StartAt.Equal(next.start) {
		out["start_at"] = change{clock.ToWIB(cur.StartAt).Format(time.RFC3339), clock.ToWIB(next.start).Format(time.RFC3339)}
	}
	if !cur.EndAt.Equal(next.end) {
		out["end_at"] = change{clock.ToWIB(cur.EndAt).Format(time.RFC3339), clock.ToWIB(next.end).Format(time.RFC3339)}
	}
	if cur.Purpose != next.purpose {
		out["purpose"] = change{cur.Purpose, next.purpose}
	}
	var curParticipants *int
	if cur.Participants.Valid {
		n := int(cur.Participants.Int32)
		curParticipants = &n
	}
	if !equalInt(curParticipants, next.participants) {
		out["participants"] = change{deref(curParticipants), deref(next.participants)}
	}
	if !equalText(textOf(cur.Notes), next.notes) {
		out["notes"] = change{derefText(textOf(cur.Notes)), derefText(next.notes)}
	}
	if !sameJSON(cur.Details, next.details) {
		out["details"] = change{json.RawMessage(cur.Details), json.RawMessage(next.details)}
	}
	var curLetter *uuid.UUID
	if cur.LetterAttachmentID.Valid {
		id := uuid.UUID(cur.LetterAttachmentID.Bytes)
		curLetter = &id
	}
	if (curLetter == nil) != (next.letterID == nil) || (curLetter != nil && *curLetter != *next.letterID) {
		out["letter_attachment_id"] = change{derefUUID(curLetter), derefUUID(next.letterID)}
	}
	if !equalText(textOf(cur.LetterNumber), next.letterNumber) {
		out["letter_number"] = change{derefText(textOf(cur.LetterNumber)), derefText(next.letterNumber)}
	}
	return out
}

// isMaterial says whether the edit changes anything an approver decided on: a change to the notes alone
// does not (BR-26).
func isMaterial(changes map[string]change) bool {
	for field := range changes {
		if field != "notes" {
			return true
		}
	}
	return false
}

// editState is the status and approval fields a booking ends up with after an edit.
type editState struct {
	status         dbgen.BookingStatus
	mode           dbgen.ApprovalMode
	reason         dbgen.NullManualReason
	expiresAt      pgtype.Timestamptz
	decidedBy      pgtype.UUID
	decidedAt      pgtype.Timestamptz
	decisionReason pgtype.Text
	statusReason   string
}

func (s *Service) stateAfterEdit(ctx context.Context, q *dbgen.Queries, actor httpx.Principal, cur dbgen.Booking,
	policy roomtypes.Policy, material bool, newStart, now time.Time) (editState, error) {

	keep := editState{
		status: cur.Status, mode: cur.ApprovalMode, reason: cur.ManualReason, expiresAt: cur.ExpiresAt,
		decidedBy: cur.DecidedBy, decidedAt: cur.DecidedAt, decisionReason: cur.DecisionReason,
	}

	if cur.Status == dbgen.BookingStatusMENUNGGU {
		// Waiting stays waiting, and the clock is never extended; an earlier start can only shorten it.
		if cur.ExpiresAt.Valid && newStart.Before(cur.ExpiresAt.Time) {
			keep.expiresAt = pgtype.Timestamptz{Time: newStart, Valid: true}
		}
		return keep, nil
	}
	if !material {
		return keep, nil
	}

	waiting := func(mode dbgen.ApprovalMode, reason dbgen.NullManualReason, why string) editState {
		return editState{
			status: dbgen.BookingStatusMENUNGGU, mode: mode, reason: reason,
			expiresAt:    pgtype.Timestamptz{Time: earliest(now.Add(s.settings.PendingTTL), newStart), Valid: true},
			statusReason: why,
		}
	}
	if cur.ApprovalMode == dbgen.ApprovalModeMANUAL {
		return waiting(dbgen.ApprovalModeMANUAL, cur.ManualReason, "Perubahan memerlukan persetujuan ulang."), nil
	}

	// Approved by the system: ask the same question again with today's facts (BR-17).
	points, err := violations.ActivePoints(ctx, q, s.settings.Violations, cur.RequesterID, now)
	if err != nil {
		return editState{}, err
	}
	decision, err := DecideApproval(ApprovalInput{
		Requester: actor.Role, Policy: policy, ActivePoints: points, AutoApproveMaxPoints: s.settings.AutoApproveMaxPoints,
	})
	if err != nil {
		return editState{}, httpx.ErrRoomTypeNotAllowed
	}
	if decision.Mode == ModeAuto {
		return keep, nil
	}
	return waiting(dbgen.ApprovalModeMANUAL,
		dbgen.NullManualReason{ManualReason: dbgen.ManualReason(decision.Reason), Valid: true},
		"Perubahan tidak lagi memenuhi syarat persetujuan otomatis."), nil
}

func intPtr(p *int) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*p), Valid: true}
}

func textFor(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func uuidFor(p *uuid.UUID) pgtype.UUID {
	if p == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *p, Valid: true}
}

func textOf(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func equalInt(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalText(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func derefText(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func derefUUID(p *uuid.UUID) any {
	if p == nil {
		return nil
	}
	return p.String()
}
