-- name: GetUserByID :one
SELECT id, org_unit_id, identifier, name, email, role, is_active, booking_blocked_until,
       pic_name, pic_phone, must_change_password, created_at, updated_at
FROM users
WHERE id = $1;

-- Credentials lookup is the only query that returns password_hash.
-- name: GetUserForLogin :one
SELECT id, org_unit_id, identifier, name, email, password_hash, role, is_active, booking_blocked_until,
       pic_name, pic_phone, must_change_password, created_at, updated_at
FROM users
WHERE identifier = $1;

-- The current password is checked against this hash when a user changes it.
-- name: GetPasswordHash :one
SELECT password_hash FROM users WHERE id = $1;

-- name: ListUsers :many
SELECT id, org_unit_id, identifier, name, email, role, is_active, booking_blocked_until,
       pic_name, pic_phone, must_change_password, created_at, updated_at
FROM users
WHERE (sqlc.narg(role)::user_role IS NULL OR role = sqlc.narg(role)::user_role)
  AND (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active)::boolean)
  AND (
    sqlc.narg(q)::text IS NULL
    OR name ILIKE '%' || sqlc.narg(q)::text || '%'
    OR identifier ILIKE '%' || sqlc.narg(q)::text || '%'
    OR email ILIKE '%' || sqlc.narg(q)::text || '%'
  )
ORDER BY created_at DESC, id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountUsers :one
SELECT count(*)
FROM users
WHERE (sqlc.narg(role)::user_role IS NULL OR role = sqlc.narg(role)::user_role)
  AND (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active)::boolean)
  AND (
    sqlc.narg(q)::text IS NULL
    OR name ILIKE '%' || sqlc.narg(q)::text || '%'
    OR identifier ILIKE '%' || sqlc.narg(q)::text || '%'
    OR email ILIKE '%' || sqlc.narg(q)::text || '%'
  );

-- name: CreateUser :one
INSERT INTO users (org_unit_id, identifier, name, email, password_hash, role, pic_name, pic_phone, must_change_password, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
RETURNING id, org_unit_id, identifier, name, email, role, is_active, booking_blocked_until,
          pic_name, pic_phone, must_change_password, created_at, updated_at;

-- clear_booking_block is a flag because "leave unchanged" and "set to NULL" cannot both be NULL.
-- name: UpdateUser :one
UPDATE users SET
  name           = COALESCE(sqlc.narg(name), name),
  email          = COALESCE(sqlc.narg(email), email),
  role           = COALESCE(sqlc.narg(role)::user_role, role),
  is_active      = COALESCE(sqlc.narg(is_active), is_active),
  pic_name       = COALESCE(sqlc.narg(pic_name), pic_name),
  pic_phone      = COALESCE(sqlc.narg(pic_phone), pic_phone),
  booking_blocked_until = CASE WHEN sqlc.arg(clear_booking_block)::boolean THEN NULL ELSE booking_blocked_until END,
  updated_at     = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
RETURNING id, org_unit_id, identifier, name, email, role, is_active, booking_blocked_until,
          pic_name, pic_phone, must_change_password, created_at, updated_at;

-- Used by change-password and reset-password; must_change_password says who chose the new password.
-- name: SetUserPassword :exec
UPDATE users SET password_hash = $2, must_change_password = $3, updated_at = $4 WHERE id = $1;

-- name: SeedUser :execrows
INSERT INTO users (org_unit_id, identifier, name, email, password_hash, role, pic_name, pic_phone, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
ON CONFLICT (identifier) DO NOTHING;

-- name: CountActiveUsersByIdentifiers :one
SELECT count(*) FROM users WHERE identifier = ANY(sqlc.arg(identifiers)::text[]) AND is_active;

-- Taken before an account change that could remove the last administrator, so two such changes made
-- at the same moment cannot each see "the other one is still there".
-- name: LockAdminGuard :exec
SELECT pg_advisory_xact_lock(hashtextextended('admin-guard', 0));

-- name: CountActiveAdmins :one
SELECT count(*) FROM users WHERE role = 'ADMIN' AND is_active;

-- Row lock for changes that read the account first (before-state for the audit log, last-admin check).
-- name: LockUser :one
SELECT id, org_unit_id, identifier, name, email, role, is_active, booking_blocked_until,
       pic_name, pic_phone, must_change_password, created_at, updated_at
FROM users
WHERE id = $1
FOR UPDATE;
