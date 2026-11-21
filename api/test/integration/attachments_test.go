package integration_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"classlab/api/internal/config"
)

// pdfBytes is the smallest thing that starts like a PDF; letters in PDF form are stored as sent.
func pdfBytes() []byte { return []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF\n") }

func p3PNG() []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 16, 16))); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// p3JPEGWithEXIF is a real JPEG with an APP1 "Exif" segment (carrying a recognisable marker) after SOI.
func p3JPEGWithEXIF() []byte {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 16, 16)), nil); err != nil {
		panic(err)
	}
	raw := buf.Bytes()
	payload := append([]byte("Exif\x00\x00"), []byte("GPS-LAT-6.2088-SECRET")...)
	seg := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte((len(payload) + 2) & 0xFF)}
	seg = append(seg, payload...)
	out := append([]byte{}, raw[:2]...)
	out = append(out, seg...)
	return append(out, raw[2:]...)
}

// uploadLetter posts a file to /attachments with the given kind; a nil file leaves the part out.
func (e *env) uploadLetter(token, kind string, file []byte) response {
	e.t.Helper()
	fields := map[string]string{}
	if kind != "" {
		fields["kind"] = kind
	}
	return e.multipartPost("/api/v1/attachments", token, fields, file)
}

// letterFor uploads a PDF letter as the given token's owner and returns its id.
func (e *env) letterFor(token string) string {
	e.t.Helper()
	res := e.uploadLetter(token, "LETTER", pdfBytes())
	mustStatus(e.t, res, http.StatusCreated)
	id, _ := res.json()["id"].(string)
	return id
}

func (e *env) getAttachment(token, id string) response {
	e.t.Helper()
	opts := []reqOpt{}
	if token != "" {
		opts = append(opts, bearer(token))
	}
	return e.do("GET", "/api/v1/attachments/"+id, nil, opts...)
}

func TestUploadLetterStoresAPDFAsSentAndServesItAsADownload(t *testing.T) {
	e := newEnv(t)
	student := e.login(mahasiswaA).accessToken

	res := e.uploadLetter(student, "LETTER", pdfBytes())
	mustStatus(t, res, http.StatusCreated)
	body := res.json()
	if body["kind"] != "LETTER" || body["content_type"] != "application/pdf" || int(body["size_bytes"].(float64)) != len(pdfBytes()) {
		t.Errorf("attachment = %v", body)
	}
	id := body["id"].(string)

	got := e.getAttachment(student, id)
	mustStatus(t, got, http.StatusOK)
	if !bytes.Equal(got.Body.Bytes(), pdfBytes()) {
		t.Error("the PDF came back different from what was sent")
	}
	h := got.Header()
	if h.Get("Content-Type") != "application/pdf" {
		t.Errorf("Content-Type = %q", h.Get("Content-Type"))
	}
	if !strings.HasPrefix(h.Get("Content-Disposition"), "attachment;") {
		t.Errorf("Content-Disposition = %q; a PDF must be a download", h.Get("Content-Disposition"))
	}
	if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Cache-Control") != "private, no-store" {
		t.Errorf("headers: nosniff=%q cache=%q", h.Get("X-Content-Type-Options"), h.Get("Cache-Control"))
	}
}

func TestUploadedImageLettersAreReencodedAndLoseTheirMetadata(t *testing.T) {
	e := newEnv(t)
	student := e.login(mahasiswaA).accessToken

	for name, data := range map[string][]byte{"png": p3PNG(), "jpeg with exif": p3JPEGWithEXIF()} {
		res := e.uploadLetter(student, "LETTER", data)
		mustStatus(t, res, http.StatusCreated)
		if res.json()["content_type"] != "image/jpeg" {
			t.Errorf("%s: content_type = %v, want image/jpeg", name, res.json()["content_type"])
		}
		got := e.getAttachment(student, res.json()["id"].(string))
		mustStatus(t, got, http.StatusOK)
		if _, err := jpeg.Decode(bytes.NewReader(got.Body.Bytes())); err != nil {
			t.Errorf("%s: stored file is not a JPEG: %v", name, err)
		}
		if bytes.Contains(got.Body.Bytes(), []byte("Exif")) || bytes.Contains(got.Body.Bytes(), []byte("GPS-LAT")) {
			t.Errorf("%s: metadata survived re-encoding", name)
		}
		if !strings.HasPrefix(got.Header().Get("Content-Disposition"), "inline;") {
			t.Errorf("%s: Content-Disposition = %q", name, got.Header().Get("Content-Disposition"))
		}
	}
}

