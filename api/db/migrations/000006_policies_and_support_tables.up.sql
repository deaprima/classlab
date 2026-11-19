-- Brief v2 tables: room type policies (BR-20), imports (BR-23, BR-24), course context, attachments
-- (BR-16, BR-19), violations (BR-14) and the audit log (BR-25).

CREATE TABLE room_type_policies (
  room_type        room_type PRIMARY KEY,
  label            text NOT NULL,
  approver_roles   user_role[] NOT NULL CHECK (cardinality(approver_roles) > 0 AND approver_roles <@ ARRAY['TU','KALAB','ADMIN']::user_role[]),
  requester_roles  user_role[] NOT NULL,
  approval_policy  approval_policy NOT NULL,
  letter_required  boolean NOT NULL DEFAULT false,
  open_time        time NOT NULL,
  close_time       time NOT NULL,
  min_duration_min int NOT NULL CHECK (min_duration_min > 0),
  max_duration_min int NOT NULL,
  max_advance_days int NOT NULL,
  updated_at       timestamptz NOT NULL DEFAULT now(),
  CHECK (open_time < close_time AND min_duration_min <= max_duration_min)
);

-- Initial values (BR-20). Administrators change them afterwards; code never branches on the type.
INSERT INTO room_type_policies
  (room_type, label, approver_roles, requester_roles, approval_policy, letter_required, open_time, close_time, min_duration_min, max_duration_min, max_advance_days)
VALUES
  ('KELAS',        'Ruang Kelas',  '{TU}',       '{MAHASISWA,ORGANISASI}', 'AUTO_IF_ELIGIBLE', false, '07:00', '21:00',  30,  480, 30),
  ('LAB',          'Laboratorium', '{KALAB}',    '{MAHASISWA,ORGANISASI}', 'ALWAYS_MANUAL',    false, '07:00', '21:00',  30,  480, 30),
  ('AUDITORIUM',   'Auditorium',   '{TU,ADMIN}', '{ORGANISASI}',           'ALWAYS_MANUAL',    true,  '07:00', '22:00',  60,  720, 90),
  ('RUANG_UJIAN',  'Ruang Ujian',  '{TU,ADMIN}', '{MAHASISWA,ORGANISASI}', 'ALWAYS_MANUAL',    false, '07:00', '18:00',  60,  240, 60),
  ('RUANG_SIDANG', 'Ruang Sidang', '{TU,ADMIN}', '{MAHASISWA}',            'ALWAYS_MANUAL',    false, '07:00', '17:00',  60,  180, 60);

-- Only a hash of the file and a summary are kept; the CSV content is never stored.
CREATE TABLE imports (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind           import_kind NOT NULL,
  uploaded_by    uuid NOT NULL REFERENCES users(id),
  file_name      text,
  file_sha256    text NOT NULL,
  semester       text,
  status         text NOT NULL DEFAULT 'COMMITTED' CHECK (status IN ('COMMITTED','ROLLED_BACK')),
  summary        jsonb NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  rolled_back_at timestamptz,
  rolled_back_by uuid REFERENCES users(id)
);
CREATE UNIQUE INDEX imports_kind_hash_uniq ON imports (kind, file_sha256) WHERE status = 'COMMITTED';

CREATE TABLE user_courses (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id),
  import_id   uuid NOT NULL REFERENCES imports(id),
  semester    text NOT NULL,
  course_code text,
  course_name text,
  class_name  text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_courses_user_idx ON user_courses (user_id, semester);

CREATE TABLE attachments (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  uploaded_by   uuid NOT NULL REFERENCES users(id),
  kind          attachment_kind NOT NULL,
  original_name text,
  storage_path  text NOT NULL,
  content_type  text NOT NULL,
  size_bytes    int  NOT NULL,
  purged_at     timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE violations (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id),
  booking_id   uuid NOT NULL REFERENCES bookings(id),
  kind         violation_kind NOT NULL,
  points       int NOT NULL CHECK (points > 0),
  annulled_at  timestamptz,
  annulled_by  uuid REFERENCES users(id),
  annul_reason text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (booking_id, kind)
);
CREATE INDEX violations_user_idx ON violations (user_id, created_at DESC);

CREATE TABLE audit_logs (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id    uuid REFERENCES users(id),
  actor_role  user_role,
  action      text NOT NULL,
  entity_type text NOT NULL,
  entity_id   uuid,
  before      jsonb,
  after       jsonb,
  ip          inet,
  user_agent  text,
  request_id  text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_created_idx ON audit_logs (created_at DESC);
CREATE INDEX audit_logs_entity_idx  ON audit_logs (entity_type, entity_id, created_at DESC);
CREATE TRIGGER audit_logs_immutable
  BEFORE UPDATE OR DELETE ON audit_logs
  FOR EACH ROW EXECUTE FUNCTION forbid_mutation();
