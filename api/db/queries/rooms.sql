-- name: GetRoom :one
SELECT id, org_unit_id, code, name, type, capacity, facilities, is_active, deleted_at, created_at, updated_at, open_time, close_time
FROM rooms
WHERE id = $1 AND deleted_at IS NULL;

-- Every write that touches a room's bookings or schedules takes this lock first (brief 4.4).
-- name: LockRoom :one
SELECT id, org_unit_id, code, name, type, capacity, facilities, is_active, deleted_at, created_at, updated_at, open_time, close_time
FROM rooms
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE;

-- name: ListRooms :many
SELECT id, org_unit_id, code, name, type, capacity, facilities, is_active, deleted_at, created_at, updated_at, open_time, close_time
FROM rooms
WHERE deleted_at IS NULL
  AND (sqlc.narg(type)::room_type IS NULL OR type = sqlc.narg(type)::room_type)
  AND (sqlc.narg(capacity_min)::int IS NULL OR capacity >= sqlc.narg(capacity_min)::int)
  AND (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active)::boolean)
  AND (
    sqlc.narg(q)::text IS NULL
    OR code ILIKE '%' || sqlc.narg(q)::text || '%'
    OR name ILIKE '%' || sqlc.narg(q)::text || '%'
  )
ORDER BY code, id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountRooms :one
SELECT count(*)
FROM rooms
WHERE deleted_at IS NULL
  AND (sqlc.narg(type)::room_type IS NULL OR type = sqlc.narg(type)::room_type)
  AND (sqlc.narg(capacity_min)::int IS NULL OR capacity >= sqlc.narg(capacity_min)::int)
  AND (sqlc.narg(is_active)::boolean IS NULL OR is_active = sqlc.narg(is_active)::boolean)
  AND (
    sqlc.narg(q)::text IS NULL
    OR code ILIKE '%' || sqlc.narg(q)::text || '%'
    OR name ILIKE '%' || sqlc.narg(q)::text || '%'
  );

-- name: CreateRoom :one
INSERT INTO rooms (org_unit_id, code, name, type, capacity, facilities, qr_token, qr_rotated_at, created_at, updated_at, open_time, close_time)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $8, sqlc.narg(open_time), sqlc.narg(close_time))
RETURNING id, org_unit_id, code, name, type, capacity, facilities, is_active, deleted_at, created_at, updated_at, open_time, close_time;

-- name: UpdateRoom :one
UPDATE rooms SET
  name       = COALESCE(sqlc.narg(name), name),
  type       = COALESCE(sqlc.narg(type)::room_type, type),
  capacity   = COALESCE(sqlc.narg(capacity)::int, capacity),
  facilities = COALESCE(sqlc.narg(facilities)::text[], facilities),
  is_active  = COALESCE(sqlc.narg(is_active)::boolean, is_active),
  -- clear_operating_hours puts the room back on the type's default hours (BR-29).
  open_time  = CASE WHEN sqlc.arg(clear_operating_hours)::boolean THEN NULL ELSE COALESCE(sqlc.narg(open_time)::time, open_time) END,
  close_time = CASE WHEN sqlc.arg(clear_operating_hours)::boolean THEN NULL ELSE COALESCE(sqlc.narg(close_time)::time, close_time) END,
  updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING id, org_unit_id, code, name, type, capacity, facilities, is_active, deleted_at, created_at, updated_at, open_time, close_time;

-- name: SoftDeleteRoom :exec
UPDATE rooms SET deleted_at = $2, is_active = false, updated_at = $2
WHERE id = $1 AND deleted_at IS NULL;

-- BR-07: a room cannot be deleted while it still has a pending or approved booking that has not ended.
-- name: CountLiveBookingsForRoom :one
SELECT count(*)
FROM bookings
WHERE room_id = $1 AND status IN ('MENUNGGU', 'DISETUJUI', 'BERLANGSUNG') AND end_at > $2;

-- name: SeedRoom :execrows
INSERT INTO rooms (org_unit_id, code, name, type, capacity, facilities, qr_token, qr_rotated_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, $8)
ON CONFLICT (org_unit_id, code) WHERE deleted_at IS NULL DO NOTHING;

-- name: GetRoomByCode :one
SELECT id, org_unit_id, code, name, type, capacity, facilities, is_active, deleted_at, created_at, updated_at, open_time, close_time
FROM rooms
WHERE org_unit_id = $1 AND code = $2 AND deleted_at IS NULL;
