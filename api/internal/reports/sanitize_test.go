package reports

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// secret stands in for a GPS position; it must never survive stripping.
const secret = "GPS-LAT-6.2088-LON-106.8456"

func testPicture() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			img.SetGray(x, y, color.Gray{Y: uint8(x*16 + y)})
		}
	}
	return img
}

// exifBlock builds an APP1 payload: an IFD with an Orientation entry (when orientation > 0) and the
// secret hiding after it, the way real EXIF stores GPS data beyond the first IFD.
type byteOrder interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

func exifBlock(order byteOrder, orientation int) []byte {
	tiff := []byte{'M', 'M'}
	if order == binary.LittleEndian {
		tiff = []byte{'I', 'I'}
	}
	tiff = order.AppendUint16(tiff, 42)
	tiff = order.AppendUint32(tiff, 8)
	if orientation > 0 {
		tiff = order.AppendUint16(tiff, 1)
		tiff = order.AppendUint16(tiff, 0x0112)
		tiff = order.AppendUint16(tiff, 3)
		tiff = order.AppendUint32(tiff, 1)
		tiff = order.AppendUint16(tiff, uint16(orientation))
		tiff = append(tiff, 0, 0)
	} else {
		tiff = order.AppendUint16(tiff, 0)
	}
	tiff = order.AppendUint32(tiff, 0)
	tiff = append(tiff, secret...)
	return append([]byte("Exif\x00\x00"), tiff...)
}

func jpegSegment(marker byte, payload []byte) []byte {
	seg := []byte{0xFF, marker}
	seg = binary.BigEndian.AppendUint16(seg, uint16(len(payload)+2))
	return append(seg, payload...)
}

// jpegWith encodes the test picture and inserts extra segments straight after the start marker.
func jpegWith(t *testing.T, segments ...[]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, testPicture(), nil); err != nil {
		t.Fatal(err)
	}
	enc := buf.Bytes()
	out := append([]byte{}, enc[:2]...)
	for _, s := range segments {
		out = append(out, s...)
	}
	return append(out, enc[2:]...)
}

func pixels(t *testing.T, data []byte) []byte {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("result is not a decodable JPEG: %v", err)
	}
	g, ok := img.(*image.Gray)
	if !ok {
		t.Fatalf("decoded to %T", img)
	}
	return g.Pix
}

