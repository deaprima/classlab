CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TYPE user_role       AS ENUM ('MAHASISWA','TU_KALAB','OB','KAPRODI');
CREATE TYPE room_type       AS ENUM ('KELAS','LAB');
CREATE TYPE org_unit_type   AS ENUM ('FAKULTAS','PRODI','RUMPUN');
CREATE TYPE schedule_source AS ENUM ('AKADEMIK','MANUAL');
CREATE TYPE booking_status  AS ENUM ('MENUNGGU','DISETUJUI','DITOLAK','DIBATALKAN');
CREATE TYPE report_status   AS ENUM ('BARU','DITERIMA','DIPROSES','SELESAI');

CREATE TABLE org_units (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  parent_id  uuid REFERENCES org_units(id),
  type       org_unit_type NOT NULL,
  name       text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_unit_id   uuid NOT NULL REFERENCES org_units(id),
  identifier    text NOT NULL UNIQUE,            -- NIM / NIP
  name          text NOT NULL,
  email         text NOT NULL,
  password_hash text NOT NULL,
  role          user_role NOT NULL,
  is_active     boolean NOT NULL DEFAULT true,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_key ON users (lower(email));

CREATE TABLE refresh_tokens (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id),
  family_id   uuid NOT NULL,                      -- untuk deteksi reuse
  token_hash  text NOT NULL UNIQUE,               -- hash, bukan token asli
  expires_at  timestamptz NOT NULL,
  revoked_at  timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rooms (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_unit_id uuid NOT NULL REFERENCES org_units(id),
  code        text NOT NULL,
  name        text NOT NULL,
  type        room_type NOT NULL,
  capacity    int NOT NULL CHECK (capacity > 0),
  facilities  text[] NOT NULL DEFAULT '{}',
  is_active   boolean NOT NULL DEFAULT true,
  deleted_at  timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_unit_id, code)
);

CREATE TABLE room_schedules (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id    uuid NOT NULL REFERENCES rooms(id),
  title      text NOT NULL,
  start_at   timestamptz NOT NULL,
  end_at     timestamptz NOT NULL,
  source     schedule_source NOT NULL DEFAULT 'MANUAL',
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (start_at < end_at),
  EXCLUDE USING gist (room_id WITH =, tstzrange(start_at, end_at, '[)') WITH &&)
);
CREATE INDEX room_schedules_room_start_idx ON room_schedules (room_id, start_at);

CREATE TABLE bookings (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id         uuid NOT NULL REFERENCES rooms(id),
  requester_id    uuid NOT NULL REFERENCES users(id),
  start_at        timestamptz NOT NULL,
  end_at          timestamptz NOT NULL,
  purpose         text NOT NULL,
  participants    int CHECK (participants > 0),
  notes           text,
  status          booking_status NOT NULL DEFAULT 'MENUNGGU',
  decided_by      uuid REFERENCES users(id),
  decided_at      timestamptz,
  decision_reason text,
  idempotency_key text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CHECK (start_at < end_at),
  UNIQUE (requester_id, idempotency_key),
  CONSTRAINT bookings_no_overlap
    EXCLUDE USING gist (room_id WITH =, tstzrange(start_at, end_at, '[)') WITH &&)
    WHERE (status IN ('MENUNGGU','DISETUJUI'))
);
CREATE INDEX bookings_room_start_idx ON bookings (room_id, start_at);
CREATE INDEX bookings_requester_idx  ON bookings (requester_id, created_at DESC);
CREATE INDEX bookings_status_idx     ON bookings (status, created_at DESC);

CREATE TABLE booking_events (            -- append-only
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id  uuid NOT NULL REFERENCES bookings(id),
  actor_id    uuid NOT NULL REFERENCES users(id),
  from_status booking_status,
  to_status   booking_status NOT NULL,
  reason      text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION forbid_mutation() RETURNS trigger AS $$
BEGIN RAISE EXCEPTION 'table % is append-only', TG_TABLE_NAME; END $$ LANGUAGE plpgsql;

CREATE TRIGGER booking_events_immutable
  BEFORE UPDATE OR DELETE ON booking_events
  FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TABLE notifications (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users(id),
  type       text NOT NULL,
  title      text NOT NULL,
  body       text NOT NULL,
  ref_type   text,
  ref_id     uuid,
  read_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications (user_id, read_at, created_at DESC);

CREATE TABLE damage_reports (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id     uuid NOT NULL REFERENCES rooms(id),
  reporter_id uuid NOT NULL REFERENCES users(id),
  description text NOT NULL,
  photo_path  text,
  status      report_status NOT NULL DEFAULT 'BARU',
  handled_by  uuid REFERENCES users(id),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE report_events (             -- append-only, trigger sama seperti booking_events
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  report_id   uuid NOT NULL REFERENCES damage_reports(id),
  actor_id    uuid NOT NULL REFERENCES users(id),
  from_status report_status,
  to_status   report_status NOT NULL,
  note        text,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER report_events_immutable
  BEFORE UPDATE OR DELETE ON report_events
  FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
