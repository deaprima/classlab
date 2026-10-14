-- Indexes for lookups the application already makes. Without them each of these is a scan that grows
-- with the table: event trails are read per booking/report, sessions are revoked per user and per
-- family, and the purge job finds expired tokens by expires_at.
CREATE INDEX booking_events_booking_idx ON booking_events (booking_id, created_at);
CREATE INDEX report_events_report_idx   ON report_events (report_id, created_at);

CREATE INDEX refresh_tokens_user_idx    ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_family_idx  ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_expires_idx ON refresh_tokens (expires_at);

-- Report lists filter by reporter (own reports), status and room, newest first.
CREATE INDEX damage_reports_reporter_idx ON damage_reports (reporter_id, created_at DESC);
CREATE INDEX damage_reports_status_idx   ON damage_reports (status, created_at DESC);
CREATE INDEX damage_reports_room_idx     ON damage_reports (room_id);

-- The per-student booking quota counts a requester's live bookings.
CREATE INDEX bookings_requester_active_idx ON bookings (requester_id, end_at) WHERE status IN ('MENUNGGU', 'DISETUJUI');
