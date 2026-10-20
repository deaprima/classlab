package roomtypes

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"classlab/api/internal/audit"
	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	maxAdvanceDaysLimit = 365
)

// Roles the policy may name. Approvers are the staff roles (the database enforces the same set);
// requesters are the roles that book.
var (
	approverCandidates  = []httpx.Role{httpx.RoleTU, httpx.RoleKalab, httpx.RoleAdmin}
	requesterCandidates = []httpx.Role{httpx.RoleMahasiswa, httpx.RoleOrganisasi}
)

// View is the API representation of a policy (GET /room-types).
type View struct {
	Type           string       `json:"type"`
	Label          string       `json:"label"`
	ApproverRoles  []httpx.Role `json:"approver_roles"`
	RequesterRoles []httpx.Role `json:"requester_roles"`
	ApprovalPolicy string       `json:"approval_policy"`
	LetterRequired bool         `json:"letter_required"`
	OpenTime       string       `json:"open_time"`
	CloseTime      string       `json:"close_time"`
	MinDurationMin int          `json:"min_duration_min"`
	MaxDurationMin int          `json:"max_duration_min"`
	MaxAdvanceDays int          `json:"max_advance_days"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// View converts a policy to its API form.
func (p Policy) View() View {
	return View{
		Type: p.Type, Label: p.Label,
		ApproverRoles: p.ApproverRoles, RequesterRoles: p.RequesterRoles,
		ApprovalPolicy: p.ApprovalPolicy, LetterRequired: p.LetterRequired,
		OpenTime: FormatClock(p.OpenMinute), CloseTime: FormatClock(p.CloseMinute),
		MinDurationMin: int(p.MinDuration / time.Minute), MaxDurationMin: int(p.MaxDuration / time.Minute),
		MaxAdvanceDays: p.MaxAdvanceDays, UpdatedAt: clock.ToWIB(p.UpdatedAt),
	}
}

// UpdateInput holds only the fields the caller sent; nil means "leave unchanged".
type UpdateInput struct {
	ApproverRoles  *[]string
	RequesterRoles *[]string
	ApprovalPolicy *string
	LetterRequired *bool
	OpenTime       *string
	CloseTime      *string
	MinDurationMin *int
	MaxDurationMin *int
	MaxAdvanceDays *int
}

func (in UpdateInput) empty() bool {
	return in.ApproverRoles == nil && in.RequesterRoles == nil && in.ApprovalPolicy == nil &&
		in.LetterRequired == nil && in.OpenTime == nil && in.CloseTime == nil &&
		in.MinDurationMin == nil && in.MaxDurationMin == nil && in.MaxAdvanceDays == nil
}

type Service struct {
	pool  *db.Pool
	q     *dbgen.Queries
	store *Store
	clock clock.Clock
}

func NewService(pool *db.Pool, clk clock.Clock) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), store: NewStore(pool), clock: clk}
}

// List returns every room type with its policy. It is open to every signed-in role: clients use it to
// decide which menus and forms to offer, while the server stays the authority on every action.
func (s *Service) List(ctx context.Context) ([]View, error) {
	all, err := s.store.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]View, len(all))
	for i, p := range all {
		out[i] = p.View()
	}
	return out, nil
}

// Update changes a policy (BR-20). Only administrators may. The change applies from the next request:
// bookings already made keep the decision they were made under.
func (s *Service) Update(ctx context.Context, actor httpx.Principal, roomType string, in UpdateInput) (View, error) {
	if actor.Role != httpx.RoleAdmin {
		return View{}, httpx.ErrForbidden
	}
	if !dbgen.RoomType(roomType).Valid() {
		return View{}, httpx.ErrNotFound
	}
	if in.empty() {
		return View{}, httpx.ValidationError(httpx.ErrorDetail{Message: "Tidak ada field yang diubah."})
	}

	params := dbgen.UpdateRoomTypePolicyParams{RoomType: dbgen.RoomType(roomType)}
	var problems []httpx.ErrorDetail
	bad := func(field, msg string) { problems = append(problems, httpx.ErrorDetail{Field: field, Message: msg}) }

	if in.ApproverRoles != nil {
		roles, msg := normalizeRoles(*in.ApproverRoles, approverCandidates)
		if msg != "" {
			bad("approver_roles", msg)
		}
		params.ApproverRoles = roles
	}
	if in.RequesterRoles != nil {
		roles, msg := normalizeRoles(*in.RequesterRoles, requesterCandidates)
		if msg != "" {
			bad("requester_roles", msg)
		}
		params.RequesterRoles = roles
	}
	if in.ApprovalPolicy != nil {
		p := dbgen.ApprovalPolicy(*in.ApprovalPolicy)
		if !p.Valid() {
			bad("approval_policy", "approval_policy harus AUTO_IF_ELIGIBLE atau ALWAYS_MANUAL.")
		}
		params.ApprovalPolicy = dbgen.NullApprovalPolicy{ApprovalPolicy: p, Valid: true}
	}
	if in.LetterRequired != nil {
		params.LetterRequired = pgtype.Bool{Bool: *in.LetterRequired, Valid: true}
	}
	if in.MaxAdvanceDays != nil {
		if *in.MaxAdvanceDays < 1 || *in.MaxAdvanceDays > maxAdvanceDaysLimit {
			bad("max_advance_days", fmt.Sprintf("max_advance_days harus antara 1 dan %d.", maxAdvanceDaysLimit))
		}
		params.MaxAdvanceDays = pgtype.Int4{Int32: int32(*in.MaxAdvanceDays), Valid: true}
	}
	var open, close *int
	if in.OpenTime != nil {
		m, ok := ParseClock(*in.OpenTime)
		if !ok {
			bad("open_time", "open_time harus berformat HH:MM.")
		}
		open = &m
	}
	if in.CloseTime != nil {
		m, ok := ParseClock(*in.CloseTime)
		if !ok {
			bad("close_time", "close_time harus berformat HH:MM.")
		}
		close = &m
	}
	if len(problems) > 0 {
		return View{}, httpx.ValidationError(problems...)
	}

	var before, after View
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()
		params.UpdatedAt = now

		cur, err := q.LockRoomTypePolicy(ctx, dbgen.RoomType(roomType))
		if err != nil {
			return err
		}
		curPolicy := fromRow(policyRow(cur))
		before = curPolicy.View()

		// Cross-field rules look at the result of the change, not at the fields alone.
		finalOpen, finalClose := curPolicy.OpenMinute, curPolicy.CloseMinute
		if open != nil {
			finalOpen = *open
		}
		if close != nil {
			finalClose = *close
		}
		finalMin, finalMax := int(curPolicy.MinDuration/time.Minute), int(curPolicy.MaxDuration/time.Minute)
		if in.MinDurationMin != nil {
			finalMin = *in.MinDurationMin
		}
		if in.MaxDurationMin != nil {
			finalMax = *in.MaxDurationMin
		}

		var cross []httpx.ErrorDetail
		if (open != nil || close != nil) && !ValidHours(finalOpen, finalClose) {
			cross = append(cross, httpx.ErrorDetail{Field: "open_time", Message: fmt.Sprintf("Jam buka harus sebelum jam tutup dan berkelipatan %d menit.", MinutesPerSlot)})
		}
		if in.MinDurationMin != nil || in.MaxDurationMin != nil {
			if finalMin < MinutesPerSlot || finalMin%MinutesPerSlot != 0 || finalMax%MinutesPerSlot != 0 || finalMin > finalMax {
				cross = append(cross, httpx.ErrorDetail{Field: "min_duration_min", Message: fmt.Sprintf("Durasi harus berkelipatan %d menit dan durasi minimal tidak boleh melebihi maksimal.", MinutesPerSlot)})
			}
		}
		if len(cross) > 0 {
			return httpx.ValidationError(cross...)
		}

		if open != nil {
			params.OpenTime = PGTime(*open)
		}
		if close != nil {
			params.CloseTime = PGTime(*close)
		}
		if in.MinDurationMin != nil {
			params.MinDurationMin = pgtype.Int4{Int32: int32(*in.MinDurationMin), Valid: true}
		}
		if in.MaxDurationMin != nil {
			params.MaxDurationMin = pgtype.Int4{Int32: int32(*in.MaxDurationMin), Valid: true}
		}

		row, err := q.UpdateRoomTypePolicy(ctx, params)
		if err != nil {
			return err
		}
		after = fromRow(policyRow(row)).View()
		return audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionRoomTypePolicyUpdated, EntityType: audit.EntityRoomType,
			Before: before, After: after, At: now,
		})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return View{}, httpx.ErrNotFound
	}
	if err != nil {
		var domain *httpx.CustomDomainError
		if errors.As(err, &domain) {
			return View{}, err
		}
		return View{}, fmt.Errorf("updating room type policy: %w", err)
	}
	return after, nil
}

// normalizeRoles checks every role is one of the allowed ones, removes repeats and refuses an empty list:
// a type nobody may approve, or nobody may book, would be a dead end rather than a policy.
func normalizeRoles(in []string, allowed []httpx.Role) ([]string, string) {
	if len(in) == 0 {
		return nil, "Minimal satu peran harus dipilih."
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		role := httpx.Role(v)
		if !slices.Contains(allowed, role) {
			return nil, "Peran " + v + " tidak diizinkan di sini."
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out, ""
}