func TestUploadLetterRejectsBadInput(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.Business.MaxLetterBytes = 1 << 10 })
	student := e.login(mahasiswaA).accessToken

	big := append(pdfBytes(), make([]byte, 2<<10)...)
	cases := map[string]response{
		"wrong kind":     e.uploadLetter(student, "PHOTO", pdfBytes()),
		"missing kind":   e.uploadLetter(student, "", pdfBytes()),
		"missing file":   e.uploadLetter(student, "LETTER", nil),
		"not a document": e.uploadLetter(student, "LETTER", []byte("just some text, not a file")),
		"too large":      e.uploadLetter(student, "LETTER", big),
		"broken image":   e.uploadLetter(student, "LETTER", []byte("\xff\xd8\xff\xe0 truncated")),
	}
	for name, res := range cases {
		if res.Code != http.StatusUnprocessableEntity || res.errorCode() != "VALIDATION_ERROR" {
			t.Errorf("%s: %d %s; want 422 VALIDATION_ERROR", name, res.Code, res.Body.String())
		}
	}
	if n := e.countRows("SELECT count(*) FROM attachments"); n != 0 {
		t.Errorf("%d attachment rows after only failed uploads", n)
	}
}

func TestOnlyRequestersUploadAndListLetters(t *testing.T) {
	e := newEnv(t)
	for name, tok := range map[string]string{"TU": e.tuToken(), "KALAB": e.kalabToken(), "ADMIN": e.adminToken(), "OB": e.obToken()} {
		if res := e.uploadLetter(tok, "LETTER", pdfBytes()); res.Code != http.StatusForbidden {
			t.Errorf("%s upload: %d; want 403", name, res.Code)
		}
		if res := e.do("GET", "/api/v1/attachments?kind=LETTER", nil, bearer(tok)); res.Code != http.StatusForbidden {
			t.Errorf("%s list: %d; want 403", name, res.Code)
		}
	}
	if res := e.uploadLetter("", "LETTER", pdfBytes()); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous upload: %d; want 401", res.Code)
	}
	if res := e.getAttachment("", "00000000-0000-0000-0000-000000000000"); res.Code != http.StatusUnauthorized {
		t.Errorf("anonymous get: %d; want 401", res.Code)
	}
}

func TestListLettersReturnsOnlyTheCallersOwn(t *testing.T) {
	e := newEnv(t)
	a, b := e.login(mahasiswaA).accessToken, e.login(mahasiswaB).accessToken
	mine := e.letterFor(a)
	e.letterFor(b)
	e.letterFor(b)

	res := e.do("GET", "/api/v1/attachments?kind=LETTER", nil, bearer(a))
	mustStatus(t, res, http.StatusOK)
	items := dataOf(res)
	if len(items) != 1 || items[0].(map[string]any)["id"] != mine {
		t.Errorf("list = %v; want only %s", items, mine)
	}
	if total := res.json()["meta"].(map[string]any)["total"].(float64); total != 1 {
		t.Errorf("total = %v", total)
	}
	if res := e.do("GET", "/api/v1/attachments?kind=PHOTO", nil, bearer(a)); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("kind=PHOTO: %d; want 422", res.Code)
	}
}

