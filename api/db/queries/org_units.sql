-- name: GetOrgUnitByNameAndType :one
SELECT id, parent_id, type, name, created_at, updated_at
FROM org_units
WHERE name = $1 AND type = $2
LIMIT 1;

-- name: CreateOrgUnit :one
INSERT INTO org_units (type, name, created_at, updated_at)
VALUES ($1, $2, $3, $3)
RETURNING id, parent_id, type, name, created_at, updated_at;

-- name: GetOrgUnitNameByID :one
SELECT name FROM org_units WHERE id = $1;
