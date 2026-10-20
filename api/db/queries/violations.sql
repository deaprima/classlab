-- Points that still count: not annulled and inside the look-back window (BR-14, BR-17).
-- name: SumActiveViolationPoints :one
SELECT COALESCE(SUM(points), 0)::int
FROM violations
WHERE user_id = $1 AND annulled_at IS NULL AND created_at > $2;
