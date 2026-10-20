// Package imports reads the CSV files TU and administrators upload each semester (brief BR-23, BR-24):
// student accounts now, academic schedules in Phase 2. An import is always two steps, a preview that
// stores nothing and an apply that must match the preview, and it is all-or-nothing. The file content
// is never stored or logged; only its hash and a summary are kept.
package imports

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// RowError points at one cell of the uploaded file. Line 1 is the header.
type RowError struct {
	Line    int    `json:"line"`
	Column  string `json:"column,omitempty"`
	Message string `json:"message"`
}

// Row is one data line with its cells keyed by lower-cased header name.
type Row struct {
	Line  int
	Cells map[string]string
}

// Table is a parsed CSV: the rows plus the SHA-256 of the raw bytes, used to refuse the same file twice.
type Table struct {
	Rows   []Row
	SHA256 string
}

// Limits bound what a single upload may cost.
type Limits struct {
	MaxBytes int64
	MaxRows  int
}

// FileError is a problem with the file as a whole (too large, unreadable, wrong header) rather than
// with one row.
type FileError struct{ Problems []RowError }

func (e *FileError) Error() string {
	if len(e.Problems) == 0 {
		return "invalid import file"
	}
	return e.Problems[0].Message
}

func fileProblem(line int, column, format string, args ...any) *FileError {
	return &FileError{Problems: []RowError{{Line: line, Column: column, Message: fmt.Sprintf(format, args...)}}}
}

const utf8BOM = "\xef\xbb\xbf"

// Parse reads a CSV with a header row. The separator, comma or semicolon, is detected from the header
// line, a UTF-8 byte order mark is accepted, and reading stops at the limits instead of loading an
// unbounded file. Columns in required must exist; columns not listed anywhere are ignored.
func Parse(r io.Reader, lim Limits, required []string) (*Table, error) {
	// One byte past the limit tells "exactly at the limit" apart from "over it".
	raw, err := io.ReadAll(io.LimitReader(r, lim.MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading upload: %w", err)
	}
	if int64(len(raw)) > lim.MaxBytes {
		return nil, fileProblem(0, "", "File terlalu besar (maksimal %d MB).", lim.MaxBytes>>20)
	}
	sum := sha256.Sum256(raw)

	text := strings.TrimPrefix(string(raw), utf8BOM)
	if strings.TrimSpace(text) == "" {
		return nil, fileProblem(1, "", "File kosong.")
	}

	cr := csv.NewReader(strings.NewReader(text))
	cr.Comma = detectSeparator(text)
	cr.FieldsPerRecord = -1 // a short row is reported with its line number, not as a parse failure
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, fileProblem(1, "", "Baris judul kolom tidak dapat dibaca: %s", csvReason(err))
	}
	cols := make(map[string]int, len(header))
	for i, h := range header {
		name := strings.ToLower(strings.TrimSpace(h))
		if name == "" {
			continue
		}
		if _, dup := cols[name]; dup {
			return nil, fileProblem(1, name, "Kolom %q muncul lebih dari sekali.", name)
		}
		cols[name] = i
	}
	var missing []RowError
	for _, name := range required {
		if _, ok := cols[name]; !ok {
			missing = append(missing, RowError{Line: 1, Column: name, Message: fmt.Sprintf("Kolom wajib %q tidak ada.", name)})
		}
	}
	if len(missing) > 0 {
		return nil, &FileError{Problems: missing}
	}

	t := &Table{SHA256: hex.EncodeToString(sum[:])}
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) {
				return nil, fileProblem(pe.Line, "", "Baris tidak dapat dibaca: %s", csvReason(err))
			}
			return nil, fmt.Errorf("parsing csv: %w", err)
		}
		if isBlank(rec) {
			continue
		}
		if len(t.Rows) >= lim.MaxRows {
			return nil, fileProblem(0, "", "File memuat lebih dari %d baris data.", lim.MaxRows)
		}
		line, _ := cr.FieldPos(0) // the line the record just read starts on
		cells := make(map[string]string, len(cols))
		for name, i := range cols {
			if i < len(rec) {
				cells[name] = strings.TrimSpace(rec[i])
			}
		}
		t.Rows = append(t.Rows, Row{Line: line, Cells: cells})
	}
	return t, nil
}

// detectSeparator picks the semicolon when the header line has more semicolons than commas, which is how
// Excel in an Indonesian locale writes CSV.
func detectSeparator(text string) rune {
	first, _, _ := strings.Cut(text, "\n")
	if bytes.Count([]byte(first), []byte(";")) > bytes.Count([]byte(first), []byte(",")) {
		return ';'
	}
	return ','
}

func isBlank(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// csvReason keeps the reader's message short and free of file content.
func csvReason(err error) string {
	var pe *csv.ParseError
	if errors.As(err, &pe) && pe.Err != nil {
		return pe.Err.Error()
	}
	return err.Error()
}
