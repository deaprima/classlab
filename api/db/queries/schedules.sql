-- name: GetSchedule :one
SELECT id, room_id, title, start_at, end_at, source, created_by, created_at, updated_at
FROM room_schedules
WHERE id = $1;

-- name: CreateSchedule :one
INSERT INTO room_schedules (room_id, title, start_at, end_at, source, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
RETURNING id, room_id, title, start_at, end_at, source, created_by, created_at, updated_at;

-- name: UpdateSchedule :one
UPDATE room_schedules SET
  title      = COALESCE(sqlc.narg(title), title),
  start_at   = sqlc.arg(start_at),
  end_at     = sqlc.arg(end_at),
  updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id)
RETURNING id, room_id, title, start_at, end_at, source, created_by, created_at, updated_at;

-- name: DeleteSchedule :execrows
DELETE FROM room_schedules WHERE id = $1;

-- No conflict target: this also swallows the exclusion-constraint violation raised by a session
-- that is already there, which is what makes seeding repeatable.
-- name: SeedSchedule :execrows
INSERT INTO room_schedules (room_id, title, start_at, end_at, source, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, 'AKADEMIK', $5, $6, $6)
ON CONFLICT DO NOTHING;
