package attachments

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// storedPath is the only shape of path the store reads or writes: a known folder, a UUID and a known
// extension. Paths come from the database, so this also stops a tampered row from escaping the upload
// directory. "reports/" holds photos written before attachments existed and is read-only history.
var storedPath = regexp.MustCompile(`^(attachments|reports)/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|webp|pdf)$`)

var errBadPath = errors.New("stored attachment path has an unexpected shape")

// Storage keeps files in a directory outside any web root; they are served only through the
// authenticated attachments endpoint.
type Storage struct {
	dir string
}

func NewStorage(uploadDir string) *Storage { return &Storage{dir: uploadDir} }

// Save writes a new file. O_EXCL means a name is never reused, so a stored file is never overwritten.
func (s *Storage) Save(rel string, data []byte) error {
	if !storedPath.MatchString(rel) {
		return errBadPath
	}
	full := filepath.Join(s.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return fmt.Errorf("creating upload directory: %w", err)
	}
	f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return fmt.Errorf("creating attachment file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		closeErr := f.Close()
		return errors.Join(fmt.Errorf("writing attachment file: %w", err), closeErr, os.Remove(full))
	}
	if err := f.Close(); err != nil {
		return errors.Join(fmt.Errorf("closing attachment file: %w", err), os.Remove(full))
	}
	return nil
}

// Open returns the file for reading; the caller closes it.
func (s *Storage) Open(rel string) (*os.File, error) {
	if !storedPath.MatchString(rel) {
		return nil, errBadPath
	}
	return os.Open(filepath.Join(s.dir, filepath.FromSlash(rel)))
}

// Remove deletes a file; a file that is already gone is not an error.
func (s *Storage) Remove(rel string) error {
	if !storedPath.MatchString(rel) {
		return errBadPath
	}
	if err := os.Remove(filepath.Join(s.dir, filepath.FromSlash(rel))); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing attachment file: %w", err)
	}
	return nil
}
