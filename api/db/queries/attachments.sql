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
