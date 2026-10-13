-- name: GetUserByID :one
SELECT id, org_unit_id, identifier, name, email, password_hash, role, is_active, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByIdentifier :one
SELECT id, org_unit_id, identifier, name, email, password_hash, role, is_active, created_at, updated_at
FROM users
WHERE identifier = $1;
