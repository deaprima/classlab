package imports

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"classlab/api/internal/audit"
	"classlab/api/internal/httpx"
	"classlab/api/internal/notifications"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	pgExclusionViolation = "23P01"

	// maxConflictNotes keeps one line's error list readable: the first few collisions, then a count.
	maxConflictNotes = 3
)

var scheduleColumns = []string{"room_code", "title", "day_of_week", "start_time", "end_time", "semester_start", "semester_end"}

// scheduleItem is one CSV line that passed every check, with the meetings it will create. Its exported
// fields are what plan_hash covers.
type scheduleItem struct {
	Line     int          `json:"line"`
	RoomID   uuid.UUID    `json:"room_id"`
	Title    string       `json:"title"`
	Windows  []occurrence `json:"windows"`
	roomCode string
}

// cancelItem is a booking the academic schedule displaces (BR-23 rule a).
type cancelItem struct {
	BookingID     uuid.UUID           `json:"booking_id"`
	Status        dbgen.BookingStatus `json:"status"`
	Start         time.Time           `json:"start"`
	End           time.Time           `json:"end"`
	ConflictsWith string              `json:"conflicts_with"`

	requesterID uuid.UUID
	roomCode    string
}

type schedulePlan struct {
	Rows    int
	Items   []scheduleItem
	Cancels []cancelItem
	Errors  []RowError
}

func (p schedulePlan) occurrences() int {
	n := 0
	for _, it := range p.Items {
		n += len(it.Windows)
	}
	return n
}

// CancelPreview is one booking the import would cancel, as shown in the preview.
type CancelPreview struct {
	BookingID     uuid.UUID `json:"booking_id"`
	RoomCode      string    `json:"room_code"`
	StartAt       time.Time `json:"start_at"`
	EndAt         time.Time `json:"end_at"`
	ConflictsWith string    `json:"conflicts_with"`
}

// SchedulePreview is what the preview step returns; nothing is stored (BR-23).
type SchedulePreview struct {
	Rows             int             `json:"rows"`
	Occurrences      int             `json:"occurrences"`
	Errors           []RowError      `json:"errors"`
	BookingsToCancel []CancelPreview `json:"bookings_to_cancel"`
	PlanHash         string          `json:"plan_hash"`
}

// ScheduleApplyResult is returned when an import is applied.
type ScheduleApplyResult struct {
	ImportID          uuid.UUID `json:"import_id"`
	Rows              int       `json:"rows"`
	Occurrences       int       `json:"occurrences"`
	BookingsCancelled int       `json:"bookings_cancelled"`
}

// PreviewSchedules parses and plans the file without writing anything.
func (s *Service) PreviewSchedules(ctx context.Context, actor httpx.Principal, file io.Reader) (SchedulePreview, error) {
	if !mayImport(actor) {
		return SchedulePreview{}, httpx.ErrForbidden
	}
	table, err := Parse(file, s.opts.Limits, scheduleColumns)
	if err != nil {
		return SchedulePreview{}, fileError(err)
	}
	importer, err := s.q.GetUserByID(ctx, actor.UserID)
	if err != nil {
		return SchedulePreview{}, fmt.Errorf("loading importer: %w", err)
	}
	plan, err := planSchedules(ctx, s.q, table, importer.OrgUnitID, s.clock.Now(), s.opts.MaxWeeks)
	if err != nil {
		return SchedulePreview{}, err
	}

	out := SchedulePreview{
		Rows: plan.Rows, Occurrences: plan.occurrences(), Errors: plan.Errors,
		BookingsToCancel: make([]CancelPreview, len(plan.Cancels)),
		PlanHash:         hashSchedulePlan(table.SHA256, plan),
	}
	if out.Errors == nil {
		out.Errors = []RowError{}
	}
	for i, c := range plan.Cancels {
		out.BookingsToCancel[i] = CancelPreview{
			BookingID: c.BookingID, RoomCode: c.roomCode, ConflictsWith: c.ConflictsWith,
			StartAt: clock.ToWIB(c.Start), EndAt: clock.ToWIB(c.End),
		}
	}
	return out, nil
}

