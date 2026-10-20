-- Points that still count: not annulled and inside the look-back window (BR-14, BR-17).
-- name: SumActiveViolationPoints :one
SELECT COALESCE(SUM(points), 0)::int
FROM violations
WHERE user_id = $1 AND annulled_at IS NULL AND created_at > $2;

-- One violation per (booking, kind): recording the same thing twice is a no-op, which is what keeps a job
-- that runs again from doubling the points (BR-13, BR-14).
-- name: InsertViolation :execrows
INSERT INTO violations (user_id, booking_id, kind, points, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (booking_id, kind) DO NOTHING;

-- Blocks the user until the given time unless an equal or later block already stands; the row count says
-- whether this call is the one that started a block (so the notification is sent once).
-- name: BlockUserUntil :execrows
UPDATE users SET booking_blocked_until = sqlc.arg(until), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND (booking_blocked_until IS NULL OR booking_blocked_until < sqlc.arg(until));
