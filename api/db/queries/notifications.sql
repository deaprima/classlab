-- name: CreateNotification :exec
INSERT INTO notifications (user_id, type, title, body, ref_type, ref_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- One statement for every active holder of a role, so a new booking cannot reach only some approvers.
-- name: CreateNotificationForRole :exec
INSERT INTO notifications (user_id, type, title, body, ref_type, ref_id, created_at)
SELECT id, sqlc.arg(type), sqlc.arg(title), sqlc.arg(body), sqlc.arg(ref_type), sqlc.arg(ref_id), sqlc.arg(created_at)
FROM users
WHERE role = sqlc.arg(role)::user_role AND is_active;