// ApplySchedules writes the schedule in one transaction under the room locks, taken in ascending room id
// so two imports (or an import and a booking) cannot deadlock. planHash must be the one the preview
// returned for this same file; if anything changed in between nothing is written.
func (s *Service) ApplySchedules(ctx context.Context, actor httpx.Principal, fileName string, file io.Reader, planHash string) (ScheduleApplyResult, error) {
	if !mayImport(actor) {
		return ScheduleApplyResult{}, httpx.ErrForbidden
	}
	if planHash == "" {
		return ScheduleApplyResult{}, httpx.ValidationError(httpx.ErrorDetail{Field: "plan_hash", Message: "plan_hash dari pratinjau wajib dikirim."})
	}
	table, err := Parse(file, s.opts.Limits, scheduleColumns)
	if err != nil {
		return ScheduleApplyResult{}, fileError(err)
	}

	applied, err := s.q.ImportAlreadyApplied(ctx, dbgen.ImportAlreadyAppliedParams{Kind: dbgen.ImportKindSCHEDULES, FileSha256: table.SHA256})
	if err != nil {
		return ScheduleApplyResult{}, fmt.Errorf("checking earlier imports: %w", err)
	}
	if applied {
		return ScheduleApplyResult{}, errAlreadyApplied
	}

	importer, err := s.q.GetUserByID(ctx, actor.UserID)
	if err != nil {
		return ScheduleApplyResult{}, fmt.Errorf("loading importer: %w", err)
	}
	plan, err := planSchedules(ctx, s.q, table, importer.OrgUnitID, s.clock.Now(), s.opts.MaxWeeks)
	if err != nil {
		return ScheduleApplyResult{}, err
	}
	if len(plan.Errors) > 0 {
		return ScheduleApplyResult{}, invalidImport(plan.Errors)
	}
	if hashSchedulePlan(table.SHA256, plan) != planHash {
		return ScheduleApplyResult{}, errPlanChanged
	}

	var result ScheduleApplyResult
	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		// Lock every room of the plan, lowest id first. Bookings take the same lock before they write, so
		// from here until commit nothing can slip a booking into a slot this import is about to claim.
		for _, id := range roomIDs(plan) {
			if _, err := q.LockRoom(ctx, id); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return errPlanChanged // the room was deleted since the preview
				}
				return fmt.Errorf("locking room: %w", err)
			}
		}
		// Plan again under the locks: this is the plan that gets written.
		fresh, err := planSchedules(ctx, q, table, importer.OrgUnitID, now, s.opts.MaxWeeks)
		if err != nil {
			return err
		}
		if len(fresh.Errors) > 0 || hashSchedulePlan(table.SHA256, fresh) != planHash {
			return errPlanChanged
		}

		summary := map[string]any{"rows": fresh.Rows, "occurrences": fresh.occurrences(), "bookings_cancelled": len(fresh.Cancels)}
		raw, err := json.Marshal(summary)
		if err != nil {
			return fmt.Errorf("encoding import summary: %w", err)
		}
		importID, err := q.CreateImport(ctx, dbgen.CreateImportParams{
			Kind: dbgen.ImportKindSCHEDULES, UploadedBy: actor.UserID, FileName: optionalText(fileName),
			FileSha256: table.SHA256, Summary: raw, CreatedAt: now,
		})
		if err != nil {
			return err
		}

		// The displaced bookings go first: their slots are held by the booking exclusion constraint, not
		// the schedule one, but cancelling first keeps the order of events readable in the history.
		for _, c := range fresh.Cancels {
			if err := s.cancelForImport(ctx, q, actor, c, now); err != nil {
				return err
			}
		}
		for _, it := range fresh.Items {
			for _, w := range it.Windows {
				if err := q.CreateImportedSchedule(ctx, dbgen.CreateImportedScheduleParams{
					RoomID: it.RoomID, Title: it.Title, StartAt: w.Start, EndAt: w.End,
					CreatedBy: actor.UserID, ImportID: pgtype.UUID{Bytes: importID, Valid: true}, CreatedAt: now,
				}); err != nil {
					return fmt.Errorf("creating schedule for line %d: %w", it.Line, err)
				}
			}
		}

		if err := audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionSchedulesImported, EntityType: audit.EntityImport,
			EntityID: importID, After: summary, At: now,
		}); err != nil {
			return err
		}
		result = ScheduleApplyResult{ImportID: importID, Rows: fresh.Rows, Occurrences: fresh.occurrences(), BookingsCancelled: len(fresh.Cancels)}
		return nil
	})
	if err != nil {
		if isAlreadyAppliedRace(err) {
			return ScheduleApplyResult{}, errAlreadyApplied
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgExclusionViolation {
			// The constraint is the last line of defence; reaching it means a writer slipped past the lock.
			return ScheduleApplyResult{}, errPlanChanged
		}
		var domain *httpx.CustomDomainError
		if errors.As(err, &domain) {
			return ScheduleApplyResult{}, err
		}
		return ScheduleApplyResult{}, fmt.Errorf("applying schedule import: %w", err)
	}

	s.log.InfoContext(ctx, "schedule import applied", "import_id", result.ImportID.String(),
		"occurrences", result.Occurrences, "bookings_cancelled", result.BookingsCancelled)
	return result, nil
}

