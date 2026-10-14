package reports

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// errBadImage means the file claims to be an image but its structure does not hold together.
var errBadImage = errors.New("malformed image")

// stripMetadata removes what a photo carries besides its picture: EXIF (which holds the GPS position
// the photo was taken at), XMP, IPTC and text comments. The picture data is copied byte for byte,
// never decoded or re-encoded, so quality is untouched and the work is a bounds-checked walk over the
// file's own framing. A file whose framing is broken is refused rather than stored half-understood.
func stripMetadata(kind imageType, data []byte) ([]byte, error) {
	switch kind {
	case imageJPEG:
		return stripJPEG(data)
	case imagePNG:
		return stripPNG(data)
	case imageWebP:
		return stripWebP(data)
	}
	return nil, errBadImage
}

// --- JPEG ------------------------------------------------------------------------------------

const (
	jpegSOI  = 0xD8
	jpegEOI  = 0xD9
	jpegSOS  = 0xDA
	jpegAPP0 = 0xE0
	jpegAPP1 = 0xE1
)

// dropsJPEGSegment reports whether a marker's segment is metadata we remove: EXIF/XMP (APP1), the
// other application segments (APP3-APP13 and APP15, e.g. IPTC) and comments. APP0 (JFIF), APP2
// (colour profile) and APP14 (Adobe colour transform) change how the picture is decoded, so they stay.
func dropsJPEGSegment(marker byte) bool {
	return marker == jpegAPP1 || (marker >= 0xE3 && marker <= 0xED) || marker == 0xEF || marker == 0xFE
}

// stripJPEG walks the segments up to the start of scan, drops metadata segments, and copies the scan
// (entropy-coded picture data through the end of the file) unchanged.
//
// Phones record how the camera was held only in the EXIF orientation tag. Dropping EXIF whole would
// show portrait photos on their side, so that one value is carried over in a minimal EXIF segment.
func stripJPEG(data []byte) ([]byte, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != jpegSOI {
		return nil, errBadImage
	}

	out := make([]byte, 0, len(data))
	out = append(out, 0xFF, jpegSOI)

	orientation := 0
	orientationWritten := false
	writeOrientation := func() {
		if !orientationWritten && orientation > 1 {
			out = append(out, orientationSegment(orientation)...)
		}
		orientationWritten = true
	}

	i := 2
	for {
		// Segments start with 0xFF, optionally preceded by fill bytes of 0xFF.
		if i >= len(data) || data[i] != 0xFF {
			return nil, errBadImage
		}
		for i < len(data) && data[i] == 0xFF {
			i++
		}
		if i >= len(data) {
			return nil, errBadImage
		}
		marker := data[i]
		i++

		switch {
		case marker == jpegEOI:
			return nil, errBadImage // the picture ended without a scan
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD8):
			// Standalone markers carry no length. They do not occur before a scan; copy and move on.
			writeOrientation()
			out = append(out, 0xFF, marker)
			continue
		}

		if i+2 > len(data) {
			return nil, errBadImage
		}
		length := int(binary.BigEndian.Uint16(data[i : i+2]))
		if length < 2 || i+length > len(data) {
			return nil, errBadImage
		}
		segment := data[i+2 : i+length] // the payload, without the length field

		if marker == jpegAPP1 && orientation == 0 {
			orientation = exifOrientation(segment)
		}
		if marker != jpegAPP0 {
			writeOrientation() // after JFIF, ahead of everything else
		}

		if marker == jpegSOS {
			// From here to the end of the file is picture data and the end marker; keep it as is.
			out = append(out, 0xFF, marker)
			out = append(out, data[i:]...)
			return out, nil
		}
		if !dropsJPEGSegment(marker) {
			out = append(out, 0xFF, marker)
			out = append(out, data[i:i+length]...)
		}
		i += length
	}
}

