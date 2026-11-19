-- The role arrays are cast to text[]: pgx cannot decode an array of a user-defined enum without the
-- type being registered on every connection, and a plain text[] needs no such setup.
-- name: GetRoomTypePolicy :one
SELECT room_type, label, approver_roles::text[] AS approver_roles, requester_roles::text[] AS requester_roles,
       approval_policy, letter_required, open_time, close_time, min_duration_min, max_duration_min,
       max_advance_days, updated_at
FROM room_type_policies
WHERE room_type = $1;

-- name: ListRoomTypePolicies :many
SELECT room_type, label, approver_roles::text[] AS approver_roles, requester_roles::text[] AS requester_roles,
       approval_policy, letter_required, open_time, close_time, min_duration_min, max_duration_min,
       max_advance_days, updated_at
FROM room_type_policies
ORDER BY room_type;

-- Taken before an administrator's change so the before-state in the audit log and the cross-field
-- checks (open < close, min <= max) see the row as it is, not as another change leaves it.
-- name: LockRoomTypePolicy :one
SELECT room_type, label, approver_roles::text[] AS approver_roles, requester_roles::text[] AS requester_roles,
       approval_policy, letter_required, open_time, close_time, min_duration_min, max_duration_min,
       max_advance_days, updated_at
FROM room_type_policies
WHERE room_type = $1
FOR UPDATE;

-- The role arrays travel as text[] and are cast in SQL (see the note on GetRoomTypePolicy).
-- name: UpdateRoomTypePolicy :one
UPDATE room_type_policies SET
  approver_roles   = COALESCE(sqlc.narg(approver_roles)::text[]::user_role[], approver_roles),
  requester_roles  = COALESCE(sqlc.narg(requester_roles)::text[]::user_role[], requester_roles),
  approval_policy  = COALESCE(sqlc.narg(approval_policy)::approval_policy, approval_policy),
  letter_required  = COALESCE(sqlc.narg(letter_required)::boolean, letter_required),
  open_time        = COALESCE(sqlc.narg(open_time)::time, open_time),
  close_time       = COALESCE(sqlc.narg(close_time)::time, close_time),
  min_duration_min = COALESCE(sqlc.narg(min_duration_min)::int, min_duration_min),
  max_duration_min = COALESCE(sqlc.narg(max_duration_min)::int, max_duration_min),
  max_advance_days = COALESCE(sqlc.narg(max_advance_days)::int, max_advance_days),
  updated_at       = sqlc.arg(updated_at)
WHERE room_type = sqlc.arg(room_type)
RETURNING room_type, label, approver_roles::text[] AS approver_roles, requester_roles::text[] AS requester_roles,
          approval_policy, letter_required, open_time, close_time, min_duration_min, max_duration_min,
          max_advance_days, updated_at;
