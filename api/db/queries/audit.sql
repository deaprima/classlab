-- name: InsertAuditLog :exec
INSERT INTO audit_logs (actor_id, actor_role, action, entity_type, entity_id, before, after, ip, user_agent, request_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: ListAuditLogs :many
SELECT a.id, a.actor_id, a.actor_role, u.identifier AS actor_identifier, u.name AS actor_name,
       a.action, a.entity_type, a.entity_id, a.before, a.after, a.ip, a.user_agent, a.request_id, a.created_at
FROM audit_logs a
LEFT JOIN users u ON u.id = a.actor_id
WHERE (sqlc.narg(actor_id)::uuid IS NULL OR a.actor_id = sqlc.narg(actor_id)::uuid)
  AND (sqlc.narg(action)::text IS NULL OR a.action = sqlc.narg(action)::text)
  AND (sqlc.narg(entity_type)::text IS NULL OR a.entity_type = sqlc.narg(entity_type)::text)
  AND (sqlc.narg(entity_id)::uuid IS NULL OR a.entity_id = sqlc.narg(entity_id)::uuid)
  AND (sqlc.narg(from_at)::timestamptz IS NULL OR a.created_at >= sqlc.narg(from_at)::timestamptz)
  AND (sqlc.narg(to_at)::timestamptz IS NULL OR a.created_at < sqlc.narg(to_at)::timestamptz)
ORDER BY a.created_at DESC, a.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountAuditLogs :one
SELECT count(*)
FROM audit_logs a
WHERE (sqlc.narg(actor_id)::uuid IS NULL OR a.actor_id = sqlc.narg(actor_id)::uuid)
  AND (sqlc.narg(action)::text IS NULL OR a.action = sqlc.narg(action)::text)
  AND (sqlc.narg(entity_type)::text IS NULL OR a.entity_type = sqlc.narg(entity_type)::text)
  AND (sqlc.narg(entity_id)::uuid IS NULL OR a.entity_id = sqlc.narg(entity_id)::uuid)
  AND (sqlc.narg(from_at)::timestamptz IS NULL OR a.created_at >= sqlc.narg(from_at)::timestamptz)
  AND (sqlc.narg(to_at)::timestamptz IS NULL OR a.created_at < sqlc.narg(to_at)::timestamptz);
