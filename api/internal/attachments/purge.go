package attachments

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// PurgePolicy says how old a file must be before it is deleted (BR-13 c, BR-16, BR-19).
type PurgePolicy struct {
	OrphanAge       time.Duration // a file nothing refers to
	PhotoRetention  time.Duration // a photo, unless a report that is not SELESAI still shows it
	LetterRetention time.Duration // a letter, unless a live booking still needs it
}

// PurgeExpired deletes the files that are due and marks them purged, so that opening one answers 410. It
// works in batches and returns how many files it removed.
//
// The rows are locked and marked purged in a transaction, and the files are deleted after it commits. A file
// whose deletion fails stays on disk and is logged with its path; it is never reachable again because the row
// says purged, which is the safe way round (deleting first could leave a row that points at nothing).
func (s *Service) PurgeExpired(ctx context.Context, now time.Time, p PurgePolicy, batch int) (int, error) {
	total := 0
	for {
		var paths []string
		err := s.pool.WithTx(ctx, func(tx pgx.Tx) error {
			q := s.q.WithTx(tx)
			rows, err := q.LockPurgeableAttachments(ctx, dbgen.LockPurgeableAttachmentsParams{
				OrphanBefore: now.Add(-p.OrphanAge), PhotoBefore: now.Add(-p.PhotoRetention), LetterBefore: now.Add(-p.LetterRetention),
				Batch: int32(batch),
			})
			if err != nil {
				return fmt.Errorf("selecting attachments to purge: %w", err)
			}
			if len(rows) == 0 {
				return nil
			}
			ids := make([]uuid.UUID, len(rows))
			for i, r := range rows {
				ids[i] = r.ID
				paths = append(paths, r.StoragePath)
			}
			if err := q.MarkAttachmentsPurged(ctx, dbgen.MarkAttachmentsPurgedParams{Now: pgtype.Timestamptz{Time: now, Valid: true}, Ids: ids}); err != nil {
				return fmt.Errorf("marking attachments purged: %w", err)
			}
			return nil
		})
		if err != nil {
			return total, err
		}
		for _, rel := range paths {
			if err := s.store.Remove(rel); err != nil {
				s.log.ErrorContext(ctx, "removing purged attachment file", slog.String("file", rel), slog.String("error", err.Error()))
			}
		}
		total += len(paths)
		if len(paths) < batch {
			return total, nil
		}
	}
}
