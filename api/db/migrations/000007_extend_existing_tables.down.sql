DROP INDEX notifications_reminder_uniq;

-- Fails while a system-made event (actor_id NULL) exists: those rows cannot be given an actor.
ALTER TABLE booking_events
  DROP COLUMN changes,
  DROP COLUMN kind,
  ALTER COLUMN actor_id SET NOT NULL;

DROP INDEX bookings_expiry_idx;
DROP INDEX bookings_group_idx;
DROP INDEX bookings_status_start_idx;
DROP INDEX bookings_requester_active_idx;
CREATE INDEX bookings_requester_active_idx ON bookings (requester_id, end_at) WHERE status IN ('MENUNGGU', 'DISETUJUI');

ALTER TABLE bookings DROP CONSTRAINT bookings_no_overlap;
ALTER TABLE bookings ADD CONSTRAINT bookings_no_overlap
  EXCLUDE USING gist (room_id WITH =, tstzrange(start_at, end_at, '[)') WITH &&)
  WHERE (status IN ('MENUNGGU','DISETUJUI'));

ALTER TABLE bookings
  DROP CONSTRAINT bookings_approval_check,
  DROP CONSTRAINT bookings_group_check,
  DROP COLUMN letter_number,
  DROP COLUMN letter_attachment_id,
  DROP COLUMN manual_reason,
  DROP COLUMN approval_mode,
  DROP COLUMN checkout_attachment_id,
  DROP COLUMN checkout_note,
  DROP COLUMN checkout_condition,
  DROP COLUMN checked_out_at,
  DROP COLUMN checked_in_at,
  DROP COLUMN edited_at,
  DROP COLUMN expires_at,
  DROP COLUMN occurrence_no,
  DROP COLUMN group_id,
  DROP COLUMN details;

DROP INDEX room_schedules_import_idx;
ALTER TABLE room_schedules DROP COLUMN import_id;

ALTER TABLE rooms
  DROP CONSTRAINT rooms_type_fkey,
  DROP CONSTRAINT rooms_hours_check,
  DROP COLUMN qr_rotated_at,
  DROP COLUMN qr_token,
  DROP COLUMN close_time,
  DROP COLUMN open_time;

ALTER TABLE users DROP CONSTRAINT users_org_pic_check;
DROP INDEX users_email_key;
-- The old column was NOT NULL; accounts created without an email get an address that cannot receive mail.
UPDATE users SET email = identifier || '@invalid.local' WHERE email IS NULL;
ALTER TABLE users
  ALTER COLUMN email SET NOT NULL,
  DROP COLUMN must_change_password,
  DROP COLUMN pic_phone,
  DROP COLUMN pic_name,
  DROP COLUMN booking_blocked_until;
CREATE UNIQUE INDEX users_email_key ON users (lower(email));