// cancelForImport cancels one booking, records who and why in its history and tells its owner; it runs
// inside the import's transaction, so none of the three can exist without the others.
func (s *Service) cancelForImport(ctx context.Context, q *dbgen.Queries, actor httpx.Principal, c cancelItem, now time.Time) error {
	reason := "Jadwal akademik: " + c.ConflictsWith
	n, err := q.CancelBookingForImport(ctx, dbgen.CancelBookingForImportParams{
		DecidedBy: pgtype.UUID{Bytes: actor.UserID, Valid: true}, Now: pgtype.Timestamptz{Time: now, Valid: true},
		Reason: pgtype.Text{String: reason, Valid: true}, ID: c.BookingID,
	})
	if err != nil {
		return fmt.Errorf("cancelling booking %s: %w", c.BookingID, err)
	}
	if n == 0 {
		return errPlanChanged // the booking left MENUNGGU/DISETUJUI since the plan was made
	}
	if err := q.CreateBookingEvent(ctx, dbgen.CreateBookingEventParams{
		BookingID: c.BookingID, ActorID: pgtype.UUID{Bytes: actor.UserID, Valid: true},
		FromStatus: dbgen.NullBookingStatus{BookingStatus: c.Status, Valid: true},
		ToStatus:   dbgen.BookingStatusDIBATALKAN, Reason: pgtype.Text{String: reason, Valid: true}, Kind: "STATUS", CreatedAt: now,
	}); err != nil {
		return fmt.Errorf("recording cancellation of booking %s: %w", c.BookingID, err)
	}
	// No violation points: the requester did nothing wrong (BR-23).
	return s.notifier.ToUser(ctx, q, c.requesterID, notifications.Message{
		Type: notifications.TypeBookingCancelled, Title: "Booking dibatalkan",
		Body: fmt.Sprintf("Booking %s pada %s-%s WIB dibatalkan karena jadwal akademik: %s.",
			c.roomCode, clock.ToWIB(c.Start).Format("02/01/2006 15:04"), clock.ToWIB(c.End).Format("15:04"), c.ConflictsWith),
		RefType: notifications.RefBooking, RefID: c.BookingID,
	}, now)
}

// RollbackResult is returned by a rollback.
type RollbackResult struct {
	ID                 uuid.UUID `json:"id"`
	Status             string    `json:"status"`
	DeletedOccurrences int64     `json:"deleted_occurrences"`
}

