-- A soft-deleted room must not keep its code reserved, so uniqueness only applies to live rooms.
ALTER TABLE rooms DROP CONSTRAINT rooms_org_unit_id_code_key;
CREATE UNIQUE INDEX rooms_org_unit_code_active_key ON rooms (org_unit_id, code) WHERE deleted_at IS NULL;
