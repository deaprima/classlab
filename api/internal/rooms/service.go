package rooms

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"classlab/api/internal/audit"
	"classlab/api/internal/httpx"
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
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"

	maxFacilities     = 30
	maxFacilityLength = 60
	maxCapacity       = 10000
)

var codePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$`)

type Service struct {
	pool  *db.Pool
	q     *dbgen.Queries
	clock clock.Clock
	types *roomtypes.Store
	// publicBaseURL is the frontend address the QR links to (the first CORS origin; see app.NewRouter).
	publicBaseURL string
}

func NewService(pool *db.Pool, clk clock.Clock, types *roomtypes.Store, publicBaseURL string) *Service {
	return &Service{pool: pool, q: dbgen.New(pool), clock: clk, types: types, publicBaseURL: strings.TrimRight(publicBaseURL, "/")}
}

type ListFilter struct {
	Type        *string
	CapacityMin *int
	IsActive    *bool
	Query       string
	Page        int
	PerPage     int
}

type CreateInput struct {
	OrgUnitID  uuid.UUID
	Code       string
	Name       string
	Type       string
	Capacity   int
	Facilities []string
	OpenTime   string // optional room-specific hours "HH:MM" (BR-29); both or neither
	CloseTime  string
}

// UpdateInput holds only the fields the caller sent; nil means "leave unchanged".
// The code is immutable: schedules and reports refer to rooms by what people read on the door.
type UpdateInput struct {
	Name       *string
	Type       *string
	Capacity   *int
	Facilities *[]string
	IsActive   *bool
	// OpenTime and CloseTime change the room's own hours; ClearOperatingHours puts it back on the type's
	// default hours (BR-29).
	OpenTime            *string
	CloseTime           *string
	ClearOperatingHours bool
}

func (in UpdateInput) empty() bool {
	return in.Name == nil && in.Type == nil && in.Capacity == nil && in.Facilities == nil && in.IsActive == nil &&
		in.OpenTime == nil && in.CloseTime == nil && !in.ClearOperatingHours
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Room, error) {
	row, err := s.q.GetRoom(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Room{}, httpx.ErrNotFound
	}
	if err != nil {
		return Room{}, fmt.Errorf("getting room: %w", err)
	}
	return FromRow(Row(row)), nil
}

func (s *Service) List(ctx context.Context, f ListFilter) ([]Room, int64, error) {
	var typ dbgen.NullRoomType
	if f.Type != nil {
		typ = dbgen.NullRoomType{RoomType: dbgen.RoomType(*f.Type), Valid: true}
	}
	var capMin pgtype.Int4
	if f.CapacityMin != nil {
		capMin = pgtype.Int4{Int32: int32(*f.CapacityMin), Valid: true}
	}
	var active pgtype.Bool
	if f.IsActive != nil {
		active = pgtype.Bool{Bool: *f.IsActive, Valid: true}
	}
	var q pgtype.Text
	if term := strings.TrimSpace(f.Query); term != "" {
		q = pgtype.Text{String: escapeLike(term), Valid: true}
	}

	total, err := s.q.CountRooms(ctx, dbgen.CountRoomsParams{Type: typ, CapacityMin: capMin, IsActive: active, Q: q})
	if err != nil {
		return nil, 0, fmt.Errorf("counting rooms: %w", err)
	}
	rows, err := s.q.ListRooms(ctx, dbgen.ListRoomsParams{
		Type: typ, CapacityMin: capMin, IsActive: active, Q: q,
		PageLimit:  int32(f.PerPage),
		PageOffset: int32((f.Page - 1) * f.PerPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("listing rooms: %w", err)
	}

	out := make([]Room, 0, len(rows))
	for _, r := range rows {
		out = append(out, FromRow(Row(r)))
	}
	return out, total, nil
}

// Create adds a room. Who may do so depends on the room's type: its approvers and administrators (BR-10).
func (s *Service) Create(ctx context.Context, actor httpx.Principal, in CreateInput) (Room, error) {
	in.Code = strings.ToUpper(strings.TrimSpace(in.Code))
	in.Name = strings.TrimSpace(in.Name)

	var problems []httpx.ErrorDetail
	if in.OrgUnitID == uuid.Nil {
		problems = append(problems, httpx.ErrorDetail{Field: "org_unit_id", Message: "org_unit_id wajib diisi."})
	}
	problems = append(problems, validateCode(in.Code)...)
	problems = append(problems, validateName(in.Name)...)
	problems = append(problems, validateType(in.Type)...)
	problems = append(problems, validateCapacity(in.Capacity)...)
	facilities, facilityProblems := normalizeFacilities(in.Facilities)
	problems = append(problems, facilityProblems...)
	open, closing, hoursProblems := parseOwnHours(in.OpenTime, in.CloseTime)
	problems = append(problems, hoursProblems...)
	if len(problems) > 0 {
		return Room{}, httpx.ValidationError(problems...)
	}
	if err := s.types.RequireRoomManager(ctx, actor, in.Type); err != nil {
		return Room{}, err
	}

	token, err := NewQRToken()
	if err != nil {
		return Room{}, err
	}

	var created Room
	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		now := s.clock.Now()
		row, err := s.q.WithTx(tx).CreateRoom(ctx, dbgen.CreateRoomParams{
			OrgUnitID:   in.OrgUnitID,
			Code:        in.Code,
			Name:        in.Name,
			Type:        dbgen.RoomType(in.Type),
			Capacity:    int32(in.Capacity),
			Facilities:  facilities,
			QrToken:     token,
			QrRotatedAt: now,
			OpenTime:    open,
			CloseTime:   closing,
		})
		if err != nil {
			return err
		}
		created = FromRow(Row(row))
		return audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionRoomCreated, EntityType: audit.EntityRoom,
			EntityID: created.ID, After: created, At: now,
		})
	})
	if err != nil {
		if d, ok := constraintDetail(err); ok {
			return Room{}, httpx.ValidationError(d)
		}
		return Room{}, fmt.Errorf("creating room: %w", err)
	}
	return created, nil
}

func (s *Service) Update(ctx context.Context, actor httpx.Principal, id uuid.UUID, in UpdateInput) (Room, error) {
	if in.empty() {
		return Room{}, httpx.ValidationError(httpx.ErrorDetail{Message: "Tidak ada field yang diubah."})
	}

	var problems []httpx.ErrorDetail
	params := dbgen.UpdateRoomParams{ID: id, UpdatedAt: s.clock.Now()}

	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		problems = append(problems, validateName(name)...)
		params.Name = pgtype.Text{String: name, Valid: true}
	}
	if in.Type != nil {
		problems = append(problems, validateType(*in.Type)...)
		params.Type = dbgen.NullRoomType{RoomType: dbgen.RoomType(*in.Type), Valid: true}
	}
	if in.Capacity != nil {
		problems = append(problems, validateCapacity(*in.Capacity)...)
		params.Capacity = pgtype.Int4{Int32: int32(*in.Capacity), Valid: true}
	}
	if in.Facilities != nil {
		facilities, p := normalizeFacilities(*in.Facilities)
		problems = append(problems, p...)
		params.Facilities = facilities
	}
	if in.IsActive != nil {
		params.IsActive = pgtype.Bool{Bool: *in.IsActive, Valid: true}
	}
	params.ClearOperatingHours = in.ClearOperatingHours
	if in.ClearOperatingHours && (in.OpenTime != nil || in.CloseTime != nil) {
		problems = append(problems, httpx.ErrorDetail{Field: "open_time", Message: "clear_operating_hours tidak dapat dikirim bersama open_time atau close_time."})
	}
	if len(problems) > 0 {
		return Room{}, httpx.ValidationError(problems...)
	}

	// Same lock as bookings and schedules take, so a change cannot interleave with them.
	var updated Room
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		locked, err := q.LockRoom(ctx, id)
		if err != nil {
			return err
		}
		before := FromRow(Row(locked))

		// Hours are judged as they will be after the change, so sending only one of the two is fine when
		// the room already has the other.
		if in.OpenTime != nil || in.CloseTime != nil {
			open, closing, problems := mergeOwnHours(locked.OpenTime, locked.CloseTime, in.OpenTime, in.CloseTime)
			if len(problems) > 0 {
				return httpx.ValidationError(problems...)
			}
			params.OpenTime, params.CloseTime = open, closing
		}

		// The actor must manage the type the room has now, and the type it is moving to.
		if err := s.types.RequireRoomManager(ctx, actor, before.Type); err != nil {
			return err
		}
		if in.Type != nil && *in.Type != before.Type {
			if err := s.types.RequireRoomManager(ctx, actor, *in.Type); err != nil {
				return err
			}
		}

		row, err := q.UpdateRoom(ctx, params)
		if err != nil {
			return err
		}
		updated = FromRow(Row(row))
		return audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionRoomUpdated, EntityType: audit.EntityRoom,
			EntityID: id, Before: before, After: updated, At: params.UpdatedAt,
		})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Room{}, httpx.ErrNotFound
	}
	if err != nil {
		if errors.Is(err, httpx.ErrForbidden) {
			return Room{}, err
		}
		if d, ok := constraintDetail(err); ok {
			return Room{}, httpx.ValidationError(d)
		}
		return Room{}, fmt.Errorf("updating room: %w", err)
	}
	return updated, nil
}

// Delete soft-deletes a room (BR-07). Schedules and past bookings stay as history.
func (s *Service) Delete(ctx context.Context, actor httpx.Principal, id uuid.UUID) error {
	now := s.clock.Now()
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		// Locking first means a booking being created right now either commits before this check
		// (and blocks the delete) or waits for the delete and is refused as ROOM_INACTIVE/not found.
		locked, err := q.LockRoom(ctx, id)
		if err != nil {
			return err
		}
		before := FromRow(Row(locked))
		if err := s.types.RequireRoomManager(ctx, actor, before.Type); err != nil {
			return err
		}

		live, err := q.CountLiveBookingsForRoom(ctx, dbgen.CountLiveBookingsForRoomParams{RoomID: id, EndAt: now})
		if err != nil {
			return fmt.Errorf("counting live bookings: %w", err)
		}
		if live > 0 {
			return httpx.ErrRoomHasActiveBookings
		}
		if err := q.SoftDeleteRoom(ctx, dbgen.SoftDeleteRoomParams{ID: id, DeletedAt: pgtype.Timestamptz{Time: now, Valid: true}}); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionRoomDeleted, EntityType: audit.EntityRoom,
			EntityID: id, Before: before, At: now,
		})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.ErrNotFound
	}
	if err != nil {
		if errors.Is(err, httpx.ErrForbidden) || errors.Is(err, httpx.ErrRoomHasActiveBookings) {
			return err
		}
		return fmt.Errorf("deleting room: %w", err)
	}
	return nil
}

func validateCode(v string) []httpx.ErrorDetail {
	if !codePattern.MatchString(v) {
		return []httpx.ErrorDetail{{Field: "code", Message: "Kode harus 1-32 karakter (huruf, angka, titik, garis) dan diawali huruf atau angka."}}
	}
	return nil
}

func validateName(v string) []httpx.ErrorDetail {
	if n := utf8.RuneCountInString(v); n < 1 || n > 120 {
		return []httpx.ErrorDetail{{Field: "name", Message: "Nama wajib diisi (maksimal 120 karakter)."}}
	}
	return nil
}

// validateType accepts any type the database knows. Which types exist is data (room_type_policies),
// not something this package lists by hand.
func validateType(v string) []httpx.ErrorDetail {
	if !dbgen.RoomType(v).Valid() {
		return []httpx.ErrorDetail{{Field: "type", Message: "Tipe ruang tidak dikenal."}}
	}
	return nil
}

func validateCapacity(v int) []httpx.ErrorDetail {
	if v < 1 || v > maxCapacity {
		return []httpx.ErrorDetail{{Field: "capacity", Message: fmt.Sprintf("Kapasitas harus antara 1 dan %d.", maxCapacity)}}
	}
	return nil
}

// normalizeFacilities trims entries, drops duplicates (case-insensitively) and always returns a
// non-nil slice, so "[]" clears the column instead of being read as "not provided".
func normalizeFacilities(in []string) ([]string, []httpx.ErrorDetail) {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, f := range in {
		f = strings.TrimSpace(f)
		if f == "" || utf8.RuneCountInString(f) > maxFacilityLength {
			return nil, []httpx.ErrorDetail{{Field: "facilities", Message: fmt.Sprintf("Setiap fasilitas harus 1-%d karakter.", maxFacilityLength)}}
		}
		if key := strings.ToLower(f); !seen[key] {
			seen[key] = true
			out = append(out, f)
		}
	}
	if len(out) > maxFacilities {
		return nil, []httpx.ErrorDetail{{Field: "facilities", Message: fmt.Sprintf("Maksimal %d fasilitas.", maxFacilities)}}
	}
	return out, nil
}

// constraintDetail maps unique/foreign-key violations to a field error instead of a 500.
func constraintDetail(err error) (httpx.ErrorDetail, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return httpx.ErrorDetail{}, false
	}
	switch {
	case pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == "rooms_org_unit_code_active_key":
		return httpx.ErrorDetail{Field: "code", Message: "Kode ruang sudah digunakan."}, true
	case pgErr.Code == pgForeignKeyViolation && pgErr.ConstraintName == "rooms_org_unit_id_fkey":
		return httpx.ErrorDetail{Field: "org_unit_id", Message: "Unit organisasi tidak ditemukan."}, true
	}
	return httpx.ErrorDetail{}, false
}

// escapeLike neutralises LIKE wildcards so user input is matched literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// parseOwnHours reads the optional room-specific hours of a new room (BR-29): both or neither.
func parseOwnHours(open, closing string) (pgtype.Time, pgtype.Time, []httpx.ErrorDetail) {
	if open == "" && closing == "" {
		return pgtype.Time{}, pgtype.Time{}, nil
	}
	return mergeOwnHours(pgtype.Time{}, pgtype.Time{}, &open, &closing)
}

// mergeOwnHours applies a requested change to a room's current hours and checks the result: both set or
// both empty, "HH:MM", on the 30-minute grid, open before close. Room hours are not bounded by the type's
// default window; a room may open longer than its type's default (BR-29).
func mergeOwnHours(curOpen, curClose pgtype.Time, newOpen, newClose *string) (pgtype.Time, pgtype.Time, []httpx.ErrorDetail) {
	open, hasOpen := roomtypes.MinutesOf(curOpen)
	closing, hasClose := roomtypes.MinutesOf(curClose)
	var problems []httpx.ErrorDetail
	if newOpen != nil {
		m, ok := roomtypes.ParseClock(*newOpen)
		if !ok {
			problems = append(problems, httpx.ErrorDetail{Field: "open_time", Message: "open_time harus berformat HH:MM."})
		}
		open, hasOpen = m, ok
	}
	if newClose != nil {
		m, ok := roomtypes.ParseClock(*newClose)
		if !ok {
			problems = append(problems, httpx.ErrorDetail{Field: "close_time", Message: "close_time harus berformat HH:MM."})
		}
		closing, hasClose = m, ok
	}
	if len(problems) > 0 {
		return pgtype.Time{}, pgtype.Time{}, problems
	}
	if hasOpen != hasClose {
		return pgtype.Time{}, pgtype.Time{}, []httpx.ErrorDetail{{Field: "open_time", Message: "open_time dan close_time harus diisi bersamaan."}}
	}
	if !roomtypes.ValidHours(open, closing) {
		return pgtype.Time{}, pgtype.Time{}, []httpx.ErrorDetail{{Field: "open_time", Message: fmt.Sprintf("Jam buka harus sebelum jam tutup dan berkelipatan %d menit.", roomtypes.MinutesPerSlot)}}
	}
	return roomtypes.PGTime(open), roomtypes.PGTime(closing), nil
}
