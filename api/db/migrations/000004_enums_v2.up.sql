-- Brief v2: new room types, booking statuses and the enums used by the new columns.
-- ALTER TYPE ... ADD VALUE cannot be used in the transaction that adds it, so nothing here uses the new
-- values; the migrations that follow do.

ALTER TYPE room_type ADD VALUE 'AUDITORIUM';
ALTER TYPE room_type ADD VALUE 'RUANG_UJIAN';
ALTER TYPE room_type ADD VALUE 'RUANG_SIDANG';

ALTER TYPE booking_status ADD VALUE 'BERLANGSUNG';
ALTER TYPE booking_status ADD VALUE 'SELESAI';
ALTER TYPE booking_status ADD VALUE 'TIDAK_HADIR';
ALTER TYPE booking_status ADD VALUE 'TANPA_CHECKOUT';
ALTER TYPE booking_status ADD VALUE 'KEDALUWARSA';

CREATE TYPE checkout_condition AS ENUM ('NORMAL','ADA_MASALAH');
CREATE TYPE violation_kind     AS ENUM ('TIDAK_HADIR','TANPA_CHECKOUT','BATAL_TERLAMBAT');
CREATE TYPE approval_mode      AS ENUM ('AUTO','MANUAL');
CREATE TYPE approval_policy    AS ENUM ('AUTO_IF_ELIGIBLE','ALWAYS_MANUAL');
CREATE TYPE manual_reason      AS ENUM ('ROOM_POLICY','ORGANIZATION','VIOLATION_HISTORY');
CREATE TYPE attachment_kind    AS ENUM ('PHOTO','LETTER');
CREATE TYPE import_kind        AS ENUM ('SCHEDULES','USERS');
