-- Reads that serve the API join the room and requester in one query, so list pages never go N+1.
-- name: GetBooking :one
SELECT b.id, b.room_id, b.requester_id, b.start_at, b.end_at, b.purpose, b.participants, b.notes,
       b.status, b.decided_by, b.decided_at, b.decision_reason, b.created_at,
       r.org_unit_id AS room_org_unit_id, r.code AS room_code, r.name AS room_name, r.type AS room_type,
       r.capacity AS room_capacity, r.facilities AS room_facilities, r.is_active AS room_is_active,
       u.identifier AS requester_identifier, u.name AS requester_name
FROM bookings b
JOIN rooms r ON r.id = b.room_id
JOIN users u ON u.id = b.requester_id
WHERE b.id = $1;

-- name: ListBookings :many
SELECT b.id, b.room_id, b.requester_id, b.start_at, b.end_at, b.purpose, b.participants, b.notes,
       b.status, b.decided_by, b.decided_at, b.decision_reason, b.created_at,
       r.org_unit_id AS room_org_unit_id, r.code AS room_code, r.name AS room_name, r.type AS room_type,
       r.capacity AS room_capacity, r.facilities AS room_facilities, r.is_active AS room_is_active,
       u.identifier AS requester_identifier, u.name AS requester_name
FROM bookings b
JOIN rooms r ON r.id = b.room_id
JOIN users u ON u.id = b.requester_id
WHERE (sqlc.narg(requester_id)::uuid IS NULL OR b.requester_id = sqlc.narg(requester_id)::uuid)
  AND (sqlc.narg(status)::booking_status IS NULL OR b.status = sqlc.narg(status)::booking_status)
  AND (sqlc.narg(room_id)::uuid IS NULL OR b.room_id = sqlc.narg(room_id)::uuid)
  AND (sqlc.narg(range_start)::timestamptz IS NULL OR b.end_at > sqlc.narg(range_start)::timestamptz)
  AND (sqlc.narg(range_end)::timestamptz IS NULL OR b.start_at < sqlc.narg(range_end)::timestamptz)
ORDER BY b.created_at DESC, b.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountBookings :one
SELECT count(*)
FROM bookings b
WHERE (sqlc.narg(requester_id)::uuid IS NULL OR b.requester_id = sqlc.narg(requester_id)::uuid)
  AND (sqlc.narg(status)::booking_status IS NULL OR b.status = sqlc.narg(status)::booking_status)
  AND (sqlc.narg(room_id)::uuid IS NULL OR b.room_id = sqlc.narg(room_id)::uuid)
  AND (sqlc.narg(range_start)::timestamptz IS NULL OR b.end_at > sqlc.narg(range_start)::timestamptz)
  AND (sqlc.narg(range_end)::timestamptz IS NULL OR b.start_at < sqlc.narg(range_end)::timestamptz);

-- Plain row, for the transition checks done after the room lock is held.
-- name: LoadBooking :one
SELECT id, room_id, requester_id, start_at, end_at, purpose, participants, notes, status,
       decided_by, decided_at, decision_reason, idempotency_key, created_at, updated_at
FROM bookings
WHERE id = $1;

-- name: GetBookingByIdempotencyKey :one
SELECT id, room_id, requester_id, start_at, end_at, purpose, participants, notes, status,
       decided_by, decided_at, decision_reason, idempotency_key, created_at, updated_at
FROM bookings
WHERE requester_id = $1 AND idempotency_key = $2;

-- name: CreateBooking :one
INSERT INTO bookings (room_id, requester_id, start_at, end_at, purpose, participants, notes, idempotency_key, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
RETURNING id, room_id, requester_id, start_at, end_at, purpose, participants, notes, status,
          decided_by, decided_at, decision_reason, idempotency_key, created_at, updated_at;

-- name: DecideBooking :one
UPDATE bookings SET
  status          = sqlc.arg(status)::booking_status,
  decided_by      = sqlc.arg(decided_by),
  decided_at      = sqlc.arg(now),
  decision_reason = sqlc.narg(decision_reason),
  updated_at      = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING id, room_id, requester_id, start_at, end_at, purpose, participants, notes, status,
          decided_by, decided_at, decision_reason, idempotency_key, created_at, updated_at;

-- name: CreateBookingEvent :exec
INSERT INTO booking_events (booking_id, actor_id, from_status, to_status, reason, created_at)
VALUES ($1, $2, sqlc.narg(from_status)::booking_status, $3, sqlc.narg(reason), $4);

-- name: ListBookingEvents :many
SELECT id, booking_id, actor_id, from_status, to_status, reason, created_at
FROM booking_events
WHERE booking_id = $1
-- Events written in one instant would tie on created_at; the status they leave from follows the
-- lifecycle (new -> MENUNGGU -> DISETUJUI -> final), which gives a stable order without a sequence column.
ORDER BY created_at,
         CASE from_status WHEN 'MENUNGGU' THEN 1 WHEN 'DISETUJUI' THEN 2 ELSE 0 END,
         id;

-- Serialises one student's booking creations so the quota check below cannot be raced by two
-- requests for different rooms. An advisory lock leaves the users row itself free for other writers.
-- name: LockRequesterQuota :exec
SELECT pg_advisory_xact_lock(hashtextextended('booking-quota:' || $1::text, 0));

-- Live bookings: still pending or approved and not yet over.
-- name: CountActiveBookingsForRequester :one
SELECT count(*)
FROM bookings
WHERE requester_id = $1 AND status IN ('MENUNGGU', 'DISETUJUI') AND end_at > $2;