// Rollback removes the meetings of a schedule import that have not started yet. Bookings the import
// cancelled are not brought back; to replace a semester's schedule, roll back and import again (BR-23).
func (s *Service) Rollback(ctx context.Context, actor httpx.Principal, id uuid.UUID) (RollbackResult, error) {
	if !mayImport(actor) {
		return RollbackResult{}, httpx.ErrForbidden
	}

	var result RollbackResult
	err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		imp, err := q.GetImportForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if imp.Kind != dbgen.ImportKindSCHEDULES {
			return httpx.ValidationError(httpx.ErrorDetail{Field: "id", Message: "Hanya impor jadwal yang dapat di-rollback."})
		}
		if imp.Status != "COMMITTED" {
			return httpx.NewCustomError(http.StatusConflict, "INVALID_STATE", "Impor ini sudah di-rollback.", nil, httpx.ErrInvalidState)
		}

		deleted, err := q.DeleteUpcomingImportSchedules(ctx, dbgen.DeleteUpcomingImportSchedulesParams{ImportID: pgtype.UUID{Bytes: id, Valid: true}, StartAt: now})
		if err != nil {
			return fmt.Errorf("deleting upcoming schedules: %w", err)
		}
		if err := q.MarkImportRolledBack(ctx, dbgen.MarkImportRolledBackParams{
			ID: id, RolledBackAt: pgtype.Timestamptz{Time: now, Valid: true}, RolledBackBy: pgtype.UUID{Bytes: actor.UserID, Valid: true},
		}); err != nil {
			return fmt.Errorf("marking import rolled back: %w", err)
		}
		result = RollbackResult{ID: id, Status: "ROLLED_BACK", DeletedOccurrences: deleted}
		return audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionImportRolledBack, EntityType: audit.EntityImport, EntityID: id,
			Before: map[string]any{"status": "COMMITTED"}, After: map[string]any{"status": "ROLLED_BACK", "deleted_occurrences": deleted}, At: now,
		})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return RollbackResult{}, httpx.ErrNotFound
	}
	if err != nil {
		var domain *httpx.CustomDomainError
		if errors.As(err, &domain) {
			return RollbackResult{}, err
		}
		return RollbackResult{}, fmt.Errorf("rolling back import: %w", err)
	}
	return result, nil
}

// roomRef is a room a CSV line refers to.
type roomRef struct {
	id     uuid.UUID
	active bool
}

