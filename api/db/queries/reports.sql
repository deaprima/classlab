-- Reads that serve the API join the room and reporter in one query, so list pages never go N+1.
-- name: GetReport :one
SELECT d.id, d.room_id, d.reporter_id, d.description, d.attachment_id, d.status, d.handled_by, d.created_at,
       r.org_unit_id AS room_org_unit_id, r.code AS room_code, r.name AS room_name, r.type AS room_type,
       r.capacity AS room_capacity, r.facilities AS room_facilities, r.is_active AS room_is_active,
       r.open_time AS room_open_time, r.close_time AS room_close_time,
       u.identifier AS reporter_identifier, u.name AS reporter_name
FROM damage_reports d
JOIN rooms r ON r.id = d.room_id
JOIN users u ON u.id = d.reporter_id
WHERE d.id = $1;

-- name: ListReports :many
SELECT d.id, d.room_id, d.reporter_id, d.description, d.attachment_id, d.status, d.handled_by, d.created_at,
       r.org_unit_id AS room_org_unit_id, r.code AS room_code, r.name AS room_name, r.type AS room_type,
       r.capacity AS room_capacity, r.facilities AS room_facilities, r.is_active AS room_is_active,
       r.open_time AS room_open_time, r.close_time AS room_close_time,
       u.identifier AS reporter_identifier, u.name AS reporter_name
FROM damage_reports d
JOIN rooms r ON r.id = d.room_id
JOIN users u ON u.id = d.reporter_id
WHERE (sqlc.narg(reporter_id)::uuid IS NULL OR d.reporter_id = sqlc.narg(reporter_id)::uuid)
  AND (sqlc.narg(status)::report_status IS NULL OR d.status = sqlc.narg(status)::report_status)
  AND (sqlc.narg(room_id)::uuid IS NULL OR d.room_id = sqlc.narg(room_id)::uuid)
  AND (sqlc.narg(room_types)::text[] IS NULL
       OR r.type::text = ANY(sqlc.narg(room_types)::text[])
       OR d.reporter_id = sqlc.narg(viewer_id)::uuid)
ORDER BY d.created_at DESC, d.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountReports :one
SELECT count(*)
FROM damage_reports d
JOIN rooms r ON r.id = d.room_id
WHERE (sqlc.narg(reporter_id)::uuid IS NULL OR d.reporter_id = sqlc.narg(reporter_id)::uuid)
  AND (sqlc.narg(status)::report_status IS NULL OR d.status = sqlc.narg(status)::report_status)
  AND (sqlc.narg(room_id)::uuid IS NULL OR d.room_id = sqlc.narg(room_id)::uuid)
  AND (sqlc.narg(room_types)::text[] IS NULL
       OR r.type::text = ANY(sqlc.narg(room_types)::text[])
       OR d.reporter_id = sqlc.narg(viewer_id)::uuid);

-- Taken before a status change so two concurrent updates cannot both pass the one-step check.
-- name: LockReport :one
SELECT id, room_id, reporter_id, description, attachment_id, status, handled_by, created_at, updated_at
FROM damage_reports
WHERE id = $1
FOR UPDATE;

-- name: CreateReport :one
INSERT INTO damage_reports (room_id, reporter_id, description, attachment_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $5)
RETURNING id, room_id, reporter_id, description, attachment_id, status, handled_by, created_at, updated_at;

-- name: UpdateReportStatus :exec
UPDATE damage_reports SET
  status     = sqlc.arg(status)::report_status,
  handled_by = sqlc.arg(handled_by),
  updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: CreateReportEvent :exec
INSERT INTO report_events (report_id, actor_id, from_status, to_status, note, created_at)
VALUES ($1, $2, sqlc.narg(from_status)::report_status, $3, sqlc.narg(note), $4);

-- name: ListReportEvents :many
SELECT id, report_id, actor_id, from_status, to_status, note, created_at
FROM report_events
WHERE report_id = $1
-- Events written in one instant would tie on created_at; the status they leave from follows the
-- lifecycle (new -> BARU -> DITERIMA -> DIPROSES), which gives a stable order without a sequence column.
ORDER BY created_at,
         CASE from_status WHEN 'BARU' THEN 1 WHEN 'DITERIMA' THEN 2 WHEN 'DIPROSES' THEN 3 ELSE 0 END,
         id;
