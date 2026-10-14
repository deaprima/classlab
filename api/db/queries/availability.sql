-- Half-open ranges [start, end): two ranges overlap iff each starts before the other ends,
-- so 08:00-10:00 and 10:00-12:00 do not collide (BR-02).

-- name: ListSchedulesOverlapping :many
SELECT id, room_id, title, start_at, end_at
FROM room_schedules
WHERE room_id = ANY(sqlc.arg(room_ids)::uuid[])
  AND start_at < sqlc.arg(range_end)
  AND end_at > sqlc.arg(range_start)
ORDER BY start_at, id;

-- name: ListBookingsOverlapping :many
SELECT id, room_id, purpose, status, start_at, end_at
FROM bookings
WHERE room_id = ANY(sqlc.arg(room_ids)::uuid[])
  AND status::text = ANY(sqlc.arg(statuses)::text[])
  AND start_at < sqlc.arg(range_end)
  AND end_at > sqlc.arg(range_start)
ORDER BY start_at, id;

-- name: ListBookableRooms :many
SELECT id, org_unit_id, code, name, type, capacity, facilities, is_active, deleted_at, created_at, updated_at
FROM rooms
WHERE deleted_at IS NULL AND is_active
  AND (sqlc.narg(type)::room_type IS NULL OR type = sqlc.narg(type)::room_type)
  AND (sqlc.narg(capacity_min)::int IS NULL OR capacity >= sqlc.narg(capacity_min)::int)
ORDER BY code, id;
