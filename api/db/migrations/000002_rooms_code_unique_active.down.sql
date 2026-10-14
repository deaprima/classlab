-- Fails if a live and a deleted room share a code; resolve those rows before rolling back.
DROP INDEX rooms_org_unit_code_active_key;
ALTER TABLE rooms ADD CONSTRAINT rooms_org_unit_id_code_key UNIQUE (org_unit_id, code);
