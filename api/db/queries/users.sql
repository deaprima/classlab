-- name: GetUserByID :one
SELECT id, org_unit_id, identifier, name, email, role, is_active, created_at, updated_at
FROM users
WHERE id = $1;

-- Credentials lookup is the only query that returns password_hash.
-- name: GetUserForLogin :one
SELECT id, org_unit_id, identifier, name, email, password_hash, role, is_active, created_at, updated_at
FROM users
WHERE identifier = $1;

-- name: ListUsers :many
SELECT id, org_unit_id, identifier, name, email, role, is_active, created_at, updated_at
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
INSERT INTO users (org_unit_id, identifier, name, email, password_hash, role, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
RETURNING id, org_unit_id, identifier, name, email, role, is_active, created_at, updated_at;

-- name: UpdateUser :one
UPDATE users SET
  name          = COALESCE(sqlc.narg(name), name),
  email         = COALESCE(sqlc.narg(email), email),
  role          = COALESCE(sqlc.narg(role)::user_role, role),
  is_active     = COALESCE(sqlc.narg(is_active), is_active),
  password_hash = COALESCE(sqlc.narg(password_hash), password_hash),
  updated_at    = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
RETURNING id, org_unit_id, identifier, name, email, role, is_active, created_at, updated_at;

-- name: SeedUser :execrows
INSERT INTO users (org_unit_id, identifier, name, email, password_hash, role, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
ON CONFLICT (identifier) DO NOTHING;

-- name: CountActiveUsersByIdentifiers :one
SELECT count(*) FROM users WHERE identifier = ANY(sqlc.arg(identifiers)::text[]) AND is_active;
