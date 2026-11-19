DROP INDEX attachments_owner_idx;
DROP INDEX bookings_checkout_idx;
DROP INDEX bookings_letter_idx;
DROP INDEX damage_reports_attachment_idx;

ALTER TABLE damage_reports ADD COLUMN photo_path text;
-- Photos stored under reports/ go back to their old column; anything newer lives under attachments/ and
-- has no home in the old schema, so those reports lose the reference (the file stays on disk).
UPDATE damage_reports d SET photo_path = substr(a.storage_path, length('reports/') + 1)
FROM attachments a
WHERE d.attachment_id = a.id AND a.storage_path LIKE 'reports/%';
ALTER TABLE damage_reports DROP COLUMN attachment_id;
DELETE FROM attachments a WHERE a.storage_path LIKE 'reports/%' AND NOT EXISTS (
  SELECT 1 FROM bookings b WHERE b.letter_attachment_id = a.id OR b.checkout_attachment_id = a.id);
