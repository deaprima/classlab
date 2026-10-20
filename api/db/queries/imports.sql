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
