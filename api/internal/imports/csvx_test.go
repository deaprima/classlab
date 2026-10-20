package imports

import (
	"errors"
	"strings"
	"testing"
)

var testLimits = Limits{MaxBytes: 1 << 20, MaxRows: 100}

func parse(t *testing.T, text string, lim Limits) (*Table, error) {
	t.Helper()
	return Parse(strings.NewReader(text), lim, []string{"identifier", "name"})
}

func TestParseReadsRowsWithLineNumbers(t *testing.T) {
	tbl, err := parse(t, "identifier,name,email\n23001,Andi,andi@x.test\n23002,Bunga,\n", testLimits)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(tbl.Rows))
	}
	if r := tbl.Rows[0]; r.Line != 2 || r.Cells["identifier"] != "23001" || r.Cells["email"] != "andi@x.test" {
		t.Errorf("row 0 = %+v", r)
	}
	if r := tbl.Rows[1]; r.Line != 3 || r.Cells["name"] != "Bunga" || r.Cells["email"] != "" {
		t.Errorf("row 1 = %+v", r)
	}
}

func TestParseDetectsSemicolonAndAcceptsBOM(t *testing.T) {
	tbl, err := parse(t, "\xef\xbb\xbfidentifier;name;email\r\n23001;Andi, S.Kom;andi@x.test\r\n", testLimits)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != 1 || tbl.Rows[0].Cells["name"] != "Andi, S.Kom" || tbl.Rows[0].Cells["email"] != "andi@x.test" {
		t.Errorf("rows = %+v", tbl.Rows)
	}
}

func TestParseTrimsHeaderCaseAndCells(t *testing.T) {
	tbl, err := parse(t, " Identifier , NAME \n  23001  ,  Andi  \n", testLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := tbl.Rows[0].Cells; got["identifier"] != "23001" || got["name"] != "Andi" {
		t.Errorf("cells = %v", got)
	}
}

func TestParseSkipsBlankLinesButKeepsLineNumbers(t *testing.T) {
	tbl, err := parse(t, "identifier,name\n23001,Andi\n,\n\n23002,Bunga\n", testLimits)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != 2 || tbl.Rows[1].Line != 5 {
		t.Errorf("rows = %+v; want the second data row on line 5", tbl.Rows)
	}
}

func TestParseHashIsOfTheRawBytes(t *testing.T) {
	a, _ := parse(t, "identifier,name\n1,A\n", testLimits)
	b, _ := parse(t, "identifier,name\n1,A\n", testLimits)
	c, _ := parse(t, "identifier,name\n1,B\n", testLimits)
	if a.SHA256 != b.SHA256 || a.SHA256 == c.SHA256 || len(a.SHA256) != 64 {
		t.Errorf("hashes: %s %s %s", a.SHA256, b.SHA256, c.SHA256)
	}
}

func TestParseRejectsFileProblems(t *testing.T) {
	cases := map[string]struct {
		text string
		lim  Limits
	}{
		"empty":                {"", testLimits},
		"only whitespace":      {"  \n\n", testLimits},
		"missing column":       {"identifier,email\n1,a@x\n", testLimits},
		"duplicate column":     {"identifier,name,name\n1,A,B\n", testLimits},
		"over the byte limit":  {"identifier,name\n" + strings.Repeat("1,A\n", 100), Limits{MaxBytes: 50, MaxRows: 100}},
		"over the row limit":   {"identifier,name\n1,A\n2,B\n3,C\n", Limits{MaxBytes: 1 << 20, MaxRows: 2}},
		"unterminated quote":   {"identifier,name\n1,\"Andi\n", testLimits},
		"bare quote in a cell": {"identifier,name\n1,An\"di\n", testLimits},
	}
	for name, c := range cases {
		_, err := parse(t, c.text, c.lim)
		var fe *FileError
		if !errors.As(err, &fe) || len(fe.Problems) == 0 {
			t.Errorf("%s: err = %v, want a *FileError", name, err)
		}
	}
}

func TestParseAcceptsExactlyTheLimits(t *testing.T) {
	text := "identifier,name\n1,A\n2,B\n"
	tbl, err := parse(t, text, Limits{MaxBytes: int64(len(text)), MaxRows: 2})
	if err != nil || len(tbl.Rows) != 2 {
		t.Errorf("at the limits: %v, %v", tbl, err)
	}
}

func TestParseMissingColumnNamesEveryOne(t *testing.T) {
	_, err := Parse(strings.NewReader("email\nx\n"), testLimits, []string{"identifier", "name"})
	var fe *FileError
	if !errors.As(err, &fe) || len(fe.Problems) != 2 {
		t.Fatalf("err = %v, want one problem per missing column", err)
	}
}
