-- pg_notify runs in the same statement as the insert, and Postgres delivers it only when the
-- transaction commits. So the live stream hears about a notification exactly when it becomes
-- visible, and never about one whose transaction rolled back. Payload: "<user_id>:<notification_id>".

-- name: CreateNotification :exec
WITH n AS (
  INSERT INTO notifications (user_id, type, title, body, ref_type, ref_id, created_at)
  VALUES ($1, $2, $3, $4, $5, $6, $7)
  RETURNING id, user_id
)
SELECT pg_notify('classlab_notifications', user_id::text || ':' || id::text) FROM n;

-- One statement for every active holder of a role, so a new booking cannot reach only some approvers.
-- exclude_user_id keeps people from being notified about what they did themselves.
-- name: CreateNotificationForRole :exec
WITH n AS (
  INSERT INTO notifications (user_id, type, title, body, ref_type, ref_id, created_at)
  SELECT id, sqlc.arg(type), sqlc.arg(title), sqlc.arg(body), sqlc.arg(ref_type), sqlc.arg(ref_id), sqlc.arg(created_at)
  FROM users
  WHERE role = sqlc.arg(role)::user_role AND is_active
    AND (sqlc.narg(exclude_user_id)::uuid IS NULL OR id <> sqlc.narg(exclude_user_id)::uuid)
  RETURNING id, user_id
)
SELECT pg_notify('classlab_notifications', user_id::text || ':' || id::text) FROM n;

-- name: ListNotifications :many
SELECT id, user_id, type, title, body, ref_type, ref_id, read_at, created_at
FROM notifications
WHERE user_id = sqlc.arg(user_id)
  AND (NOT sqlc.arg(unread_only)::boolean OR read_at IS NULL)
ORDER BY created_at DESC, id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountNotifications :one
SELECT count(*)
FROM notifications
WHERE user_id = sqlc.arg(user_id)
  AND (NOT sqlc.arg(unread_only)::boolean OR read_at IS NULL);

-- name: GetNotificationForUser :one
SELECT id, user_id, type, title, body, ref_type, ref_id, read_at, created_at
FROM notifications
WHERE id = $1 AND user_id = $2;

-- Marking an already-read notification keeps its original read_at.
-- name: MarkNotificationRead :one
UPDATE notifications SET read_at = COALESCE(read_at, $3)
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, type, title, body, ref_type, ref_id, read_at, created_at;

-- name: MarkAllNotificationsRead :execrows
UPDATE notifications SET read_at = $2
WHERE user_id = $1 AND read_at IS NULL;
