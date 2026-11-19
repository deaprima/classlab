-- Report photos move into the shared attachments table (BR-16): one place for photos and letters, one
-- access rule. Existing photos keep their files; the new rows point at them (storage_path is relative to the
-- upload directory, and old files live under reports/).

ALTER TABLE damage_reports ADD COLUMN attachment_id uuid REFERENCES attachments(id);

-- size_bytes is not known for files written before this migration; 0 marks "not recorded".
WITH moved AS (
  INSERT INTO attachments (uploaded_by, kind, storage_path, content_type, size_bytes, created_at)
  SELECT d.reporter_id, 'PHOTO', 'reports/' || d.photo_path,
         CASE lower(right(d.photo_path, 4)) WHEN '.png' THEN 'image/png' WHEN 'webp' THEN 'image/webp' ELSE 'image/jpeg' END,
         0, d.created_at
  FROM damage_reports d
  WHERE d.photo_path IS NOT NULL
  RETURNING id, storage_path
)
UPDATE damage_reports d SET attachment_id = moved.id
FROM moved
WHERE d.photo_path IS NOT NULL AND moved.storage_path = 'reports/' || d.photo_path;

ALTER TABLE damage_reports DROP COLUMN photo_path;

-- A reference lookup per attachment is made on every download (who may see this file?).
CREATE INDEX damage_reports_attachment_idx ON damage_reports (attachment_id) WHERE attachment_id IS NOT NULL;
CREATE INDEX bookings_letter_idx   ON bookings (letter_attachment_id)   WHERE letter_attachment_id IS NOT NULL;
CREATE INDEX bookings_checkout_idx ON bookings (checkout_attachment_id) WHERE checkout_attachment_id IS NOT NULL;
CREATE INDEX attachments_owner_idx ON attachments (uploaded_by, kind, created_at DESC);
