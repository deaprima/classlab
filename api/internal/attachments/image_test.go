package attachments

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// jpegWithOrientation encodes a w x h picture whose top-left pixel is red and the rest blue, then inserts
// an EXIF segment carrying the orientation and a fake GPS block, as a phone camera would.
func jpegWithOrientation(t *testing.T, w, h, orientation int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{0, 0, 255, 255})
		}
	}
	// A 4x4 red block keeps the corner visible after JPEG's 8x8 compression.
	for y := 0; y < min(4, h); y++ {
		for x := 0; x < min(4, w); x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()

	exif := []byte{
		0xFF, 0xE1, 0x00, 0x22,
		'E', 'x', 'i', 'f', 0, 0,
		'M', 'M', 0x00, 0x2A, 0, 0, 0, 8,
		0x00, 0x01,
		0x01, 0x12, 0x00, 0x03, 0, 0, 0, 1, 0, byte(orientation), 0, 0,
		0, 0, 0, 0,
	}
	gps := append([]byte{0xFF, 0xFE, 0x00, 0x16}, []byte("GPS:-6.200000,106.816666")[:20]...) // a comment segment
	out := append([]byte{}, raw[:2]...)
	out = append(out, exif...)
	out = append(out, gps...)
	return append(out, raw[2:]...)
}

func decodeJPEG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output is not a decodable JPEG: %v", err)
	}
	return img
}

func isReddish(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r>>8 > 180 && g>>8 < 90 && b>>8 < 90
}

func TestProcessImageAppliesExifOrientationToThePixels(t *testing.T) {
	// The red block is at the top-left of the stored picture. Where it must end up once the picture is
	// upright, for a 16x8 source: orientation 1 stays, 6 (rotate clockwise) moves it to the top-right of
	// an 8x16 picture, 8 (counter-clockwise) to the bottom-left, 3 (180) to the bottom-right.
	cases := []struct {
		orientation int
		wantW       int
		wantH       int
		redAt       image.Point
	}{
		{1, 16, 8, image.Pt(1, 1)},
		{3, 16, 8, image.Pt(14, 6)},
		{6, 8, 16, image.Pt(6, 1)},
		{8, 8, 16, image.Pt(1, 14)},
	}
	for _, c := range cases {
		out, err := processImage(jpegWithOrientation(t, 16, 8, c.orientation))
		if err != nil {
			t.Fatalf("orientation %d: %v", c.orientation, err)
		}
		img := decodeJPEG(t, out)
		if b := img.Bounds(); b.Dx() != c.wantW || b.Dy() != c.wantH {
			t.Errorf("orientation %d: size %dx%d, want %dx%d", c.orientation, b.Dx(), b.Dy(), c.wantW, c.wantH)
			continue
		}
		if !isReddish(img.At(c.redAt.X, c.redAt.Y)) {
			t.Errorf("orientation %d: pixel %v is %v, want the red corner there", c.orientation, c.redAt, img.At(c.redAt.X, c.redAt.Y))
		}
	}
}

func TestProcessImageRemovesMetadataAndTrailingPayload(t *testing.T) {
	src := jpegWithOrientation(t, 16, 8, 6)
	payload := []byte("<?php system($_GET['c']); ?>")
	src = append(src, payload...) // bytes after the end-of-image marker, as in a polyglot file

	out, err := processImage(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range [][]byte{[]byte("Exif"), []byte("GPS:"), payload, {0xFF, 0xE1}} {
		if bytes.Contains(out, leaked) {
			t.Errorf("output still contains %q", leaked)
		}
	}
	if jpegOrientation(out) != 0 {
		t.Error("output still carries an orientation tag; the pixels should already be upright")
	}
}

func TestProcessImageShrinksLargePicturesOnly(t *testing.T) {
	big := image.NewRGBA(image.Rect(0, 0, 3200, 1600))
	var buf bytes.Buffer
	if err := png.Encode(&buf, big); err != nil {
		t.Fatal(err)
	}
	out, err := processImage(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if b := decodeJPEG(t, out).Bounds(); b.Dx() != 1600 || b.Dy() != 800 {
		t.Errorf("3200x1600 became %dx%d, want 1600x800", b.Dx(), b.Dy())
	}

	tall := image.NewRGBA(image.Rect(0, 0, 900, 2400))
	buf.Reset()
	if err := png.Encode(&buf, tall); err != nil {
		t.Fatal(err)
	}
	out, err = processImage(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if b := decodeJPEG(t, out).Bounds(); b.Dy() != 1600 || b.Dx() != 600 {
		t.Errorf("900x2400 became %dx%d, want 600x1600", b.Dx(), b.Dy())
	}

	small := jpegWithOrientation(t, 640, 480, 1)
	out, err = processImage(small)
	if err != nil {
		t.Fatal(err)
	}
	if b := decodeJPEG(t, out).Bounds(); b.Dx() != 640 || b.Dy() != 480 {
		t.Errorf("a 640x480 picture was resized to %dx%d", b.Dx(), b.Dy())
	}
}

func TestProcessImageFlattensTransparencyOnWhite(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8)) // every pixel fully transparent
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	out, err := processImage(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := decodeJPEG(t, out).At(4, 4).RGBA()
	if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Errorf("transparent pixel became (%d,%d,%d), want white", r>>8, g>>8, b>>8)
	}
}

func TestProcessImageReadsWebP(t *testing.T) {
	// A 1x1 lossless WebP.
	webp, err := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	if err != nil {
		t.Fatal(err)
	}
	out, err := processImage(webp)
	if err != nil {
		t.Fatalf("webp: %v", err)
	}
	if b := decodeJPEG(t, out).Bounds(); b.Dx() != 1 || b.Dy() != 1 {
		t.Errorf("webp became %dx%d", b.Dx(), b.Dy())
	}
}

func TestProcessImageRefusesWhatIsNotAPicture(t *testing.T) {
	good := jpegWithOrientation(t, 16, 8, 1)

	cases := map[string]struct {
		data []byte
		want error
	}{
		"text":                 {[]byte("<html>bukan gambar</html>"), ErrNotImage},
		"a pdf":                {[]byte("%PDF-1.7 ..."), ErrNotImage},
		"empty":                {nil, ErrNotImage},
		"gif":                  {[]byte("GIF89a......"), ErrNotImage},
		"jpeg cut in half":     {good[:len(good)/2], ErrBrokenImage},
		"png signature only":   {[]byte("\x89PNG\r\n\x1a\n"), ErrBrokenImage},
		"jpeg header, garbage": {append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x00}, 40)...), ErrBrokenImage},
	}
	for name, c := range cases {
		if _, err := processImage(c.data); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
}

