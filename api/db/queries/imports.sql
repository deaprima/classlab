-- name: CreateImport :one
INSERT INTO imports (kind, uploaded_by, file_name, file_sha256, semester, summary, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- A file that was applied and not rolled back cannot be applied again (BR-23, BR-24).
-- name: ImportAlreadyApplied :one
SELECT EXISTS (
  SELECT 1 FROM imports WHERE kind = $1 AND file_sha256 = $2 AND status = 'COMMITTED'
);

-- name: ListImports :many
SELECT i.id, i.kind, i.uploaded_by, u.identifier AS uploader_identifier, u.name AS uploader_name,
       i.file_name, i.semester, i.status, i.summary, i.created_at, i.rolled_back_at
FROM imports i
JOIN users u ON u.id = i.uploaded_by
WHERE (sqlc.narg(kind)::import_kind IS NULL OR i.kind = sqlc.narg(kind)::import_kind)
ORDER BY i.created_at DESC, i.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountImports :one
SELECT count(*) FROM imports i
WHERE (sqlc.narg(kind)::import_kind IS NULL OR i.kind = sqlc.narg(kind)::import_kind);

-- Serialises student imports so two of them cannot each plan "create" for the same identifier.
-- name: LockUserImport :exec
SELECT pg_advisory_xact_lock(hashtextextended('import-users', 0));

-- Existing accounts for a whole file in one round trip, so planning does not cost a query per row.
-- name: ListUsersByIdentifiers :many
SELECT id, identifier, role, is_active
FROM users
WHERE identifier = ANY(sqlc.arg(identifiers)::text[]);

-- name: ListUsersByEmails :many
SELECT id, identifier, lower(email)::text AS email
FROM users
WHERE lower(email) = ANY(sqlc.arg(emails)::text[]);

-- An import updates the name and email of an existing student and reactivates the account; it never
-- touches the password (BR-24). An empty email in the file leaves the stored one alone.
-- name: UpdateImportedStudent :exec
UPDATE users SET
  name       = sqlc.arg(name),
  email      = COALESCE(sqlc.narg(email), email),
  is_active  = true,
  updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND role = 'MAHASISWA';

-- name: CreateUserCourse :exec
INSERT INTO user_courses (user_id, import_id, semester, course_code, course_name, class_name, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- Rooms a schedule file refers to, by the code printed on the door. Inactive rooms come back too so the
-- import can say "inactive" rather than "unknown"; deleted ones do not.
-- name: ListRoomsByCodes :many
SELECT id, code, is_active
FROM rooms
WHERE org_unit_id = sqlc.arg(org_unit_id) AND code = ANY(sqlc.arg(codes)::text[]) AND deleted_at IS NULL;

-- Everything already in the way, for all rooms of the file at once. Both lists are ordered by start so the
-- import can search them instead of comparing every pair.
-- name: ListSchedulesInRange :many
SELECT id, room_id, title, source, start_at, end_at
FROM room_schedules
WHERE room_id = ANY(sqlc.arg(room_ids)::uuid[]) AND start_at < sqlc.arg(range_end) AND end_at > sqlc.arg(range_start)
ORDER BY room_id, start_at;

-- name: ListLiveBookingsInRange :many
SELECT id, room_id, requester_id, status, purpose, start_at, end_at
FROM bookings
WHERE room_id = ANY(sqlc.arg(room_ids)::uuid[])
  AND status IN ('MENUNGGU', 'DISETUJUI', 'BERLANGSUNG')
  AND start_at < sqlc.arg(range_end) AND end_at > sqlc.arg(range_start)
ORDER BY room_id, start_at;

-- name: CreateImportedSchedule :exec
INSERT INTO room_schedules (room_id, title, start_at, end_at, source, created_by, import_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, 'AKADEMIK', $5, $6, $7, $7);

-- An academic schedule always wins over a request (BR-23). Only a booking still waiting or approved is
-- cancelled; one that is under way is an import error and never reaches this statement.
-- name: CancelBookingForImport :execrows
UPDATE bookings SET
  status = 'DIBATALKAN', decided_by = sqlc.arg(decided_by), decided_at = sqlc.arg(now),
  decision_reason = sqlc.arg(reason), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status IN ('MENUNGGU', 'DISETUJUI');

-- name: GetImportForUpdate :one
SELECT id, kind, status FROM imports WHERE id = $1 FOR UPDATE;

-- Rollback removes what has not started; meetings that already took place stay as history.
-- name: DeleteUpcomingImportSchedules :execrows
DELETE FROM room_schedules WHERE import_id = $1 AND start_at > $2;

-- name: MarkImportRolledBack :exec
UPDATE imports SET status = 'ROLLED_BACK', rolled_back_at = $2, rolled_back_by = $3 WHERE id = $1;
