# ERD ClassLab

Skema PostgreSQL setelah migrasi `000009` (000009 hanya menambah trigger, tanpa tabel atau kolom baru). Sumber kebenarannya tetap berkas di `migrations/`; diagram ini
dibuat dari database yang sudah dimigrasi dan hanya memuat kolom yang menjelaskan hubungan antartabel.
Semua primary key bertipe `uuid`, semua waktu `timestamptz`.

```mermaid
erDiagram
    org_units ||--o{ org_units : "parent_id"
    org_units ||--o{ users : "org_unit_id"
    org_units ||--o{ rooms : "org_unit_id"
    room_type_policies ||--o{ rooms : "type"

    users ||--o{ bookings : "requester_id"
    users ||--o{ bookings : "decided_by"
    rooms ||--o{ bookings : "room_id"
    bookings ||--o{ booking_events : "booking_id"
    bookings ||--o{ violations : "booking_id"
    users ||--o{ violations : "user_id"

    attachments ||--o| bookings : "letter_attachment_id"
    attachments ||--o| bookings : "checkout_attachment_id"
    attachments ||--o| damage_reports : "attachment_id"
    users ||--o{ attachments : "uploaded_by"

    rooms ||--o{ damage_reports : "room_id"
    users ||--o{ damage_reports : "reporter_id"
    damage_reports ||--o{ report_events : "report_id"

    rooms ||--o{ room_schedules : "room_id"
    imports ||--o{ room_schedules : "import_id"
    imports ||--o{ user_courses : "import_id"
    users ||--o{ user_courses : "user_id"
    users ||--o{ imports : "uploaded_by"

    users ||--o{ notifications : "user_id"
    users ||--o{ refresh_tokens : "user_id"
    users ||--o{ audit_logs : "actor_id"

    org_units {
        uuid id PK
        uuid parent_id FK
        org_unit_type type "FAKULTAS PRODI RUMPUN"
        text name
    }
    users {
        uuid id PK
        uuid org_unit_id FK
        text identifier UK
        user_role role
        text password_hash
        bool must_change_password
        timestamptz booking_blocked_until
    }
    rooms {
        uuid id PK
        uuid org_unit_id FK
        text code "unik selama aktif"
        room_type type FK
        text qr_token
        timestamptz deleted_at
    }
    room_type_policies {
        room_type room_type PK
        approval_policy approval_policy
        bool letter_required
        time open_time
        time close_time
        int max_advance_days
    }
    bookings {
        uuid id PK
        uuid room_id FK
        uuid requester_id FK
        timestamptz start_at
        timestamptz end_at
        booking_status status
        approval_mode approval_mode
        uuid group_id "booking berulang"
        uuid letter_attachment_id FK
        uuid checkout_attachment_id FK
        text idempotency_key
    }
    booking_events {
        uuid id PK
        uuid booking_id FK
        uuid actor_id FK
        booking_status from_status
        booking_status to_status
    }
    violations {
        uuid id PK
        uuid user_id FK
        uuid booking_id FK
        violation_kind kind
        int points
        timestamptz annulled_at
    }
    damage_reports {
        uuid id PK
        uuid room_id FK
        uuid reporter_id FK
        report_status status
        uuid attachment_id FK
    }
    report_events {
        uuid id PK
        uuid report_id FK
        report_status to_status
    }
    attachments {
        uuid id PK
        uuid uploaded_by FK
        attachment_kind kind "PHOTO LETTER"
        text storage_path
        timestamptz purged_at
    }
    room_schedules {
        uuid id PK
        uuid room_id FK
        uuid import_id FK
        schedule_source source "AKADEMIK MANUAL"
        timestamptz start_at
        timestamptz end_at
    }
    imports {
        uuid id PK
        import_kind kind "SCHEDULES USERS"
        uuid uploaded_by FK
        text file_sha256
        timestamptz rolled_back_at
    }
    user_courses {
        uuid id PK
        uuid user_id FK
        uuid import_id FK
        text semester
    }
    notifications {
        uuid id PK
        uuid user_id FK
        text type
        timestamptz read_at
    }
    refresh_tokens {
        uuid id PK
        uuid user_id FK
        uuid family_id
        text token_hash
        timestamptz revoked_at
    }
    audit_logs {
        uuid id PK
        uuid actor_id FK
        text action
        text entity_type
        jsonb before
        jsonb after
        inet ip
    }
```

## Catatan

- **Ada tabel lain yang bukan data aplikasi:** `schema_migrations` milik golang-migrate (satu baris, versi
  migrasi saat ini).
- **Relasi ke `users` yang tidak digambar** agar diagram terbaca: `imports.rolled_back_by`,
  `violations.annulled_by`, `booking_events.actor_id`, `report_events.actor_id`, `damage_reports.handled_by`,
  dan `room_schedules.created_by`.
- **Dua lampiran per booking:** surat pengajuan (`letter_attachment_id`) dan foto check-out
  (`checkout_attachment_id`) sama-sama menunjuk `attachments`. Berkas fisiknya ada di `UPLOAD_DIR`; setelah
  masa simpan lewat, berkasnya dihapus dan baris `attachments` diberi `purged_at`.
- **Ruang dan jam tidak bisa terpakai dua kali, dijaga database:** constraint `EXCLUDE` menolak dua
  booking aktif (MENUNGGU, DISETUJUI, BERLANGSUNG) yang tumpang tindih di satu ruang, dan dua jadwal yang
  tumpang tindih. Booking vs jadwal ada di dua tabel, jadi dijaga trigger `bookings_no_schedule_overlap` dan
  `room_schedules_no_booking_overlap` (migrasi 000009) yang mengunci baris ruang lalu menolak dengan kode
  `23P01`. Lapisan pertamanya tetap kunci `rooms ... FOR UPDATE` di service.
- **`audit_logs`:** `actor_id` mengacu ke `users` tetapi boleh kosong (aksi sistem). `entity_id` tidak punya
  foreign key karena `entity_type` yang menentukan tabel tujuannya.
- **Enum:** `user_role` (MAHASISWA, ORGANISASI, TU, KALAB, OB, ADMIN), `booking_status` (MENUNGGU,
  DISETUJUI, DITOLAK, DIBATALKAN, BERLANGSUNG, SELESAI, TIDAK_HADIR, TANPA_CHECKOUT, KEDALUWARSA),
  `room_type` (KELAS, LAB, AUDITORIUM, RUANG_UJIAN, RUANG_SIDANG), `report_status` (BARU, DITERIMA,
  DIPROSES, SELESAI), `violation_kind` (TIDAK_HADIR, TANPA_CHECKOUT, BATAL_TERLAMBAT), `manual_reason`
  (ROOM_POLICY, ORGANIZATION, VIOLATION_HISTORY).

## Membuat ulang diagram

Setelah menambah migrasi, perbarui diagram ini dari database yang sudah dimigrasi:

```sh
# kolom
docker exec classlab-postgres psql -U classlab -d classlab -c "\d+ bookings"
# semua foreign key
docker exec classlab-postgres psql -U classlab -d classlab -At -c \
  "select conrelid::regclass, confrelid::regclass from pg_constraint where contype='f' order by 1"
```
