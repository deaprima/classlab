-- Check-in and check-out (BR-11, BR-12). The booking row is locked before its status changes, so a
-- check-in cannot race a cancellation or a job (brief 4.4, point 5).

-- The QR is matched by room code. qr_token leaves the database only here and in GetRoomQR.
-- name: GetRoomForCheckin :one
SELECT id, code, type, is_active, qr_token
FROM rooms
WHERE lower(code) = lower($1) AND deleted_at IS NULL
ORDER BY created_at, id
LIMIT 1;

-- name: GetRoomQR :one
SELECT id, code, type, qr_token, qr_rotated_at
FROM rooms
WHERE id = $1 AND deleted_at IS NULL;

-- name: LockRoomQR :one
SELECT id, code, type, qr_token, qr_rotated_at
FROM rooms
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE;

-- name: RotateRoomQR :one
UPDATE rooms SET qr_token = sqlc.arg(qr_token), qr_rotated_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING id, code, type, qr_token, qr_rotated_at;

-- Approved bookings of the user in the room whose check-in window contains now, i.e.
-- start_at - early <= now <= start_at + late, written as start_at in [now - late, now + early].
-- name: LockCheckinCandidates :many
SELECT * FROM bookings
WHERE requester_id = sqlc.arg(requester_id) AND room_id = sqlc.arg(room_id) AND status = 'DISETUJUI'
  AND start_at >= sqlc.arg(window_from) AND start_at <= sqlc.arg(window_to)
ORDER BY start_at, id
FOR UPDATE;

-- The next approved booking of the user in the room whose window has not opened yet, for the
-- NO_ELIGIBLE_BOOKING details.
-- name: NextCheckinWindowStart :one
SELECT start_at FROM bookings
WHERE requester_id = $1 AND room_id = $2 AND status = 'DISETUJUI' AND start_at > $3
ORDER BY start_at
LIMIT 1;

-- name: CheckInBooking :one
UPDATE bookings SET status = 'BERLANGSUNG', checked_in_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- What the room looked like when the previous user left it (BR-11).
-- name: LatestCheckoutOfRoom :one
SELECT id, checked_out_at, checkout_condition, checkout_attachment_id
FROM bookings
WHERE room_id = $1 AND id <> $2 AND checked_out_at IS NOT NULL AND checkout_attachment_id IS NOT NULL
ORDER BY checked_out_at DESC, id
LIMIT 1;

-- name: LockBooking :one
SELECT * FROM bookings WHERE id = $1 FOR UPDATE;

-- name: CheckOutBooking :one
UPDATE bookings SET
  status = 'SELESAI', checked_out_at = sqlc.arg(now), updated_at = sqlc.arg(now),
  checkout_condition = sqlc.arg(condition)::checkout_condition,
  checkout_note = sqlc.narg(note),
  checkout_attachment_id = sqlc.arg(attachment_id)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListCheckouts :many
SELECT b.id AS booking_id, b.room_id, r.code AS room_code, r.name AS room_name, r.type AS room_type,
       b.checked_out_at, b.checkout_condition, b.checkout_note, b.checkout_attachment_id
FROM bookings b
JOIN rooms r ON r.id = b.room_id
WHERE b.checked_out_at IS NOT NULL
  AND (sqlc.narg(room_id)::uuid IS NULL OR b.room_id = sqlc.narg(room_id)::uuid)
  AND (sqlc.narg(range_start)::timestamptz IS NULL OR b.checked_out_at >= sqlc.narg(range_start)::timestamptz)
  AND (sqlc.narg(range_end)::timestamptz IS NULL OR b.checked_out_at < sqlc.narg(range_end)::timestamptz)
  AND (sqlc.narg(room_types)::text[] IS NULL OR r.type::text = ANY(sqlc.narg(room_types)::text[]))
ORDER BY b.checked_out_at DESC, b.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountCheckouts :one
SELECT count(*)
FROM bookings b
JOIN rooms r ON r.id = b.room_id
WHERE b.checked_out_at IS NOT NULL
  AND (sqlc.narg(room_id)::uuid IS NULL OR b.room_id = sqlc.narg(room_id)::uuid)
  AND (sqlc.narg(range_start)::timestamptz IS NULL OR b.checked_out_at >= sqlc.narg(range_start)::timestamptz)
  AND (sqlc.narg(range_end)::timestamptz IS NULL OR b.checked_out_at < sqlc.narg(range_end)::timestamptz)
  AND (sqlc.narg(room_types)::text[] IS NULL OR r.type::text = ANY(sqlc.narg(room_types)::text[]));
