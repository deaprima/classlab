DROP TRIGGER room_schedules_no_booking_overlap ON room_schedules;
DROP TRIGGER bookings_no_schedule_overlap ON bookings;
DROP FUNCTION guard_booking_schedule_overlap();