// planSchedules checks every line, expands it into meetings and measures it against what already
// occupies the rooms. It never writes, so preview and apply share it (BR-23).
func planSchedules(ctx context.Context, q *dbgen.Queries, t *Table, orgUnitID uuid.UUID, now time.Time, maxWeeks int) (schedulePlan, error) {
	plan := schedulePlan{Rows: len(t.Rows)}
	addError := func(line int, col, msg string) {
		plan.Errors = append(plan.Errors, RowError{Line: line, Column: col, Message: msg})
	}

	// 1. Each line on its own: columns, dates and times.
	type parsed struct {
		row  scheduleRow
		bad  bool
		room roomRef
	}
	lines := make([]*parsed, 0, len(t.Rows))
	codes := map[string]bool{}
	for _, r := range t.Rows {
		row, problems := parseScheduleRow(r, maxWeeks)
		p := &parsed{row: row, bad: len(problems) > 0}
		plan.Errors = append(plan.Errors, problems...)
		if !p.bad {
			codes[row.RoomCode] = true
		}
		lines = append(lines, p)
	}

	// 2. Rooms.
	codeList := make([]string, 0, len(codes))
	for c := range codes {
		codeList = append(codeList, c)
	}
	slices.Sort(codeList)
	found, err := q.ListRoomsByCodes(ctx, dbgen.ListRoomsByCodesParams{OrgUnitID: orgUnitID, Codes: codeList})
	if err != nil {
		return schedulePlan{}, fmt.Errorf("looking up rooms: %w", err)
	}
	byCode := make(map[string]roomRef, len(found))
	for _, r := range found {
		byCode[r.Code] = roomRef{id: r.ID, active: r.IsActive}
	}
	for _, p := range lines {
		if p.bad {
			continue
		}
		room, ok := byCode[p.row.RoomCode]
		switch {
		case !ok:
			p.bad = true
			addError(p.row.Line, "room_code", fmt.Sprintf("Kode ruang %q tidak dikenal.", p.row.RoomCode))
		case !room.active:
			p.bad = true
			addError(p.row.Line, "room_code", fmt.Sprintf("Ruang %s tidak aktif.", p.row.RoomCode))
		default:
			p.room = room
		}
	}

	// 3. Meetings per line, and collisions between lines of the same file (rule c).
	meetings := make(map[*parsed][]occurrence, len(lines))
	type entry struct {
		line       int
		title      string
		start, end time.Time
	}
	perRoom := map[uuid.UUID][]entry{}
	for _, p := range lines {
		if p.bad {
			continue
		}
		occ := p.row.expand(now)
		meetings[p] = occ
		for _, o := range occ {
			perRoom[p.room.id] = append(perRoom[p.room.id], entry{p.row.Line, p.row.Title, o.Start, o.End})
		}
	}
	reportedInFile := map[int]bool{}
	byLine := make(map[int]*parsed, len(lines))
	for _, p := range lines {
		byLine[p.row.Line] = p
	}
	for _, entries := range perRoom {
		sort.SliceStable(entries, func(i, j int) bool {
			if !entries[i].start.Equal(entries[j].start) {
				return entries[i].start.Before(entries[j].start)
			}
			return entries[i].line < entries[j].line
		})
		var maxEnd time.Time
		var maxLine int
		var maxTitle string
		for i, e := range entries {
			if i > 0 && e.start.Before(maxEnd) && e.line != maxLine && !reportedInFile[e.line] {
				reportedInFile[e.line] = true
				byLine[e.line].bad = true
				addError(e.line, "start_time", fmt.Sprintf("Bentrok dengan baris %d (%s) pada %s.", maxLine, maxTitle, clock.ToWIB(e.start).Format("02/01/2006 15:04")))
			}
			if e.end.After(maxEnd) {
				maxEnd, maxLine, maxTitle = e.end, e.line, e.title
			}
		}
	}

	// 4. What is already in the rooms: schedules and live bookings in the span the file covers.
	var ids []uuid.UUID
	var from, to time.Time
	seenRoom := map[uuid.UUID]bool{}
	for p, occ := range meetings {
		if p.bad || len(occ) == 0 {
			continue
		}
		if !seenRoom[p.room.id] {
			seenRoom[p.room.id] = true
			ids = append(ids, p.room.id)
		}
		if from.IsZero() || occ[0].Start.Before(from) {
			from = occ[0].Start
		}
		if last := occ[len(occ)-1].End; to.IsZero() || last.After(to) {
			to = last
		}
	}
	var existingSchedules []dbgen.ListSchedulesInRangeRow
	var existingBookings []dbgen.ListLiveBookingsInRangeRow
	if len(ids) > 0 {
		if existingSchedules, err = q.ListSchedulesInRange(ctx, dbgen.ListSchedulesInRangeParams{RoomIds: ids, RangeStart: from, RangeEnd: to}); err != nil {
			return schedulePlan{}, fmt.Errorf("loading schedules: %w", err)
		}
		if existingBookings, err = q.ListLiveBookingsInRange(ctx, dbgen.ListLiveBookingsInRangeParams{RoomIds: ids, RangeStart: from, RangeEnd: to}); err != nil {
			return schedulePlan{}, fmt.Errorf("loading bookings: %w", err)
		}
	}
	schedsOf := map[uuid.UUID][]dbgen.ListSchedulesInRangeRow{}
	for _, s := range existingSchedules {
		schedsOf[s.RoomID] = append(schedsOf[s.RoomID], s)
	}
	booksOf := map[uuid.UUID][]dbgen.ListLiveBookingsInRangeRow{}
	for _, b := range existingBookings {
		booksOf[b.RoomID] = append(booksOf[b.RoomID], b)
	}

	// 5. The rules of BR-23, line by line.
	cancelSeen := map[uuid.UUID]bool{}
	var cancels []cancelItem
	for _, p := range lines {
		if p.bad {
			continue
		}
		var (
			academic, manual, running conflictNote
			lineCancels               []cancelItem
			lineCancelSeen            = map[uuid.UUID]bool{}
			scheds                    = schedsOf[p.room.id]
			books                     = booksOf[p.room.id]
		)
		for _, o := range meetings[p] {
			for _, sc := range overlapping(scheds, func(s dbgen.ListSchedulesInRangeRow) (time.Time, time.Time) { return s.StartAt, s.EndAt }, o) {
				note := &academic
				if sc.Source == dbgen.ScheduleSourceMANUAL {
					note = &manual
				}
				note.add(o.Start, sc.Title)
			}
			for _, b := range overlapping(books, func(b dbgen.ListLiveBookingsInRangeRow) (time.Time, time.Time) { return b.StartAt, b.EndAt }, o) {
				if b.Status == dbgen.BookingStatusBERLANGSUNG {
					running.add(o.Start, b.Purpose)
					continue
				}
				if lineCancelSeen[b.ID] || cancelSeen[b.ID] {
					continue
				}
				lineCancelSeen[b.ID] = true
				lineCancels = append(lineCancels, cancelItem{
					BookingID: b.ID, Status: b.Status, Start: b.StartAt.UTC(), End: b.EndAt.UTC(), ConflictsWith: p.row.Title,
					requesterID: b.RequesterID, roomCode: p.row.RoomCode,
				})
			}
		}
		if academic.n > 0 {
			addError(p.row.Line, "start_time", academic.message("Bentrok dengan jadwal akademik lain"))
		}
		if manual.n > 0 {
			addError(p.row.Line, "start_time", manual.message("Bentrok dengan jadwal manual (selesaikan dulu secara manual)"))
		}
		if running.n > 0 {
			addError(p.row.Line, "start_time", running.message("Bentrok dengan booking yang sedang berlangsung"))
		}
		if academic.n == 0 && manual.n == 0 && running.n == 0 {
			for _, c := range lineCancels {
				cancelSeen[c.BookingID] = true
			}
			cancels = append(cancels, lineCancels...)
			plan.Items = append(plan.Items, scheduleItem{
				Line: p.row.Line, RoomID: p.room.id, Title: p.row.Title, Windows: meetings[p], roomCode: p.row.RoomCode,
			})
		}
	}

	sort.SliceStable(cancels, func(i, j int) bool {
		if cancels[i].roomCode != cancels[j].roomCode {
			return cancels[i].roomCode < cancels[j].roomCode
		}
		if !cancels[i].Start.Equal(cancels[j].Start) {
			return cancels[i].Start.Before(cancels[j].Start)
		}
		return cancels[i].BookingID.String() < cancels[j].BookingID.String()
	})
	plan.Cancels = cancels
	sort.SliceStable(plan.Errors, func(i, j int) bool { return plan.Errors[i].Line < plan.Errors[j].Line })
	return plan, nil
}

