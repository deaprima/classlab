-- Scheduled work (BR-13, BR-22, BR-28). Every select locks the booking rows with SKIP LOCKED: a row another
-- transaction (a cancellation, a check-in, another instance of the job) is working on is left for the next
-- tick, so two instances never act on the same booking.

-- Approved, and the check-in window closed before the cutoff (now - CHECKIN_LATE).
-- name: LockNoShows :many
SELECT b.*, r.code AS room_code
FROM bookings b JOIN rooms r ON r.id = b.room_id
WHERE b.status = 'DISETUJUI' AND b.start_at < sqlc.arg(cutoff)
ORDER BY b.start_at, b.id
LIMIT sqlc.arg(batch)
FOR UPDATE OF b SKIP LOCKED;

-- In use, and end_at + CHECKOUT_GRACE has passed.
-- name: LockMissedCheckouts :many
SELECT b.*, r.code AS room_code
FROM bookings b JOIN rooms r ON r.id = b.room_id
WHERE b.status = 'BERLANGSUNG' AND b.end_at < sqlc.arg(cutoff)
ORDER BY b.end_at, b.id
LIMIT sqlc.arg(batch)
FOR UPDATE OF b SKIP LOCKED;

-- Waiting past expires_at (BR-22).
-- name: LockExpiredRequests :many
SELECT b.*, r.code AS room_code
FROM bookings b JOIN rooms r ON r.id = b.room_id
WHERE b.status = 'MENUNGGU' AND b.expires_at IS NOT NULL AND b.expires_at < sqlc.arg(now)
ORDER BY b.expires_at, b.group_id NULLS FIRST, b.occurrence_no, b.id
LIMIT sqlc.arg(batch)
FOR UPDATE OF b SKIP LOCKED;

-- A change of status made by the system: no decision fields are touched.
-- name: SystemSetBookingStatus :exec
UPDATE bookings SET status = sqlc.arg(status)::booking_status, updated_at = sqlc.arg(now) WHERE id = sqlc.arg(id);

-- Reminder candidates (BR-28): bookings that have not been sent the reminder of the given type yet. Locked
-- rows cannot be reminded twice by two instances; the unique index notifications_reminder_uniq is the last safeguard.
-- Check-in window open (REMINDER_CHECKIN, REMINDER_CHECKIN_LAST): approved, start_at in [from, to].
-- name: LockCheckinReminderCandidates :many
SELECT b.*, r.code AS room_code
FROM bookings b JOIN rooms r ON r.id = b.room_id
WHERE b.status = 'DISETUJUI' AND b.start_at >= sqlc.arg(start_from) AND b.start_at <= sqlc.arg(start_to)
  AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.user_id = b.requester_id AND n.type = sqlc.arg(reminder_type)::text AND n.ref_id = b.id)
ORDER BY b.start_at, b.id
LIMIT sqlc.arg(batch)
FOR UPDATE OF b SKIP LOCKED;

-- name: LockCheckoutReminderCandidates :many
SELECT b.*, r.code AS room_code
FROM bookings b JOIN rooms r ON r.id = b.room_id
WHERE b.status = 'BERLANGSUNG' AND b.end_at >= sqlc.arg(end_from) AND b.end_at <= sqlc.arg(end_to)
  AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.user_id = b.requester_id AND n.type = sqlc.arg(reminder_type)::text AND n.ref_id = b.id)
ORDER BY b.end_at, b.id
LIMIT sqlc.arg(batch)
FOR UPDATE OF b SKIP LOCKED;
