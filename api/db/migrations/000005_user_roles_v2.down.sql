-- Lossy by nature: TU and KALAB both become TU_KALAB, ADMIN becomes KAPRODI and ORGANISASI accounts
-- fall back to MAHASISWA, the only requester role the old schema knows.

CREATE TYPE user_role_v1 AS ENUM ('MAHASISWA','TU_KALAB','OB','KAPRODI');

ALTER TABLE users ALTER COLUMN role TYPE user_role_v1 USING (
  CASE role::text
    WHEN 'TU'         THEN 'TU_KALAB'
    WHEN 'KALAB'      THEN 'TU_KALAB'
    WHEN 'ADMIN'      THEN 'KAPRODI'
    WHEN 'ORGANISASI' THEN 'MAHASISWA'
    ELSE role::text
  END
)::user_role_v1;

DROP TYPE user_role;
ALTER TYPE user_role_v1 RENAME TO user_role;
