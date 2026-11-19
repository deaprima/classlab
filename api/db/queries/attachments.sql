-- name: InsertAttachment :one
INSERT INTO attachments (uploaded_by, kind, original_name, storage_path, content_type, size_bytes, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, uploaded_by, kind, original_name, storage_path, content_type, size_bytes, purged_at, created_at;

-- A download needs the file's own row and the room types of everything that refers to it: the
-- people who may open a file are decided by what the file is attached to (BR-16, BR-19), never by
-- guessing from its name. The type arrays are cast to text[] (see the note in room_types.sql).
-- name: GetAttachmentAccess :one
SELECT a.id, a.uploaded_by, a.kind, a.original_name, a.storage_path, a.content_type, a.size_bytes, a.purged_at, a.created_at,
  COALESCE((
    SELECT array_agg(DISTINCT r.type::text)
    FROM bookings b JOIN rooms r ON r.id = b.room_id
    WHERE b.letter_attachment_id = a.id OR b.checkout_attachment_id = a.id
  ), '{}')::text[] AS booking_room_types,
  COALESCE((
    SELECT array_agg(DISTINCT r.type::text)
    FROM damage_reports d JOIN rooms r ON r.id = d.room_id
    WHERE d.attachment_id = a.id
  ), '{}')::text[] AS report_room_types
FROM attachments a
WHERE a.id = $1;

-- A requester's own files, for picking a letter again (a letter may back several bookings).
-- name: ListAttachmentsByUploader :many
SELECT id, kind, original_name, content_type, size_bytes, created_at
FROM attachments
WHERE uploaded_by = $1 AND kind = sqlc.arg(kind)::attachment_kind AND purged_at IS NULL
ORDER BY created_at DESC, id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountAttachmentsByUploader :one
SELECT count(*) FROM attachments
WHERE uploaded_by = $1 AND kind = sqlc.arg(kind)::attachment_kind AND purged_at IS NULL;

-- Used when a booking names a letter: it must exist, be a letter, be the requester's own and not be purged.
-- name: GetLetterForRequester :one
SELECT id, uploaded_by, kind, purged_at FROM attachments WHERE id = $1;

-- Files due for deletion (BR-13 c, BR-16, BR-19), locked so two instances of the job do not both take them.
--  * orphans: older than orphan_before and referred to by nothing;
--  * photos older than photo_before, unless a report that is not SELESAI still shows them;
--  * letters older than letter_before, unless a live booking (MENUNGGU, DISETUJUI, BERLANGSUNG) still needs them.
-- name: LockPurgeableAttachments :many
SELECT a.id, a.storage_path
FROM attachments a
WHERE a.purged_at IS NULL
  AND (
    (a.created_at < sqlc.arg(orphan_before)
      AND NOT EXISTS (SELECT 1 FROM bookings b WHERE b.letter_attachment_id = a.id OR b.checkout_attachment_id = a.id)
      AND NOT EXISTS (SELECT 1 FROM damage_reports d WHERE d.attachment_id = a.id))
    OR (a.kind = 'PHOTO' AND a.created_at < sqlc.arg(photo_before)
      AND NOT EXISTS (SELECT 1 FROM damage_reports d WHERE d.attachment_id = a.id AND d.status <> 'SELESAI'))
    OR (a.kind = 'LETTER' AND a.created_at < sqlc.arg(letter_before)
      AND NOT EXISTS (SELECT 1 FROM bookings b WHERE b.letter_attachment_id = a.id AND b.status IN ('MENUNGGU', 'DISETUJUI', 'BERLANGSUNG')))
  )
ORDER BY a.created_at, a.id
LIMIT sqlc.arg(batch)
FOR UPDATE OF a SKIP LOCKED;

-- name: MarkAttachmentsPurged :exec
UPDATE attachments SET purged_at = sqlc.arg(now) WHERE id = ANY(sqlc.arg(ids)::uuid[]);
