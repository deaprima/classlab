-- Brief v2 has six roles. TU_KALAB is split into TU and KALAB and KAPRODI is replaced by ADMIN.
-- PostgreSQL cannot drop enum values, so a new type replaces the old one.
-- Existing accounts: TU_KALAB becomes TU and KAPRODI becomes ADMIN (decision G1, 2026-10-20);
-- KALAB accounts are created afterwards by an administrator or the dev seed.

CREATE TYPE user_role_v2 AS ENUM ('MAHASISWA','ORGANISASI','TU','KALAB','OB','ADMIN');

ALTER TABLE users ALTER COLUMN role TYPE user_role_v2 USING (
  CASE role::text
    WHEN 'TU_KALAB' THEN 'TU'
    WHEN 'KAPRODI'  THEN 'ADMIN'
    ELSE role::text
  END
)::user_role_v2;

DROP TYPE user_role;
ALTER TYPE user_role_v2 RENAME TO user_role;
