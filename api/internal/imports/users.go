package imports

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	"classlab/api/internal/audit"
	"classlab/api/internal/httpx"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/users"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	actionCreate = "CREATE"
	actionUpdate = "UPDATE"

	maxCourseFieldRunes = 120
)

var (
	studentColumns  = []string{"identifier", "name"}
	semesterPattern = regexp.MustCompile(`^[0-9]{4}-(GANJIL|GENAP)$`)
)

// userItem is one planned change. Its exported fields are exactly what plan_hash covers, so the apply
// step can tell whether the database moved since the preview.
type userItem struct {
	Line       int    `json:"line"`
	Action     string `json:"action"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	CourseCode string `json:"course_code"`
	CourseName string `json:"course_name"`
	ClassName  string `json:"class_name"`

	existingID uuid.UUID
}

func (i userItem) hasCourse() bool {
	return i.CourseCode != "" || i.CourseName != "" || i.ClassName != ""
}

// UserPreview is what the preview step returns; nothing is stored (BR-24).
type UserPreview struct {
	Rows     int        `json:"rows"`
	ToCreate int        `json:"to_create"`
	ToUpdate int        `json:"to_update"`
	Errors   []RowError `json:"errors"`
	PlanHash string     `json:"plan_hash"`
}

// Credential is a new account's temporary password. It exists only in the apply response.
type Credential struct {
	Identifier        string `json:"identifier"`
	TemporaryPassword string `json:"temporary_password"`
}

// UserApplyResult is returned once, with Cache-Control: no-store (BR-24).
type UserApplyResult struct {
	ImportID    uuid.UUID    `json:"import_id"`
	Created     int          `json:"created"`
	Updated     int          `json:"updated"`
	Credentials []Credential `json:"credentials"`
}

// PreviewUsers parses and plans the file without writing anything.
func (s *Service) PreviewUsers(ctx context.Context, actor httpx.Principal, file io.Reader, semester string) (UserPreview, error) {
	if !mayImport(actor) {
		return UserPreview{}, httpx.ErrForbidden
	}
	if err := validateSemester(semester); err != nil {
		return UserPreview{}, err
	}
	table, err := Parse(file, s.opts.Limits, studentColumns)
	if err != nil {
		return UserPreview{}, fileError(err)
	}
	items, problems, err := planUsers(ctx, s.q, table)
	if err != nil {
		return UserPreview{}, err
	}

	p := UserPreview{Rows: len(table.Rows), Errors: problems, PlanHash: hashPlan(semester, table.SHA256, items)}
	if p.Errors == nil {
		p.Errors = []RowError{}
	}
	for _, it := range items {
		if it.Action == actionCreate {
			p.ToCreate++
		} else {
			p.ToUpdate++
		}
	}
	return p, nil
}

// ApplyUsers creates and updates the accounts in one transaction. planHash must be the one the preview
// returned for this same file and semester; if the database changed in between the plan differs and
// nothing is written. Temporary passwords are generated here, returned once and stored only as hashes.
func (s *Service) ApplyUsers(ctx context.Context, actor httpx.Principal, fileName string, file io.Reader, semester, planHash string) (UserApplyResult, error) {
	if !mayImport(actor) {
		return UserApplyResult{}, httpx.ErrForbidden
	}
	if err := validateSemester(semester); err != nil {
		return UserApplyResult{}, err
	}
	if planHash == "" {
		return UserApplyResult{}, httpx.ValidationError(httpx.ErrorDetail{Field: "plan_hash", Message: "plan_hash dari pratinjau wajib dikirim."})
	}
	table, err := Parse(file, s.opts.Limits, studentColumns)
	if err != nil {
		return UserApplyResult{}, fileError(err)
	}

	applied, err := s.q.ImportAlreadyApplied(ctx, dbgen.ImportAlreadyAppliedParams{Kind: dbgen.ImportKindUSERS, FileSha256: table.SHA256})
	if err != nil {
		return UserApplyResult{}, fmt.Errorf("checking earlier imports: %w", err)
	}
	if applied {
		return UserApplyResult{}, errAlreadyApplied
	}

	// Plan outside the transaction first: hashing passwords is slow and must not hold the import lock.
	items, problems, err := planUsers(ctx, s.q, table)
	if err != nil {
		return UserApplyResult{}, err
	}
	if len(problems) > 0 {
		return UserApplyResult{}, invalidImport(problems)
	}
	if hashPlan(semester, table.SHA256, items) != planHash {
		return UserApplyResult{}, errPlanChanged
	}

	creds, hashes, err := s.newCredentials(items)
	if err != nil {
		return UserApplyResult{}, err
	}

	boss, err := s.q.GetUserByID(ctx, actor.UserID)
	if err != nil {
		return UserApplyResult{}, fmt.Errorf("loading importer: %w", err)
	}

	var result UserApplyResult
	err = s.pool.WithTx(ctx, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		now := s.clock.Now()

		if err := q.LockUserImport(ctx); err != nil {
			return fmt.Errorf("locking user import: %w", err)
		}
		// Plan again under the lock: another import or edit may have landed since the check above.
		fresh, problems, err := planUsers(ctx, q, table)
		if err != nil {
			return err
		}
		if len(problems) > 0 || hashPlan(semester, table.SHA256, fresh) != planHash {
			return errPlanChanged
		}

		created, updated := 0, 0
		for _, it := range fresh {
			if it.Action == actionCreate {
				created++
			} else {
				updated++
			}
		}
		summary, err := json.Marshal(map[string]any{"rows": len(table.Rows), "created": created, "updated": updated, "semester": semester})
		if err != nil {
			return fmt.Errorf("encoding import summary: %w", err)
		}
		importID, err := q.CreateImport(ctx, dbgen.CreateImportParams{
			Kind: dbgen.ImportKindUSERS, UploadedBy: actor.UserID, FileName: optionalText(fileName),
			FileSha256: table.SHA256, Semester: optionalText(semester), Summary: summary, CreatedAt: now,
		})
		if err != nil {
			return err
		}

		for _, it := range fresh {
			userID := it.existingID
			switch it.Action {
			case actionCreate:
				row, err := q.CreateUser(ctx, dbgen.CreateUserParams{
					OrgUnitID: boss.OrgUnitID, Identifier: it.Identifier, Name: it.Name, Email: optionalText(it.Email),
					PasswordHash: hashes[it.Identifier], Role: dbgen.UserRoleMAHASISWA, MustChangePassword: true, CreatedAt: now,
				})
				if err != nil {
					return fmt.Errorf("creating account at line %d: %w", it.Line, err)
				}
				userID = row.ID
			case actionUpdate:
				if err := q.UpdateImportedStudent(ctx, dbgen.UpdateImportedStudentParams{
					ID: userID, Name: it.Name, Email: optionalText(it.Email), UpdatedAt: now,
				}); err != nil {
					return fmt.Errorf("updating account at line %d: %w", it.Line, err)
				}
			}
			if it.hasCourse() {
				if err := q.CreateUserCourse(ctx, dbgen.CreateUserCourseParams{
					UserID: userID, ImportID: importID, Semester: semester,
					CourseCode: optionalText(it.CourseCode), CourseName: optionalText(it.CourseName),
					ClassName: optionalText(it.ClassName), CreatedAt: now,
				}); err != nil {
					return fmt.Errorf("recording course at line %d: %w", it.Line, err)
				}
			}
		}

		// The summary holds counts and the semester only; passwords never reach the audit log.
		if err := audit.Record(ctx, tx, audit.Entry{
			Actor: &actor, Action: audit.ActionUsersImported, EntityType: audit.EntityImport,
			EntityID: importID, After: map[string]any{"rows": len(table.Rows), "created": created, "updated": updated, "semester": semester},
			At: now,
		}); err != nil {
			return err
		}

		result = UserApplyResult{ImportID: importID, Created: created, Updated: updated, Credentials: []Credential{}}
		for _, it := range fresh {
			if it.Action == actionCreate {
				result.Credentials = append(result.Credentials, Credential{Identifier: it.Identifier, TemporaryPassword: creds[it.Identifier]})
			}
		}
		return nil
	})
	if err != nil {
		if isAlreadyAppliedRace(err) {
			return UserApplyResult{}, errAlreadyApplied
		}
		var domain *httpx.CustomDomainError
		if errors.As(err, &domain) {
			return UserApplyResult{}, err
		}
		return UserApplyResult{}, fmt.Errorf("applying user import: %w", err)
	}

	// Counts only: neither the file content nor a credential is ever logged.
	s.log.InfoContext(ctx, "user import applied", "import_id", result.ImportID.String(), "created", result.Created, "updated", result.Updated)
	return result, nil
}

func validateSemester(v string) error {
	if !semesterPattern.MatchString(v) {
		return httpx.ValidationError(httpx.ErrorDetail{Field: "semester", Message: "semester harus berformat TAHUN-GANJIL atau TAHUN-GENAP, mis. 2026-GANJIL."})
	}
	return nil
}

// planUsers checks every row and decides create or update, reading existing accounts in two queries for
// the whole file. It never writes, so preview and apply share it.
func planUsers(ctx context.Context, q *dbgen.Queries, t *Table) ([]userItem, []RowError, error) {
	identifiers := make([]string, 0, len(t.Rows))
	emails := make([]string, 0, len(t.Rows))
	for _, r := range t.Rows {
		identifiers = append(identifiers, r.Cells["identifier"])
		if e := strings.ToLower(r.Cells["email"]); e != "" {
			emails = append(emails, e)
		}
	}

	existing, err := q.ListUsersByIdentifiers(ctx, identifiers)
	if err != nil {
		return nil, nil, fmt.Errorf("looking up accounts: %w", err)
	}
	byIdentifier := make(map[string]dbgen.ListUsersByIdentifiersRow, len(existing))
	for _, u := range existing {
		byIdentifier[u.Identifier] = u
	}
	owners, err := q.ListUsersByEmails(ctx, emails)
	if err != nil {
		return nil, nil, fmt.Errorf("looking up emails: %w", err)
	}
	emailOwner := make(map[string]string, len(owners))
	for _, o := range owners {
		emailOwner[o.Email] = o.Identifier
	}

	var (
		items     []userItem
		problems  []RowError
		seenID    = make(map[string]int, len(t.Rows))
		seenEmail = make(map[string]int, len(t.Rows))
		bad       = func(line int, col, msg string) {
			problems = append(problems, RowError{Line: line, Column: col, Message: msg})
		}
		rowHasError bool
	)
	for _, r := range t.Rows {
		rowHasError = false
		flag := func(col, msg string) { rowHasError = true; bad(r.Line, col, msg) }

		it := userItem{
			Line:       r.Line,
			Identifier: r.Cells["identifier"],
			Name:       r.Cells["name"],
			Email:      strings.ToLower(r.Cells["email"]),
			CourseCode: r.Cells["course_code"],
			CourseName: r.Cells["course_name"],
			ClassName:  r.Cells["class_name"],
		}

		for _, d := range users.ValidateIdentifier(it.Identifier) {
			flag("identifier", d.Message)
		}
		for _, d := range users.ValidateName(it.Name) {
			flag("name", d.Message)
		}
		if it.Email != "" {
			for _, d := range users.ValidateEmail(it.Email) {
				flag("email", d.Message)
			}
		}
		for col, v := range map[string]string{"course_code": it.CourseCode, "course_name": it.CourseName, "class_name": it.ClassName} {
			if utf8.RuneCountInString(v) > maxCourseFieldRunes {
				flag(col, fmt.Sprintf("Maksimal %d karakter.", maxCourseFieldRunes))
			}
		}

		if first, dup := seenID[it.Identifier]; dup {
			flag("identifier", fmt.Sprintf("Identifier ganda; sudah ada di baris %d.", first))
		} else if it.Identifier != "" {
			seenID[it.Identifier] = r.Line
		}
		if it.Email != "" {
			if first, dup := seenEmail[it.Email]; dup {
				flag("email", fmt.Sprintf("Email ganda; sudah ada di baris %d.", first))
			} else {
				seenEmail[it.Email] = r.Line
			}
		}

		it.Action = actionCreate
		if u, ok := byIdentifier[it.Identifier]; ok {
			if u.Role != dbgen.UserRoleMAHASISWA {
				// Only students are managed by import; another role is changed by an administrator (BR-24).
				flag("identifier", "Identifier ini milik akun yang bukan mahasiswa dan tidak dapat diubah lewat impor.")
			}
			it.Action, it.existingID = actionUpdate, u.ID
		}
		if it.Email != "" {
			if owner, ok := emailOwner[it.Email]; ok && owner != it.Identifier {
				flag("email", "Email sudah dipakai akun lain.")
			}
		}

		if !rowHasError {
			items = append(items, it)
		}
	}
	return items, problems, nil
}

// hashPlan fingerprints the plan so the apply step can detect that the database changed after the
// preview. It covers the semester, the file and every planned change.
func hashPlan(semester, fileSHA string, items []userItem) string {
	h := sha256.New()
	if err := json.NewEncoder(h).Encode(struct {
		Semester string     `json:"semester"`
		File     string     `json:"file"`
		Items    []userItem `json:"items"`
	}{semester, fileSHA, items}); err != nil {
		// Encoding plain strings and ints cannot fail; reaching this means the struct changed shape.
		panic(fmt.Sprintf("hashing import plan: %v", err))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// newCredentials makes a temporary password and its hash for every account the plan creates. Hashing is
// deliberately slow, so it runs on several cores.
func (s *Service) newCredentials(items []userItem) (plain, hashes map[string]string, err error) {
	plain, hashes = make(map[string]string), make(map[string]string)
	var ids []string
	for _, it := range items {
		if it.Action != actionCreate {
			continue
		}
		pw, err := users.NewTemporaryPassword(s.opts.TempPasswordLength)
		if err != nil {
			return nil, nil, err
		}
		plain[it.Identifier] = pw
		ids = append(ids, it.Identifier)
	}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		firstErr error
		sem      = make(chan struct{}, min(runtime.NumCPU(), 8))
	)
	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			h, err := s.hasher.Hash(plain[id])
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("hashing temporary password: %w", err)
				}
				return
			}
			hashes[id] = h
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, nil, firstErr
	}
	return plain, hashes, nil
}
