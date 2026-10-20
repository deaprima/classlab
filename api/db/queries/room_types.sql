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
