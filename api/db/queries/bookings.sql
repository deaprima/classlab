-- Reads that serve the API join the room and requester in one query, so list pages never go N+1.
-- occurrence_count is how many rows share the booking's group (0 for a booking that is not in one).
-- name: GetBooking :one
SELECT b.id, b.room_id, b.requester_id, b.start_at, b.end_at, b.purpose, b.participants, b.notes,
       b.status, b.decided_by, b.decided_at, b.decision_reason, b.created_at,
       b.group_id, b.occurrence_no,
       (SELECT count(*) FROM bookings g WHERE g.group_id = b.group_id)::int AS occurrence_count,
       b.expires_at, b.edited_at, b.details, b.approval_mode, b.manual_reason,
       b.letter_attachment_id, b.letter_number,
       b.checked_in_at, b.checked_out_at, b.checkout_condition, b.checkout_note, b.checkout_attachment_id,
       r.org_unit_id AS room_org_unit_id, r.code AS room_code, r.name AS room_name, r.type AS room_type,
       r.capacity AS room_capacity, r.facilities AS room_facilities, r.is_active AS room_is_active,
       r.open_time AS room_open_time, r.close_time AS room_close_time,
       u.identifier AS requester_identifier, u.name AS requester_name
FROM bookings b
JOIN rooms r ON r.id = b.room_id
JOIN users u ON u.id = b.requester_id
WHERE b.id = $1;

-- queue_order sorts the approvers' queue: the request that expires soonest first (BR-22).
-- name: ListBookings :many
SELECT b.id, b.room_id, b.requester_id, b.start_at, b.end_at, b.purpose, b.participants, b.notes,
       b.status, b.decided_by, b.decided_at, b.decision_reason, b.created_at,
       b.group_id, b.occurrence_no,
       (SELECT count(*) FROM bookings g WHERE g.group_id = b.group_id)::int AS occurrence_count,
       b.expires_at, b.edited_at, b.details, b.approval_mode, b.manual_reason,
       b.letter_attachment_id, b.letter_number,
       b.checked_in_at, b.checked_out_at, b.checkout_condition, b.checkout_note, b.checkout_attachment_id,
       r.org_unit_id AS room_org_unit_id, r.code AS room_code, r.name AS room_name, r.type AS room_type,
       r.capacity AS room_capacity, r.facilities AS room_facilities, r.is_active AS room_is_active,
       r.open_time AS room_open_time, r.close_time AS room_close_time,
       u.identifier AS requester_identifier, u.name AS requester_name
FROM bookings b
JOIN rooms r ON r.id = b.room_id
JOIN users u ON u.id = b.requester_id
WHERE (sqlc.narg(requester_id)::uuid IS NULL OR b.requester_id = sqlc.narg(requester_id)::uuid)
  AND (sqlc.narg(status)::booking_status IS NULL OR b.status = sqlc.narg(status)::booking_status)
  AND (sqlc.narg(room_id)::uuid IS NULL OR b.room_id = sqlc.narg(room_id)::uuid)
  AND (sqlc.narg(group_id)::uuid IS NULL OR b.group_id = sqlc.narg(group_id)::uuid)
  AND (sqlc.narg(range_start)::timestamptz IS NULL OR b.end_at > sqlc.narg(range_start)::timestamptz)
  AND (sqlc.narg(range_end)::timestamptz IS NULL OR b.start_at < sqlc.narg(range_end)::timestamptz)
  AND (sqlc.narg(room_types)::text[] IS NULL OR r.type::text = ANY(sqlc.narg(room_types)::text[]))
  AND (sqlc.narg(room_type)::text IS NULL OR r.type::text = sqlc.narg(room_type)::text)
ORDER BY CASE WHEN sqlc.arg(queue_order)::boolean THEN b.expires_at END ASC NULLS LAST,
         b.created_at DESC, b.occurrence_no NULLS FIRST, b.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountBookings :one
SELECT count(*)
FROM bookings b
JOIN rooms r ON r.id = b.room_id
WHERE (sqlc.narg(requester_id)::uuid IS NULL OR b.requester_id = sqlc.narg(requester_id)::uuid)
  AND (sqlc.narg(status)::booking_status IS NULL OR b.status = sqlc.narg(status)::booking_status)
  AND (sqlc.narg(room_id)::uuid IS NULL OR b.room_id = sqlc.narg(room_id)::uuid)
  AND (sqlc.narg(group_id)::uuid IS NULL OR b.group_id = sqlc.narg(group_id)::uuid)
  AND (sqlc.narg(range_start)::timestamptz IS NULL OR b.end_at > sqlc.narg(range_start)::timestamptz)
  AND (sqlc.narg(range_end)::timestamptz IS NULL OR b.start_at < sqlc.narg(range_end)::timestamptz)
  AND (sqlc.narg(room_types)::text[] IS NULL OR r.type::text = ANY(sqlc.narg(room_types)::text[]))
  AND (sqlc.narg(room_type)::text IS NULL OR r.type::text = sqlc.narg(room_type)::text);

-- Plain row, for the transition checks done after the room lock is held.
-- name: LoadBooking :one
SELECT * FROM bookings WHERE id = $1;

