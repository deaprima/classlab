// Package attachments stores the files people upload: letters (PDF or image) and photos (check-out and
// damage reports). Every file is checked by its magic bytes, images are decoded and written again as
// JPEG so metadata and any payload hidden in the original are gone, and a file is served only through an
// endpoint that decides who may open it (brief BR-16, BR-19).
package attachments

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // registers the PNG decoder
	"math"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers the WebP decoder
)

const (
	// maxImageSide is the longest side kept after processing (BR-16).
	maxImageSide = 1600
	// maxDecodePixels refuses a small file that expands into a huge bitmap (a "decompression bomb")
	// before it is decoded. 40 megapixels is above any phone camera.
	maxDecodePixels = 40_000_000
	jpegQuality     = 85
)

var (
	// ErrNotImage means the bytes are not a JPEG, PNG or WebP.
	ErrNotImage = errors.New("not a supported image")
	// ErrBrokenImage means the file claims to be an image but cannot be decoded.
	ErrBrokenImage = errors.New("malformed image")
	// ErrImageTooLarge means the picture has more pixels than we are willing to decode.
	ErrImageTooLarge = errors.New("image has too many pixels")
)

// isImage recognises JPEG, PNG and WebP by their first bytes; the name and content type a client sends
// are never trusted.
func isImage(head []byte) bool {
	switch {
	case bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF}):
		return true
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		return true
	case len(head) >= 12 && bytes.Equal(head[0:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")):
		return true
	}
	return false
}

// isPDF reports whether data starts like a PDF. That is all that is checked: the file is stored as it
// came and never parsed, rendered or executed on the server (BR-19).
func isPDF(head []byte) bool { return bytes.HasPrefix(head, []byte("%PDF-")) }

// processImage turns an uploaded picture into a JPEG with its longest side at most maxImageSide:
// decode, shrink, apply the EXIF orientation to the pixels, flatten transparency on white, encode.
// Nothing of the original container survives, so EXIF (including the GPS position), XMP, comments and
// trailing bytes are gone.
func processImage(data []byte) ([]byte, error) {
	if !isImage(data) {
		return nil, ErrNotImage
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBrokenImage, err)
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > maxDecodePixels || cfg.Width > 1<<15 || cfg.Height > 1<<15 {
		return nil, ErrImageTooLarge
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBrokenImage, err)
	}

	// Shrink first: orienting and flattening then work on at most 2.5 megapixels.
	src = shrink(src)
	if o := jpegOrientation(data); o > 1 {
		src = orient(src, o)
	}

	b := src.Bounds()
	canvas := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), src, b.Min, draw.Over)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, canvas, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("encoding jpeg: %w", err)
	}
	return out.Bytes(), nil
}

// shrink scales the picture down so its longest side is at most maxImageSide; smaller pictures are not enlarged.
func shrink(src image.Image) image.Image {
	b := src.Bounds()
	longest := max(b.Dx(), b.Dy())
	if longest <= maxImageSide {
		return src
	}
	scale := float64(maxImageSide) / float64(longest)
	w := max(1, int(math.Round(float64(b.Dx())*scale)))
	h := max(1, int(math.Round(float64(b.Dy())*scale)))
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}

// orient moves the pixels as the EXIF orientation value (2 to 8) says, so the picture is upright without
// the tag. Go's JPEG decoder ignores the tag, and re-encoding drops it, so without this a photo taken with
// the phone held upright would be stored on its side.
func orient(src image.Image, o int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 { // the four transposed/rotated cases swap the axes
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2: // mirrored horizontally
				dx, dy = w-1-x, y
			case 3: // rotated 180
				dx, dy = w-1-x, h-1-y
			case 4: // mirrored vertically
				dx, dy = x, h-1-y
			case 5: // transposed
				dx, dy = y, x
			case 6: // rotated 90 clockwise
				dx, dy = h-1-y, x
			case 7: // transverse
				dx, dy = h-1-y, w-1-x
			case 8: // rotated 90 counter-clockwise
				dx, dy = y, w-1-x
			default:
				dx, dy = x, y
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// jpegOrientation finds the EXIF orientation of a JPEG, or 0 when the file is not a JPEG or has none.
// It only reads framing and never trusts a length beyond the data it was given.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 0
		}
		marker := data[i+1]
		if marker == 0xFF { // fill byte
			i++
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // start of scan or end of image: no more headers
			return 0
		}
		length := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if length < 2 || i+2+length > len(data) {
			return 0
		}
		if marker == 0xE1 {
			if o := exifOrientation(data[i+4 : i+2+length]); o != 0 {
				return o
			}
		}
		i += 2 + length
	}
	return 0
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
