-- Brief v2 columns on the tables that already existed. damage_reports is not touched here: its move
-- from photo_path to attachments belongs with the attachments module (Phase 3).

-- users: optional email, organisation contact, block date, forced password change (BR-14, BR-18, BR-24)
ALTER TABLE users
  ALTER COLUMN email DROP NOT NULL,
  ADD COLUMN booking_blocked_until timestamptz,
  ADD COLUMN pic_name text,
  ADD COLUMN pic_phone text,
  ADD COLUMN must_change_password boolean NOT NULL DEFAULT false;
DROP INDEX users_email_key;
CREATE UNIQUE INDEX users_email_key ON users (lower(email)) WHERE email IS NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_org_pic_check
  CHECK (role <> 'ORGANISASI' OR (pic_name IS NOT NULL AND pic_phone IS NOT NULL));

-- rooms: per-room hours (BR-29), QR token (BR-11), and a foreign key to the policy of the type (BR-20)
ALTER TABLE rooms
  ADD COLUMN open_time time,
  ADD COLUMN close_time time,
  ADD COLUMN qr_token text,
  ADD COLUMN qr_rotated_at timestamptz NOT NULL DEFAULT now();
-- Two random UUIDs give 244 random bits, enough for a token that only has to be unguessable.
UPDATE rooms SET qr_token = replace(gen_random_uuid()::text || gen_random_uuid()::text, '-', '');
ALTER TABLE rooms ALTER COLUMN qr_token SET NOT NULL;
ALTER TABLE rooms
  ADD CONSTRAINT rooms_hours_check CHECK ((open_time IS NULL) = (close_time IS NULL) AND (open_time IS NULL OR open_time < close_time)),
  ADD CONSTRAINT rooms_type_fkey FOREIGN KEY (type) REFERENCES room_type_policies(room_type);

-- room_schedules: rows created by a CSV import point back to it (BR-23)
ALTER TABLE room_schedules ADD COLUMN import_id uuid REFERENCES imports(id);
CREATE INDEX room_schedules_import_idx ON room_schedules (import_id) WHERE import_id IS NOT NULL;

-- bookings: approval (BR-17), expiry (BR-22), edits (BR-26), groups (BR-27), check-in/out (BR-11, BR-12), letters (BR-18)
ALTER TABLE bookings
  ADD COLUMN details jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN group_id uuid,
  ADD COLUMN occurrence_no int,
  ADD COLUMN expires_at timestamptz,
  ADD COLUMN edited_at timestamptz,
  ADD COLUMN checked_in_at timestamptz,
  ADD COLUMN checked_out_at timestamptz,
  ADD COLUMN checkout_condition checkout_condition,
  ADD COLUMN checkout_note text,
  ADD COLUMN checkout_attachment_id uuid REFERENCES attachments(id),
  ADD COLUMN approval_mode approval_mode NOT NULL DEFAULT 'MANUAL',
  ADD COLUMN manual_reason manual_reason,
  ADD COLUMN letter_attachment_id uuid REFERENCES attachments(id),
  ADD COLUMN letter_number text;

-- Every booking made before this migration was decided by a person.
UPDATE bookings SET manual_reason = 'ROOM_POLICY';
UPDATE bookings SET expires_at = LEAST(created_at + interval '24 hours', start_at) WHERE status = 'MENUNGGU';
-- The default only filled existing rows; new rows must say how they were decided.
ALTER TABLE bookings ALTER COLUMN approval_mode DROP DEFAULT;

ALTER TABLE bookings
  ADD CONSTRAINT bookings_group_check CHECK ((group_id IS NULL) = (occurrence_no IS NULL)),
  ADD CONSTRAINT bookings_approval_check CHECK (
    (approval_mode = 'AUTO' AND manual_reason IS NULL) OR (approval_mode = 'MANUAL' AND manual_reason IS NOT NULL));

-- BERLANGSUNG holds the slot too (section 4.1).
ALTER TABLE bookings DROP CONSTRAINT bookings_no_overlap;
ALTER TABLE bookings ADD CONSTRAINT bookings_no_overlap
  EXCLUDE USING gist (room_id WITH =, tstzrange(start_at, end_at, '[)') WITH &&)
  WHERE (status IN ('MENUNGGU','DISETUJUI','BERLANGSUNG'));

DROP INDEX bookings_requester_active_idx;
CREATE INDEX bookings_requester_active_idx ON bookings (requester_id, end_at) WHERE status IN ('MENUNGGU', 'DISETUJUI', 'BERLANGSUNG');
CREATE INDEX bookings_status_start_idx ON bookings (status, start_at);
CREATE INDEX bookings_group_idx        ON bookings (group_id) WHERE group_id IS NOT NULL;
CREATE INDEX bookings_expiry_idx       ON bookings (expires_at) WHERE status = 'MENUNGGU';

-- booking_events: system actions have no actor; edits record what changed (BR-26)
ALTER TABLE booking_events
  ALTER COLUMN actor_id DROP NOT NULL,
  ADD COLUMN kind text NOT NULL DEFAULT 'STATUS' CHECK (kind IN ('STATUS','EDITED')),
  ADD COLUMN changes jsonb;

-- notifications: one reminder per user, type and booking (BR-28)
CREATE UNIQUE INDEX notifications_reminder_uniq ON notifications (user_id, type, ref_id) WHERE type LIKE 'REMINDER_%';