// TestAttachmentAccessMatrix walks who may open a letter: the uploader and ADMIN always; TU and KALAB only
// when it is attached to a booking of a room type they approve; nobody else, and not an unattached file.
func TestAttachmentAccessMatrix(t *testing.T) {
	e := newEnv(t)
	owner := e.login(mahasiswaA).accessToken
	other := e.login(mahasiswaB).accessToken
	kelas := e.insertRoom("ATT-KLS", "KELAS", 40)
	lab := e.insertRoom("ATT-LAB", "LAB", 30)

	unattached := e.letterFor(owner)
	onKelas, onLab := e.letterFor(owner), e.letterFor(owner)
	mustStatus(t, e.postBooking(owner, kelas, at(tue, "08:00"), at(tue, "09:00"), map[string]any{"letter_attachment_id": onKelas}), http.StatusCreated)
	mustStatus(t, e.postBooking(owner, lab, at(tue, "08:00"), at(tue, "09:00"), map[string]any{"letter_attachment_id": onLab}), http.StatusCreated)

	tu, kalab, admin, ob, org := e.tuToken(), e.kalabToken(), e.adminToken(), e.obToken(), e.orgToken()
	type row struct {
		id   string
		who  string
		tok  string
		want int
	}
	var rows []row
	add := func(id string, want map[string]int) {
		toks := map[string]string{"owner": owner, "other student": other, "organisation": org, "TU": tu, "KALAB": kalab, "ADMIN": admin, "OB": ob}
		for who, tok := range toks {
			rows = append(rows, row{id, who, tok, want[who]})
		}
	}
	add(unattached, map[string]int{"owner": 200, "other student": 404, "organisation": 404, "TU": 404, "KALAB": 404, "ADMIN": 200, "OB": 404})
	add(onKelas, map[string]int{"owner": 200, "other student": 404, "organisation": 404, "TU": 200, "KALAB": 404, "ADMIN": 200, "OB": 404})
	add(onLab, map[string]int{"owner": 200, "other student": 404, "organisation": 404, "TU": 404, "KALAB": 200, "ADMIN": 200, "OB": 404})
	for _, r := range rows {
		if got := e.getAttachment(r.tok, r.id); got.Code != r.want {
			t.Errorf("%s opening a letter: %d; want %d", r.who, got.Code, r.want)
		}
	}
}

func TestAttachmentUnknownOrMalformedIdIs404(t *testing.T) {
	e := newEnv(t)
	tok := e.login(mahasiswaA).accessToken
	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid"} {
		if res := e.getAttachment(tok, id); res.Code != http.StatusNotFound {
			t.Errorf("%s: %d; want 404", id, res.Code)
		}
	}
}

func TestAPurgedAttachmentIs410ForThoseWhoMayOpenItAnd404ForOthers(t *testing.T) {
	e := newEnv(t)
	owner := e.login(mahasiswaA).accessToken
	other := e.login(mahasiswaB).accessToken
	id := e.letterFor(owner)
	if _, err := e.pool.Exec(t.Context(), "UPDATE attachments SET purged_at = now() WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}

	res := e.getAttachment(owner, id)
	if res.Code != http.StatusGone || res.errorCode() != "GONE" {
		t.Errorf("owner: %d %s; want 410 GONE", res.Code, res.Body.String())
	}
	if res := e.getAttachment(e.adminToken(), id); res.Code != http.StatusGone {
		t.Errorf("admin: %d; want 410", res.Code)
	}
	if res := e.getAttachment(other, id); res.Code != http.StatusNotFound {
		t.Errorf("someone else: %d; want 404, not a hint that the file existed", res.Code)
	}
}

func TestAttachmentFileNameFromTheClientIsNeverUsedInHeaders(t *testing.T) {
	e := newEnv(t)
	student := e.login(mahasiswaA).accessToken
	id := e.letterFor(student)
	h := e.getAttachment(student, id).Header().Get("Content-Disposition")
	if strings.Contains(h, "daftar") || !strings.Contains(h, "surat-") {
		t.Errorf("Content-Disposition = %q; want a generated name", h)
	}
}