// conflictNote collects the collisions of one kind on one line into a single readable error.
type conflictNote struct {
	n       int
	example string
}

func (c *conflictNote) add(start time.Time, title string) {
	c.n++
	if c.n <= maxConflictNotes {
		sep := ""
		if c.example != "" {
			sep = "; "
		}
		c.example += fmt.Sprintf("%s%s (%q)", sep, clock.ToWIB(start).Format("02/01/2006 15:04"), title)
	}
}

func (c conflictNote) message(prefix string) string {
	msg := prefix + ": " + c.example
	if c.n > maxConflictNotes {
		msg += fmt.Sprintf(" dan %d lainnya", c.n-maxConflictNotes)
	}
	return msg + "."
}

// overlapping returns the items that intersect o. items are ordered by start and, within one room, do not
// overlap each other (the exclusion constraints guarantee that), so their ends are ordered too and a
// binary search finds the first candidate.
func overlapping[T any](items []T, span func(T) (time.Time, time.Time), o occurrence) []T {
	i := sort.Search(len(items), func(i int) bool { _, end := span(items[i]); return end.After(o.Start) })
	var out []T
	for ; i < len(items); i++ {
		start, _ := span(items[i])
		if !start.Before(o.End) {
			break
		}
		out = append(out, items[i])
	}
	return out
}

// roomIDs lists the distinct rooms of a plan in ascending id order, the order locks are taken in.
func roomIDs(p schedulePlan) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	var ids []uuid.UUID
	for _, it := range p.Items {
		if !seen[it.RoomID] {
			seen[it.RoomID] = true
			ids = append(ids, it.RoomID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return strings.Compare(ids[i].String(), ids[j].String()) < 0 })
	return ids
}

// hashSchedulePlan fingerprints the plan so apply can tell the database moved since the preview.
func hashSchedulePlan(fileSHA string, p schedulePlan) string {
	h := sha256.New()
	if err := json.NewEncoder(h).Encode(struct {
		File    string         `json:"file"`
		Items   []scheduleItem `json:"items"`
		Cancels []cancelItem   `json:"cancels"`
	}{fileSHA, p.Items, p.Cancels}); err != nil {
		panic(fmt.Sprintf("hashing schedule plan: %v", err))
	}
	return hex.EncodeToString(h.Sum(nil))
}