-- Every row of a multi-date request, locked, in date order (BR-27). Approving, rejecting and cancelling a
-- group read it first so two such actions cannot interleave.
-- name: LockGroupBookings :many
SELECT * FROM bookings WHERE group_id = $1 ORDER BY occurrence_no FOR UPDATE;

-- name: GetBookingByIdempotencyKey :one
SELECT * FROM bookings WHERE requester_id = $1 AND idempotency_key = $2;

-- name: CreateBooking :one
INSERT INTO bookings (
  room_id, requester_id, start_at, end_at, purpose, participants, notes, details,
  group_id, occurrence_no, expires_at, status, decided_at,
  approval_mode, manual_reason, letter_attachment_id, letter_number, idempotency_key, created_at, updated_at)
VALUES (
  sqlc.arg(room_id), sqlc.arg(requester_id), sqlc.arg(start_at), sqlc.arg(end_at), sqlc.arg(purpose),
  sqlc.narg(participants), sqlc.narg(notes), sqlc.arg(details)::jsonb,
  sqlc.narg(group_id), sqlc.narg(occurrence_no), sqlc.narg(expires_at), sqlc.arg(status)::booking_status, sqlc.narg(decided_at),
  sqlc.arg(approval_mode)::approval_mode, sqlc.narg(manual_reason)::manual_reason,
  sqlc.narg(letter_attachment_id), sqlc.narg(letter_number), sqlc.narg(idempotency_key), sqlc.arg(created_at), sqlc.arg(created_at))
RETURNING *;

-- name: DecideBooking :one
UPDATE bookings SET
  status          = sqlc.arg(status)::booking_status,
  decided_by      = sqlc.arg(decided_by),
  decided_at      = sqlc.arg(now),
  decision_reason = sqlc.narg(decision_reason),
  updated_at      = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- An edit rewrites everything an owner may change together with the state it leads to (BR-26), so the
-- row is never half-edited. A booking sent back to MENUNGGU loses its earlier decision.
-- name: UpdateBookingEdit :one
UPDATE bookings SET
  start_at             = sqlc.arg(start_at),
  end_at               = sqlc.arg(end_at),
  purpose              = sqlc.arg(purpose),
  participants         = sqlc.narg(participants),
  notes                = sqlc.narg(notes),
  details              = sqlc.arg(details)::jsonb,
  letter_attachment_id = sqlc.narg(letter_attachment_id),
  letter_number        = sqlc.narg(letter_number),
  status               = sqlc.arg(status)::booking_status,
  approval_mode        = sqlc.arg(approval_mode)::approval_mode,
  manual_reason        = sqlc.narg(manual_reason)::manual_reason,
  expires_at           = sqlc.narg(expires_at),
  decided_by           = sqlc.narg(decided_by),
  decided_at           = sqlc.narg(decided_at),
  decision_reason      = sqlc.narg(decision_reason),
  edited_at            = sqlc.arg(now),
  updated_at           = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- actor_id is NULL when the system acts (auto-approval, jobs); kind is STATUS or EDITED, and changes
-- holds { field: { from, to } } for an edit (BR-26).
-- name: CreateBookingEvent :exec
INSERT INTO booking_events (booking_id, actor_id, from_status, to_status, reason, kind, changes, created_at)
VALUES ($1, sqlc.narg(actor_id)::uuid, sqlc.narg(from_status)::booking_status, $2, sqlc.narg(reason), sqlc.arg(kind)::text, sqlc.narg(changes)::jsonb, $3);

-- name: ListBookingEvents :many
SELECT * FROM booking_events
WHERE booking_id = $1
-- Events written in one instant would tie on created_at; the status they leave from follows the
-- lifecycle (new -> MENUNGGU -> DISETUJUI -> final), which gives a stable order without a sequence column.
ORDER BY created_at,
         CASE from_status WHEN 'MENUNGGU' THEN 1 WHEN 'DISETUJUI' THEN 2 ELSE 0 END,
         id;

-- Serialises one requester's booking creations so the quota check below cannot be raced by two
-- requests for different rooms. An advisory lock leaves the users row itself free for other writers.
-- name: LockRequesterQuota :exec
SELECT pg_advisory_xact_lock(hashtextextended('booking-quota:' || $1::text, 0));

-- What counts against the quota: live requests that are not over yet. A multi-date group counts once
-- (BR-15, BR-27).
-- name: CountActiveRequests :one
SELECT count(DISTINCT COALESCE(group_id, id))
FROM bookings
WHERE requester_id = $1 AND status IN ('MENUNGGU', 'DISETUJUI', 'BERLANGSUNG') AND end_at > $2;

-- The requester's course context for an approver looking at a booking (BR-24); latest semester first.
-- name: ListUserCourses :many
SELECT semester, course_code, course_name, class_name
FROM user_courses
WHERE user_id = $1
ORDER BY semester DESC, course_code NULLS LAST, id
LIMIT 20;

-- Plain rows of one group, for comparing a replayed request with what was stored.
-- name: ListGroupBookings :many
SELECT * FROM bookings WHERE group_id = $1 ORDER BY occurrence_no;