func TestProcessImageRefusesDecompressionBombs(t *testing.T) {
	// 45 megapixels of one grey value compress to a few kilobytes but would need 45 MB to decode.
	img := image.NewGray(image.Rect(0, 0, 30000, 1500))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if buf.Len() > 1<<20 {
		t.Fatalf("test file is %d bytes; it should be tiny", buf.Len())
	}
	if _, err := processImage(buf.Bytes()); !errors.Is(err, ErrImageTooLarge) {
		t.Errorf("err = %v, want ErrImageTooLarge", err)
	}
}

func TestMagicByteChecks(t *testing.T) {
	if !isPDF([]byte("%PDF-1.4\n...")) || isPDF([]byte(" %PDF-")) || isPDF([]byte("PDF")) {
		t.Error("isPDF must match only a leading %PDF-")
	}
	for name, head := range map[string][]byte{
		"jpeg": {0xFF, 0xD8, 0xFF, 0xE0},
		"png":  []byte("\x89PNG\r\n\x1a\n"),
		"webp": []byte("RIFF\x10\x00\x00\x00WEBPVP8 "),
	} {
		if !isImage(head) {
			t.Errorf("%s not recognised", name)
		}
	}
	if isImage([]byte("RIFF\x10\x00\x00\x00WAVEfmt ")) || isImage([]byte("GIF89a")) {
		t.Error("a RIFF/WAVE or GIF file was accepted as an image")
	}
}

func TestSanitizeNameKeepsOnlyABaseName(t *testing.T) {
	for in, want := range map[string]string{
		"surat izin.pdf":          "surat izin.pdf",
		`C:\Users\ani\surat.pdf`:  "surat.pdf",
		"../../etc/passwd":        "passwd",
		"bad\x00name\x1b[31m.pdf": "badname[31m.pdf",
		"   spasi.pdf  ":          "spasi.pdf",
		"":                        "",
	} {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := sanitizeName(string(bytes.Repeat([]byte("a"), 500))); len(got) != maxOriginalNameRunes {
		t.Errorf("long name length = %d, want %d", len(got), maxOriginalNameRunes)
	}
}

func TestStoredPathShapeBlocksEscapes(t *testing.T) {
	good := "attachments/0b2f8f0e-0f4c-4d65-9d0a-2f6a1c3b9e11.jpg"
	for _, p := range []string{good, "reports/0b2f8f0e-0f4c-4d65-9d0a-2f6a1c3b9e11.png", "attachments/0b2f8f0e-0f4c-4d65-9d0a-2f6a1c3b9e11.pdf"} {
		if !storedPath.MatchString(p) {
			t.Errorf("%q should be valid", p)
		}
	}
	for _, p := range []string{
		"../attachments/0b2f8f0e-0f4c-4d65-9d0a-2f6a1c3b9e11.jpg", "attachments/../x.jpg", "/etc/passwd",
		"attachments/0b2f8f0e-0f4c-4d65-9d0a-2f6a1c3b9e11.exe", "other/0b2f8f0e-0f4c-4d65-9d0a-2f6a1c3b9e11.jpg",
		"attachments/not-a-uuid.jpg", "attachments/0b2f8f0e-0f4c-4d65-9d0a-2f6a1c3b9e11.jpg/extra", "",
	} {
		if storedPath.MatchString(p) {
			t.Errorf("%q should be refused", p)
		}
	}
}
