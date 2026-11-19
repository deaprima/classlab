-- A booking that holds its slot never shares a room and time with an academic schedule (brief 4.4). The two
-- live in different tables, so no exclusion constraint can see both; these triggers are the database's own
-- guard behind the room lock every service takes. Each trigger takes that same lock before it looks, so two
-- transactions that each write one side cannot both pass the check. The error is an exclusion violation
-- (23P01), which the services already turn into 409 BOOKING_CONFLICT.

CREATE FUNCTION guard_booking_schedule_overlap() RETURNS trigger AS $$
BEGIN
  IF TG_TABLE_NAME = 'bookings' THEN
    -- Only a write that can start a new overlap needs the check: a booking that becomes live, or a live one
    -- that moves. Approval and check-in keep the same window and skip the lock entirely.
    IF NEW.status NOT IN ('MENUNGGU', 'DISETUJUI', 'BERLANGSUNG') THEN
      RETURN NEW;
    END IF;
    IF TG_OP = 'UPDATE' AND OLD.status IN ('MENUNGGU', 'DISETUJUI', 'BERLANGSUNG')
       AND OLD.room_id = NEW.room_id AND OLD.start_at = NEW.start_at AND OLD.end_at = NEW.end_at THEN
      RETURN NEW;
    END IF;
  ELSIF TG_OP = 'UPDATE'
     AND OLD.room_id = NEW.room_id AND OLD.start_at = NEW.start_at AND OLD.end_at = NEW.end_at THEN
    RETURN NEW;
  END IF;

  PERFORM 1 FROM rooms WHERE id = NEW.room_id FOR UPDATE;

  IF TG_TABLE_NAME = 'bookings' THEN
    IF EXISTS (
      SELECT 1 FROM room_schedules s
      WHERE s.room_id = NEW.room_id
        AND tstzrange(s.start_at, s.end_at, '[)') && tstzrange(NEW.start_at, NEW.end_at, '[)')
    ) THEN
      RAISE EXCEPTION 'booking overlaps an academic schedule in room %', NEW.room_id
        USING ERRCODE = 'exclusion_violation', CONSTRAINT = 'bookings_no_schedule_overlap';
    END IF;
  ELSIF EXISTS (
    SELECT 1 FROM bookings b
    WHERE b.room_id = NEW.room_id
      AND b.status IN ('MENUNGGU', 'DISETUJUI', 'BERLANGSUNG')
      AND tstzrange(b.start_at, b.end_at, '[)') && tstzrange(NEW.start_at, NEW.end_at, '[)')
  ) THEN
    RAISE EXCEPTION 'schedule overlaps a live booking in room %', NEW.room_id
      USING ERRCODE = 'exclusion_violation', CONSTRAINT = 'room_schedules_no_booking_overlap';
  END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER bookings_no_schedule_overlap
  BEFORE INSERT OR UPDATE OF room_id, start_at, end_at, status ON bookings
  FOR EACH ROW EXECUTE FUNCTION guard_booking_schedule_overlap();

CREATE TRIGGER room_schedules_no_booking_overlap
  BEFORE INSERT OR UPDATE OF room_id, start_at, end_at ON room_schedules
  FOR EACH ROW EXECUTE FUNCTION guard_booking_schedule_overlap();