func TestStripJPEGRemovesMetadataAndKeepsThePicture(t *testing.T) {
	jfif := jpegSegment(jpegAPP0, []byte("JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00"))
	icc := jpegSegment(0xE2, []byte("ICC_PROFILE\x00\x01\x01profile"))
	adobe := jpegSegment(0xEE, []byte("Adobe\x00\x64\x00\x00\x00\x00\x01"))
	exif := jpegSegment(jpegAPP1, exifBlock(binary.BigEndian, 6))
	xmp := jpegSegment(jpegAPP1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta>"+secret+"</x:xmpmeta>"))
	iptc := jpegSegment(0xED, []byte("Photoshop 3.0\x008BIM"+secret))
	comment := jpegSegment(0xFE, []byte("taken at "+secret))

	in := jpegWith(t, jfif, exif, xmp, icc, iptc, adobe, comment)
	out, err := stripJPEG(in)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(out, []byte(secret)) || bytes.Contains(out, []byte("xmpmeta")) || bytes.Contains(out, []byte("Photoshop")) {
		t.Error("metadata survived stripping")
	}
	for name, keep := range map[string][]byte{"JFIF": jfif, "colour profile": icc, "Adobe transform": adobe} {
		if !bytes.Contains(out, keep) {
			t.Errorf("%s segment was removed; it changes how the picture is decoded", name)
		}
	}
	if !bytes.Equal(pixels(t, out), pixels(t, in)) {
		t.Error("stripping changed the picture")
	}
	if len(out) >= len(in) {
		t.Errorf("output (%d bytes) is not smaller than input (%d)", len(out), len(in))
	}

	// The orientation is kept: phones rely on it to show portrait photos upright.
	i := bytes.Index(out, []byte("Exif\x00\x00"))
	if i < 0 {
		t.Fatal("orientation was dropped along with the rest of EXIF")
	}
	if got := exifOrientation(out[i : i+32]); got != 6 {
		t.Errorf("kept orientation = %d; want 6", got)
	}
	if bytes.Count(out, []byte("Exif\x00\x00")) != 1 {
		t.Error("more than one EXIF segment in the output")
	}
	if j := bytes.Index(out, jfif); j > i {
		t.Error("EXIF was placed ahead of JFIF")
	}
}

func TestStripJPEGOrientationCases(t *testing.T) {
	for name, tc := range map[string]struct {
		exif     []byte
		wantKept int
	}{
		"big endian 8":    {exifBlock(binary.BigEndian, 8), 8},
		"little endian 3": {exifBlock(binary.LittleEndian, 3), 3},
		"normal (1)":      {exifBlock(binary.BigEndian, 1), 0}, // nothing to preserve
		"none":            {exifBlock(binary.BigEndian, 0), 0},
		"out of range":    {exifBlock(binary.BigEndian, 9), 0},
		"not exif":        {[]byte("http://ns.adobe.com/xap/1.0/\x00" + secret), 0},
	} {
		out, err := stripJPEG(jpegWith(t, jpegSegment(jpegAPP1, tc.exif)))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if bytes.Contains(out, []byte(secret)) {
			t.Errorf("%s: the secret survived", name)
		}
		i := bytes.Index(out, []byte("Exif\x00\x00"))
		switch {
		case tc.wantKept == 0 && i >= 0:
			t.Errorf("%s: an EXIF segment was kept although there is no orientation to preserve", name)
		case tc.wantKept > 0 && (i < 0 || exifOrientation(out[i:i+32]) != tc.wantKept):
			t.Errorf("%s: orientation not preserved", name)
		}
		pixels(t, out)
	}
}

func TestStripJPEGRefusesBrokenFraming(t *testing.T) {
	good := jpegWith(t, jpegSegment(jpegAPP1, exifBlock(binary.BigEndian, 6)))
	for name, data := range map[string][]byte{
		"empty":                        nil,
		"just the start marker":        {0xFF, 0xD8},
		"no start marker":              append([]byte{0x00, 0x00}, good[2:]...),
		"ends before the scan":         append(append([]byte{0xFF, 0xD8}, jpegSegment(jpegAPP0, []byte("JFIF\x00"))...), 0xFF, jpegEOI),
		"segment longer than the file": append(append([]byte{0xFF, 0xD8}, 0xFF, jpegAPP1), 0xFF, 0xFF, 'x'),
		"length below the minimum":     {0xFF, 0xD8, 0xFF, jpegAPP1, 0x00, 0x01, 0xFF, jpegSOS},
		"garbage between segments":     {0xFF, 0xD8, 0x12, 0x34, 0x56},
		"truncated length":             {0xFF, 0xD8, 0xFF, jpegAPP1, 0x00},
	} {
		if _, err := stripJPEG(data); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// pngWith encodes the test picture and inserts extra chunks right after the header chunk.
func pngWith(t *testing.T, chunks ...[]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, testPicture()); err != nil {
		t.Fatal(err)
	}
	enc := buf.Bytes()
	ihdrEnd := 8 + 8 + 13 + 4 // signature, chunk header, IHDR data, CRC
	out := append([]byte{}, enc[:ihdrEnd]...)
	for _, c := range chunks {
		out = append(out, c...)
	}
	return append(out, enc[ihdrEnd:]...)
}

func pngChunk(typ string, data []byte) []byte {
	c := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	c = append(c, typ...)
	c = append(c, data...)
	return binary.BigEndian.AppendUint32(c, crc32.ChecksumIEEE(append([]byte(typ), data...)))
}

func TestStripPNGRemovesMetadataAndKeepsThePicture(t *testing.T) {
	gamma := pngChunk("gAMA", []byte{0, 1, 0x86, 0xA0})
	in := pngWith(t,
		pngChunk("tEXt", []byte("Location\x00"+secret)),
		pngChunk("eXIf", exifBlock(binary.BigEndian, 6)[6:]),
		pngChunk("iTXt", []byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00"+secret)),
		pngChunk("zTXt", []byte("Comment\x00\x00"+secret)),
		pngChunk("tIME", []byte{0x07, 0xEA, 10, 14, 10, 0, 0}),
		gamma,
	)
	out, err := stripPNG(in)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte(secret)) {
		t.Error("metadata survived stripping")
	}
	for _, typ := range []string{"tEXt", "eXIf", "iTXt", "zTXt", "tIME"} {
		if bytes.Contains(out, []byte(typ)) {
			t.Errorf("%s chunk survived", typ)
		}
	}
	if !bytes.Contains(out, gamma) {
		t.Error("the gamma chunk, which affects rendering, was removed")
	}

	want, err := png.Decode(bytes.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("result is not a decodable PNG: %v", err)
	}
	if !bytes.Equal(got.(*image.Gray).Pix, want.(*image.Gray).Pix) {
		t.Error("stripping changed the picture")
	}
}

func TestStripPNGDropsTrailingDataAndRefusesBrokenFraming(t *testing.T) {
	good := pngWith(t)

	out, err := stripPNG(append(append([]byte{}, good...), []byte("trailing "+secret)...))
	if err != nil || bytes.Contains(out, []byte(secret)) || !bytes.Equal(out, good) {
		t.Errorf("data after IEND was kept (err %v)", err)
	}

	for name, data := range map[string][]byte{
		"empty":             nil,
		"signature only":    pngSignature,
		"no IEND":           good[:len(good)-12],
		"truncated chunk":   good[:len(good)-5],
		"first not IHDR":    append(append([]byte{}, pngSignature...), pngChunk("tEXt", []byte("a\x00b"))...),
		"length past end":   append(append([]byte{}, pngSignature...), 0xFF, 0xFF, 0xFF, 0xFF, 'I', 'H', 'D', 'R'),
		"not a png at all":  []byte("hello, this is not a picture"),
		"truncated header":  good[:10],
		"huge declared len": append(append([]byte{}, pngSignature...), 0x7F, 0xFF, 0xFF, 0xFF, 'I', 'H', 'D', 'R', 0, 0, 0, 0),
	} {
		if _, err := stripPNG(data); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func webpChunk(fourcc string, payload []byte) []byte {
	c := append([]byte(fourcc), binary.LittleEndian.AppendUint32(nil, uint32(len(payload)))...)
	c = append(c, payload...)
	if len(payload)%2 == 1 {
		c = append(c, 0)
	}
	return c
}

func webpFile(chunks ...[]byte) []byte {
	body := []byte("WEBP")
	for _, c := range chunks {
		body = append(body, c...)
	}
	out := append([]byte("RIFF"), binary.LittleEndian.AppendUint32(nil, uint32(len(body)))...)
	return append(out, body...)
}

func TestStripWebPRemovesMetadataAndFixesTheContainer(t *testing.T) {
	// VP8X flags: ICC (0x20), alpha (0x10), EXIF (0x08), XMP (0x04). Only the last two must be cleared.
	vp8x := webpChunk("VP8X", []byte{0x20 | 0x10 | 0x08 | 0x04, 0, 0, 0, 15, 0, 0, 15, 0, 0})
	iccp := webpChunk("ICCP", []byte("profile-bytes!"))     // even length
	odd := webpChunk("VP8L", []byte("pixel-data-odd")[:13]) // odd length, padded
	in := webpFile(vp8x, iccp, webpChunk("EXIF", []byte("Exif\x00\x00"+secret)), odd, webpChunk("XMP ", []byte("<xmp>"+secret+"</xmp>")))

	out, err := stripWebP(in)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte(secret)) || bytes.Contains(out, []byte("EXIF")) || bytes.Contains(out, []byte("XMP ")) {
		t.Error("metadata survived stripping")
	}
	if want := 0x20 | 0x10; out[12+8] != byte(want) {
		t.Errorf("VP8X flags = %#x; want %#x (EXIF and XMP bits cleared, the rest kept)", out[12+8], want)
	}
	if !bytes.Contains(out, iccp) || !bytes.Contains(out, odd) {
		t.Error("a picture chunk was lost or its padding changed")
	}
	if size := int(binary.LittleEndian.Uint32(out[4:8])); size != len(out)-8 {
		t.Errorf("RIFF size = %d; want %d", size, len(out)-8)
	}
	if string(out[:4]) != "RIFF" || string(out[8:12]) != "WEBP" {
		t.Error("container header damaged")
	}

	// Content after the container's declared end is not part of the file.
	trailing := append(append([]byte{}, in...), []byte("trailer "+secret)...)
	if out2, err := stripWebP(trailing); err != nil || bytes.Contains(out2, []byte(secret)) {
		t.Errorf("trailing data was kept (err %v)", err)
	}
}

func TestStripWebPRefusesBrokenFraming(t *testing.T) {
	good := webpFile(webpChunk("VP8L", []byte("0123456789")))
	badSize := append([]byte{}, good...)
	binary.LittleEndian.PutUint32(badSize[4:8], uint32(len(good))) // claims more than there is
	chunkPastEnd := webpFile(append([]byte("VP8L"), 0xFF, 0xFF, 0xFF, 0x7F))
	shortVP8X := webpFile(webpChunk("VP8X", []byte{1, 2, 3}))

	for name, data := range map[string][]byte{
		"empty":                nil,
		"not riff":             []byte("RIFX\x04\x00\x00\x00WEBP"),
		"not webp":             []byte("RIFF\x04\x00\x00\x00WAVE"),
		"size too large":       badSize,
		"chunk past end":       chunkPastEnd,
		"truncated":            good[:len(good)-3],
		"short VP8X":           shortVP8X,
		"cut after the header": good[:12],
		"no chunks at all":     webpFile(),
	} {
		out, err := stripWebP(data)
		if name == "no chunks at all" {
			// A container without chunks is well-framed, just empty; it is not worth refusing.
			if err != nil || len(out) != 12 {
				t.Errorf("%s: %v / %d bytes", name, err, len(out))
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Hostile input must produce an error or a result, never a panic or a hang.
func TestStripNeverPanicsOnTruncatedOrCorruptedFiles(t *testing.T) {
	samples := map[imageType][]byte{
		imageJPEG: jpegWith(t, jpegSegment(jpegAPP1, exifBlock(binary.BigEndian, 6)), jpegSegment(0xFE, []byte("c"))),
		imagePNG:  pngWith(t, pngChunk("tEXt", []byte("a\x00b"))),
		imageWebP: webpFile(webpChunk("VP8X", make([]byte, 10)), webpChunk("EXIF", []byte("e")), webpChunk("VP8L", []byte("pixels"))),
	}
	for kind, data := range samples {
		for n := range len(data) {
			_, _ = stripMetadata(kind, data[:n])
		}
		for i := range data {
			for _, b := range []byte{0x00, 0xFF, 0x7F, 0x80} {
				mutated := append([]byte{}, data...)
				mutated[i] = b
				_, _ = stripMetadata(kind, mutated)
			}
		}
	}
}

func FuzzStripMetadata(f *testing.F) {
	var jb, pb bytes.Buffer
	_ = jpeg.Encode(&jb, testPicture(), nil)
	_ = png.Encode(&pb, testPicture())
	f.Add(jb.Bytes())
	f.Add(pb.Bytes())
	f.Add(webpFile(webpChunk("VP8X", make([]byte, 10)), webpChunk("EXIF", []byte("e")), webpChunk("VP8L", []byte("pixels"))))
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x02, 0xFF, 0xDA})

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, kind := range []imageType{imageJPEG, imagePNG, imageWebP} {
			out, err := stripMetadata(kind, data)
			if err == nil && len(out) > len(data) {
				// Stripping only ever removes bytes; the one addition is a 34-byte orientation segment.
				if len(out) > len(data)+34 {
					t.Fatalf("%v: output (%d) grew far beyond input (%d)", kind, len(out), len(data))
				}
			}
		}
	})
}
