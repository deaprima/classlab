package reports

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// imageType is a file type the upload accepts. The type is decided from the file's own leading bytes;
// the file name and Content-Type the client sends are never trusted.
type imageType struct {
	ext         string
	contentType string
}

var (
	imageJPEG = imageType{".jpg", "image/jpeg"}
	imagePNG  = imageType{".png", "image/png"}
	imageWebP = imageType{".webp", "image/webp"}
)

// sniffImage recognises JPEG, PNG and WebP by their magic bytes.
func sniffImage(head []byte) (imageType, bool) {
	switch {
	case bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF}):
		return imageJPEG, true
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		return imagePNG, true
	case len(head) >= 12 && bytes.Equal(head[0:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")):
		return imageWebP, true
	}
	return imageType{}, false
}

// storedName is the only shape of file name the store reads or writes: a UUID plus a known extension.
// Names come from the database, so this also stops a tampered row from escaping the upload directory.
var storedName = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|webp)$`)

var errBadStoredName = errors.New("stored photo name has an unexpected shape")

// Storage keeps report photos in a directory outside any web root; they are served only through the
// authenticated photo endpoint.
type Storage struct {
	dir string
}

func NewStorage(uploadDir string) *Storage {
	return &Storage{dir: filepath.Join(uploadDir, "reports")}
}

func (s *Storage) Save(name string, data []byte) error {
	if !storedName.MatchString(name) {
		return errBadStoredName
	}
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return fmt.Errorf("creating upload directory: %w", err)
	}
	// O_EXCL: a name is never reused, so a stored photo can never be overwritten.
	f, err := os.OpenFile(filepath.Join(s.dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return fmt.Errorf("creating photo file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		closeErr := f.Close()
		return errors.Join(fmt.Errorf("writing photo file: %w", err), closeErr, os.Remove(f.Name()))
	}
	if err := f.Close(); err != nil {
		return errors.Join(fmt.Errorf("closing photo file: %w", err), os.Remove(f.Name()))
	}
	return nil
}

func (s *Storage) Open(name string) (*os.File, error) {
	if !storedName.MatchString(name) {
		return nil, errBadStoredName
	}
	return os.Open(filepath.Join(s.dir, name))
}

// Remove deletes a stored photo; a file that is already gone is not an error.
func (s *Storage) Remove(name string) error {
	if !storedName.MatchString(name) {
		return errBadStoredName
	}
	if err := os.Remove(filepath.Join(s.dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// imageForName maps a stored name's extension back to its image type.
func imageForName(name string) imageType {
	switch filepath.Ext(name) {
	case ".png":
		return imagePNG
	case ".webp":
		return imageWebP
	}
	return imageJPEG
}