// exifOrientation reads the orientation (1-8) from an APP1 payload, or 0 if it is not EXIF or has none.
func exifOrientation(payload []byte) int {
	const header = "Exif\x00\x00"
	if !bytes.HasPrefix(payload, []byte(header)) {
		return 0
	}
	tiff := payload[len(header):]
	if len(tiff) < 8 {
		return 0
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return 0
	}
	ifd := int(order.Uint32(tiff[4:8]))
	if ifd < 8 || ifd+2 > len(tiff) {
		return 0
	}
	entries := int(order.Uint16(tiff[ifd : ifd+2]))
	for e := range entries {
		at := ifd + 2 + e*12
		if at+12 > len(tiff) {
			return 0
		}
		entry := tiff[at : at+12]
		// tag 0x0112, type SHORT (3), count 1: the value sits in the first two bytes of the value field.
		if order.Uint16(entry[0:2]) == 0x0112 && order.Uint16(entry[2:4]) == 3 && order.Uint32(entry[4:8]) == 1 {
			if v := int(order.Uint16(entry[8:10])); v >= 1 && v <= 8 {
				return v
			}
			return 0
		}
	}
	return 0
}

// orientationSegment is an APP1 segment holding an EXIF block with the orientation tag and nothing else.
func orientationSegment(orientation int) []byte {
	seg := []byte{
		0xFF, jpegAPP1, 0x00, 0x22, // marker, length 34 (this field, the header and the TIFF block)
		'E', 'x', 'i', 'f', 0, 0,
		'M', 'M', 0x00, 0x2A, 0, 0, 0, 8, // big-endian TIFF header, first IFD at offset 8
		0x00, 0x01, // one entry
		0x01, 0x12, 0x00, 0x03, 0, 0, 0, 1, 0, byte(orientation), 0, 0, // Orientation, SHORT, count 1
		0, 0, 0, 0, // no next IFD
	}
	return seg
}

// --- PNG -------------------------------------------------------------------------------------

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// dropsPNGChunk lists the ancillary chunks that carry metadata: EXIF, free text and the last-modified time.
func dropsPNGChunk(typ string) bool {
	switch typ {
	case "eXIf", "tEXt", "zTXt", "iTXt", "tIME":
		return true
	}
	return false
}

// stripPNG copies the chunks of a PNG except metadata ones. Each chunk is length, type, data and CRC;
// the CRCs of kept chunks stay valid because those chunks are not touched.
func stripPNG(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, pngSignature) {
		return nil, errBadImage
	}
	out := make([]byte, 0, len(data))
	out = append(out, pngSignature...)

	i := len(pngSignature)
	first, sawEnd := true, false
	for i < len(data) {
		if i+8 > len(data) {
			return nil, errBadImage
		}
		length := int(binary.BigEndian.Uint32(data[i : i+4]))
		typ := string(data[i+4 : i+8])
		end := i + 8 + length + 4
		if length < 0 || length > len(data) || end > len(data) || end < i {
			return nil, errBadImage
		}
		if first && typ != "IHDR" {
			return nil, errBadImage
		}
		first = false

		if !dropsPNGChunk(typ) {
			out = append(out, data[i:end]...)
		}
		i = end
		if typ == "IEND" {
			sawEnd = true
			break // anything after IEND is not part of the picture
		}
	}
	if !sawEnd {
		return nil, errBadImage
	}
	return out, nil
}

// --- WebP ------------------------------------------------------------------------------------

const (
	webpFlagEXIF = 0x08
	webpFlagXMP  = 0x04
)

// stripWebP copies the chunks of a RIFF/WebP container except EXIF and XMP, clears the matching flags
// in the VP8X header, and rewrites the RIFF size.
func stripWebP(data []byte) ([]byte, error) {
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil, errBadImage
	}
	declared := int(binary.LittleEndian.Uint32(data[4:8]))
	if declared < 4 || declared+8 > len(data) {
		return nil, errBadImage
	}
	data = data[:declared+8] // the container ends where its own size says, whatever follows is not ours

	out := make([]byte, 0, len(data))
	out = append(out, data[0:12]...)

	for i := 12; i < len(data); {
		if i+8 > len(data) {
			return nil, errBadImage
		}
		fourcc := string(data[i : i+4])
		size := int(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		end := i + 8 + size
		if size < 0 || end > len(data) || end < i {
			return nil, errBadImage
		}
		next := end + size%2 // chunks are padded to an even length
		if next > len(data) {
			return nil, errBadImage
		}

		switch fourcc {
		case "EXIF", "XMP ":
			// dropped
		case "VP8X":
			if size < 10 {
				return nil, errBadImage
			}
			chunk := append([]byte(nil), data[i:next]...)
			chunk[8] &^= webpFlagEXIF | webpFlagXMP // first payload byte holds the feature flags
			out = append(out, chunk...)
		default:
			out = append(out, data[i:next]...)
		}
		i = next
	}

	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out, nil
}
