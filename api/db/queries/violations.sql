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

-- name: ListUserViolations :many
SELECT v.id, v.user_id, v.booking_id, v.kind, v.points, v.annulled_at, v.annulled_by, v.annul_reason, v.created_at,
       r.code AS room_code, b.start_at AS booking_start_at
FROM violations v
JOIN bookings b ON b.id = v.booking_id
JOIN rooms r ON r.id = b.room_id
WHERE v.user_id = $1
ORDER BY v.created_at DESC, v.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountUserViolations :one
SELECT count(*) FROM violations WHERE user_id = $1;

-- name: LockViolation :one
SELECT * FROM violations WHERE id = $1 FOR UPDATE;

-- name: AnnulViolation :one
UPDATE violations SET annulled_at = sqlc.arg(now), annulled_by = sqlc.arg(annulled_by), annul_reason = sqlc.arg(reason)
WHERE id = sqlc.arg(id) AND annulled_at IS NULL
RETURNING *;
