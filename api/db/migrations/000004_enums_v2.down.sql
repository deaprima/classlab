-- PostgreSQL cannot remove enum values, so room_type and booking_status are rebuilt without them.
-- This fails while any row still uses a removed value; that is deliberate, the data would be lost.

DROP TYPE import_kind;
DROP TYPE attachment_kind;
DROP TYPE manual_reason;
DROP TYPE approval_policy;
DROP TYPE approval_mode;
DROP TYPE violation_kind;
DROP TYPE checkout_condition;

ALTER TYPE room_type RENAME TO room_type_old;
CREATE TYPE room_type AS ENUM ('KELAS','LAB');
ALTER TABLE rooms ALTER COLUMN type TYPE room_type USING type::text::room_type;
DROP TYPE room_type_old;

-- Objects whose definition names booking_status values are dropped and recreated around the swap.
ALTER TABLE bookings DROP CONSTRAINT bookings_no_overlap;
DROP INDEX bookings_requester_active_idx;
ALTER TABLE bookings ALTER COLUMN status DROP DEFAULT;

ALTER TYPE booking_status RENAME TO booking_status_old;
CREATE TYPE booking_status AS ENUM ('MENUNGGU','DISETUJUI','DITOLAK','DIBATALKAN');
ALTER TABLE bookings ALTER COLUMN status TYPE booking_status USING status::text::booking_status;
ALTER TABLE booking_events
  ALTER COLUMN from_status TYPE booking_status USING from_status::text::booking_status,
  ALTER COLUMN to_status   TYPE booking_status USING to_status::text::booking_status;
DROP TYPE booking_status_old;

ALTER TABLE bookings ALTER COLUMN status SET DEFAULT 'MENUNGGU';
ALTER TABLE bookings ADD CONSTRAINT bookings_no_overlap
  EXCLUDE USING gist (room_id WITH =, tstzrange(start_at, end_at, '[)') WITH &&)
  WHERE (status IN ('MENUNGGU','DISETUJUI'));
CREATE INDEX bookings_requester_active_idx ON bookings (requester_id, end_at) WHERE status IN ('MENUNGGU', 'DISETUJUI');
