# ClassLab.io: Agent Brief

Dokumen ini adalah **sumber kebenaran tentang APA yang dibangun**. Aturan **BAGAIMANA** bekerja ada di `agent.md`. Baca keduanya sebelum menulis kode.

- Versi: 0.1 (12 Oktober 2026)
- Asal: Project Charter & WBS ManPro Kelompok 2, dipertajam menjadi spesifikasi teknis.
- Hal bertanda **[default]** adalah keputusan yang diambil agar tidak ada yang terblokir. Pakai nilainya, jangan berhenti bertanya, dan catat di `docs/decisions.md` bila diubah.

---

## 1. Produk

**ClassLab.io** adalah sistem web peminjaman ruang kelas dan laboratorium kampus.

**Masalah:** peminjaman masih manual, informasi ketersediaan tidak terpusat, jadwal tidak selalu cocok dengan kondisi aktual, dan tidak ada proses booking + approval yang terdokumentasi.

**Solusi:** pengguna melihat jadwal dan ketersediaan ruang, mengajukan peminjaman, TU/Kepala Lab menyetujui atau menolak, semua pihak mendapat notifikasi, riwayat tersimpan, dan kerusakan ruang bisa dilaporkan.

**Cakupan organisasi:** MVP untuk satu unit (rumpun TI). Model data sudah memuat `org_unit_id` agar bisa diperluas ke fakultas, tetapi fitur lintas unit **tidak** dibangun sekarang.

### In scope (MVP)

1. Login/logout dan hak akses per peran
2. Pengelolaan data ruang dan jadwal
3. Status ketersediaan ruang: `TERSEDIA`, `DIGUNAKAN`, `DIAJUKAN`
4. Pengajuan booking dengan validasi bentrok
5. Approval/penolakan oleh TU/Kalab (penolakan wajib alasan)
6. Notifikasi in-app
7. Riwayat pengajuan dan penggunaan
8. Pelaporan kondisi/kerusakan ruang
9. Kelola pengguna oleh Kaprodi

### Out of scope (JANGAN dibangun)

Aplikasi mobile · pengubahan otomatis jadwal akademik · integrasi CCTV · deteksi ruang kosong otomatis · pembayaran · inventarisasi alat lab · integrasi SIAKAD/SSO · email/WhatsApp · registrasi mandiri · lupa password via email.

Jika ada permintaan yang menyentuh daftar ini, hentikan dan tanyakan.

---

## 2. Stack (final)

| Lapisan | Pilihan |
|---|---|
| Backend | Go (versi stabil terbaru), `net/http` + `chi`, `pgx/v5`, `sqlc`, `golang-migrate`, `log/slog`, `golang-jwt/jwt/v5`, `golang.org/x/crypto` (argon2id) |
| Database | PostgreSQL 15+ dengan ekstensi `btree_gist` |
| Frontend | Template **satnaing/shadcn-admin** (Vite, React 19, TypeScript strict, TanStack Router/Query/Table, shadcn/ui, Tailwind v4, Zustand, react-hook-form + Zod, axios, date-fns), paket manager `pnpm` |
| Kontrak | OpenAPI 3.1 di `api/openapi.yaml`; tipe FE di-generate dengan `openapi-typescript` |
| Test | `testcontainers-go` (Postgres asli), `kin-openapi` untuk contract test, Vitest di FE |
| Infra | Docker Compose (dev dan prod), Caddy (HTTPS), file upload di volume lokal |

**[default]** Backend memakai Go. Jika tim memutuskan lain, ubah bagian ini dan `agent.md` bagian 3 sebelum Phase 1.

Tambah dependency di luar tabel ini hanya setelah bertanya (lihat `agent.md`).

---

## 3. Peran dan hak akses

| Kode | Nama | Ringkas |
|---|---|---|
| `MAHASISWA` | Mahasiswa/PJ | Peminjam |
| `TU_KALAB` | TU / Kepala Lab | Approver, pengelola ruang, jadwal, laporan |
| `OB` | OB | Verifikator kondisi ruangan; boleh membuat laporan |
| `KAPRODI` | Kaprodi | System admin; kelola pengguna; akses baca luas |

| Aksi | MAHASISWA | TU_KALAB | OB | KAPRODI |
|---|---|---|---|---|
| Lihat jadwal dan ketersediaan | ✓ | ✓ | ✓ | ✓ |
| Buat pengajuan booking | ✓ | | | |
| Lihat / batalkan pengajuan sendiri (saat `MENUNGGU`) | ✓ | | | |
| Lihat semua pengajuan dan riwayat | | ✓ | | ✓ (baca) |
| Approve / reject | | ✓ | | |
| Batalkan booking `DISETUJUI` (alasan wajib) | | ✓ | | |
| CRUD ruang dan jadwal | | ✓ | | ✓ |
| Buat laporan kerusakan | ✓ | ✓ | ✓ | ✓ |
| Ubah status laporan | | ✓ | | |
| Kelola pengguna | | | | ✓ |

RBAC **wajib ditegakkan di server** (middleware + cek di service). Menyembunyikan menu di FE hanya UX.

---

## 4. Aturan bisnis

### 4.1 Status booking

| Dari | Ke | Oleh | Syarat |
|---|---|---|---|
| (baru) | `MENUNGGU` | MAHASISWA | Lolos validasi + cek bentrok |
| `MENUNGGU` | `DISETUJUI` | TU_KALAB | Cek bentrok ulang di transaksi |
| `MENUNGGU` | `DITOLAK` | TU_KALAB | `reason` wajib |
| `MENUNGGU` | `DIBATALKAN` | Pemilik | - |
| `DISETUJUI` | `DIBATALKAN` | TU_KALAB | `reason` wajib |

`DITOLAK` dan `DIBATALKAN` final. Transisi lain → `409 INVALID_STATE`.

### 4.2 Status ketersediaan (dihitung, tidak disimpan)

Untuk rentang waktu yang ditanyakan, prioritas `DIGUNAKAN` > `DIAJUKAN` > `TERSEDIA`.

- `DIGUNAKAN`: ada `room_schedules` atau booking `DISETUJUI` yang overlap
- `DIAJUKAN`: ada booking `MENUNGGU` yang overlap
- `TERSEDIA`: tidak ada keduanya

### 4.3 Aturan

- **BR-01 Validasi waktu.** `start_at < end_at`; satu hari yang sama; kelipatan 30 menit; tidak di masa lalu. **[default]** durasi 30 menit sampai 8 jam, jam operasional 07:00–21:00 WIB, maksimal H+30. Semua nilai dari konfigurasi (env), bukan hardcode.
- **BR-02 Bentrok.** Interval setengah-terbuka `[start, end)`. 08:00–10:00 dan 10:00–12:00 tidak bentrok.
- **BR-03 `MENUNGGU` menahan slot** **[default]**. Slot terbuka kembali bila ditolak/dibatalkan.
- **BR-04 Alasan.** Penolakan dan pembatalan booking disetujui wajib `reason` minimal 5 karakter.
- **BR-05 Jadwal akademik tidak diubah.** Booking disetujui tampil sebagai lapisan (overlay) di timeline ruang, bukan mengubah `room_schedules`.
- **BR-06 Riwayat append-only.** Setiap perubahan status menambah baris `booking_events`. Tidak ada endpoint ubah/hapus riwayat; trigger DB juga melarang.
- **BR-07 Ruang.** Ruang `is_active = false` tidak bisa dibooking. Hapus ruang = soft delete, ditolak (`ROOM_HAS_ACTIVE_BOOKINGS`) bila ada booking aktif masa depan.
- **BR-08 Laporan.** Status `BARU → DITERIMA → DIPROSES → SELESAI` (hanya maju satu langkah, tidak mundur). Tiap perubahan menotifikasi pelapor.
- **BR-09 Jadwal berulang.** `POST /schedules` dengan `repeat` men-generate kejadian mingguan sampai tanggal `until` (maks 26 minggu). Semua atau tidak sama sekali (satu transaksi); bila ada satu bentrok, seluruh request gagal dengan daftar bentrok.

### 4.4 Konkurensi dan integritas (PENTING)

Cek di aplikasi saja **tidak cukup**. Terapkan berlapis:

1. Operasi create booking, approve, dan create/update schedule berjalan dalam **satu transaksi** dengan `SELECT ... FROM rooms WHERE id = $1 FOR UPDATE`.
2. Di dalam transaksi, cek overlap ke `room_schedules` dan `bookings` aktif (`MENUNGGU`, `DISETUJUI`).
3. Exclusion constraint database sebagai pengaman terakhir (lihat skema). Constraint tidak bisa menjangkau lintas tabel, maka langkah 1–2 tetap wajib.
4. Pelanggaran constraint (`23P01`) dipetakan ke `409 BOOKING_CONFLICT`.

**Wajib ada tes:** 20 request paralel untuk slot yang sama menghasilkan tepat 1 sukses.

---

## 5. Skema database

Migrasi lewat `golang-migrate` (`db/migrations`). Semua waktu `timestamptz` (UTC).

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TYPE user_role       AS ENUM ('MAHASISWA','TU_KALAB','OB','KAPRODI');
CREATE TYPE room_type       AS ENUM ('KELAS','LAB');
CREATE TYPE org_unit_type   AS ENUM ('FAKULTAS','PRODI','RUMPUN');
CREATE TYPE schedule_source AS ENUM ('AKADEMIK','MANUAL');
CREATE TYPE booking_status  AS ENUM ('MENUNGGU','DISETUJUI','DITOLAK','DIBATALKAN');
CREATE TYPE report_status   AS ENUM ('BARU','DITERIMA','DIPROSES','SELESAI');

CREATE TABLE org_units (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  parent_id  uuid REFERENCES org_units(id),
  type       org_unit_type NOT NULL,
  name       text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_unit_id   uuid NOT NULL REFERENCES org_units(id),
  identifier    text NOT NULL UNIQUE,            -- NIM / NIP
  name          text NOT NULL,
  email         text NOT NULL,
  password_hash text NOT NULL,
  role          user_role NOT NULL,
  is_active     boolean NOT NULL DEFAULT true,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_key ON users (lower(email));

CREATE TABLE refresh_tokens (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id),
  family_id   uuid NOT NULL,                      -- untuk deteksi reuse
  token_hash  text NOT NULL UNIQUE,               -- hash, bukan token asli
  expires_at  timestamptz NOT NULL,
  revoked_at  timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rooms (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  org_unit_id uuid NOT NULL REFERENCES org_units(id),
  code        text NOT NULL,
  name        text NOT NULL,
  type        room_type NOT NULL,
  capacity    int NOT NULL CHECK (capacity > 0),
  facilities  text[] NOT NULL DEFAULT '{}',
  is_active   boolean NOT NULL DEFAULT true,
  deleted_at  timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_unit_id, code)
);

CREATE TABLE room_schedules (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id    uuid NOT NULL REFERENCES rooms(id),
  title      text NOT NULL,
  start_at   timestamptz NOT NULL,
  end_at     timestamptz NOT NULL,
  source     schedule_source NOT NULL DEFAULT 'MANUAL',
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (start_at < end_at),
  EXCLUDE USING gist (room_id WITH =, tstzrange(start_at, end_at, '[)') WITH &&)
);
CREATE INDEX room_schedules_room_start_idx ON room_schedules (room_id, start_at);

CREATE TABLE bookings (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id         uuid NOT NULL REFERENCES rooms(id),
  requester_id    uuid NOT NULL REFERENCES users(id),
  start_at        timestamptz NOT NULL,
  end_at          timestamptz NOT NULL,
  purpose         text NOT NULL,
  participants    int CHECK (participants > 0),
  notes           text,
  status          booking_status NOT NULL DEFAULT 'MENUNGGU',
  decided_by      uuid REFERENCES users(id),
  decided_at      timestamptz,
  decision_reason text,
  idempotency_key text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CHECK (start_at < end_at),
  UNIQUE (requester_id, idempotency_key),
  CONSTRAINT bookings_no_overlap
    EXCLUDE USING gist (room_id WITH =, tstzrange(start_at, end_at, '[)') WITH &&)
    WHERE (status IN ('MENUNGGU','DISETUJUI'))
);
CREATE INDEX bookings_room_start_idx ON bookings (room_id, start_at);
CREATE INDEX bookings_requester_idx  ON bookings (requester_id, created_at DESC);
CREATE INDEX bookings_status_idx     ON bookings (status, created_at DESC);

CREATE TABLE booking_events (            -- append-only
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id  uuid NOT NULL REFERENCES bookings(id),
  actor_id    uuid NOT NULL REFERENCES users(id),
  from_status booking_status,
  to_status   booking_status NOT NULL,
  reason      text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION forbid_mutation() RETURNS trigger AS $$
BEGIN RAISE EXCEPTION 'table % is append-only', TG_TABLE_NAME; END $$ LANGUAGE plpgsql;
CREATE TRIGGER booking_events_immutable
  BEFORE UPDATE OR DELETE ON booking_events
  FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

CREATE TABLE notifications (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users(id),
  type       text NOT NULL,
  title      text NOT NULL,
  body       text NOT NULL,
  ref_type   text,
  ref_id     uuid,
  read_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications (user_id, read_at, created_at DESC);

CREATE TABLE damage_reports (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id     uuid NOT NULL REFERENCES rooms(id),
  reporter_id uuid NOT NULL REFERENCES users(id),
  description text NOT NULL,
  photo_path  text,
  status      report_status NOT NULL DEFAULT 'BARU',
  handled_by  uuid REFERENCES users(id),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE report_events (             -- append-only, trigger sama seperti booking_events
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  report_id   uuid NOT NULL REFERENCES damage_reports(id),
  actor_id    uuid NOT NULL REFERENCES users(id),
  from_status report_status,
  to_status   report_status NOT NULL,
  note        text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
```

Catatan: pasang trigger `forbid_mutation` juga pada `report_events`. Kolom `updated_at` diperbarui oleh aplikasi.

### Tipe notifikasi

| `type` | Penerima | Pemicu |
|---|---|---|
| `BOOKING_CREATED` | semua `TU_KALAB` | booking baru |
| `BOOKING_APPROVED` / `BOOKING_REJECTED` / `BOOKING_CANCELLED` | peminjam | keputusan/pembatalan |
| `REPORT_CREATED` | semua `TU_KALAB` | laporan baru |
| `REPORT_STATUS_CHANGED` | pelapor | status laporan berubah |

Notifikasi dibuat **dalam transaksi yang sama** dengan perubahan status.

---

## 6. Kontrak API v1

Base path `/api/v1`. JSON `snake_case`. Detail lengkap dan skema ada di `api/openapi.yaml` (dibuat di Phase 0, dan menjadi acuan setelah disepakati).

### 6.1 Konvensi

- Waktu: ISO 8601 dengan offset, mis. `2026-10-20T08:00:00+07:00`. Server menyimpan UTC dan merespons dengan `+07:00`.
- Auth: `Authorization: Bearer <access_token>`. Access token 15 menit. **Refresh token hanya lewat cookie `httpOnly; Secure; SameSite=Strict; Path=/api/v1/auth`**, tidak pernah ada di body respons. Refresh dirotasi pada setiap pemakaian; pemakaian ulang token lama mencabut seluruh `family_id`.
- Paginasi: `?page=1&per_page=20` (maks 100). Respons list: `{ "data": [...], "meta": { "page", "per_page", "total" } }`.
- `POST /bookings` menerima header opsional `Idempotency-Key`.
- Format error seragam:

```json
{ "error": { "code": "BOOKING_CONFLICT", "message": "Ruang sudah terpakai pada rentang waktu tersebut.", "details": [], "request_id": "req_..." } }
```

| HTTP | `code` | Kapan |
|---|---|---|
| 400 | `BAD_REQUEST` | Body/query tidak bisa diparse |
| 401 | `UNAUTHENTICATED` | Token tidak ada/kedaluwarsa, kredensial salah |
| 403 | `FORBIDDEN` | Peran tidak berhak |
| 404 | `NOT_FOUND` | Resource tidak ada (atau bukan milik pengguna, untuk data privat) |
| 409 | `BOOKING_CONFLICT` | Rentang waktu bentrok |
| 409 | `INVALID_STATE` | Transisi status tidak diizinkan |
| 409 | `ROOM_INACTIVE`, `ROOM_HAS_ACTIVE_BOOKINGS` | BR-07 |
| 422 | `VALIDATION_ERROR` | Field/format/BR-01; `details` berisi `{ field, message }` |
| 429 | `RATE_LIMITED` | Login: 5 percobaan per 15 menit per (identifier + IP) |

### 6.2 Endpoint

| Method | Path | Akses | Fungsi |
|---|---|---|---|
| POST | `/auth/login` | publik | Login; set cookie refresh; body `{ access_token, expires_in, user }` |
| POST | `/auth/refresh` | cookie | Rotasi refresh, kembalikan access token baru |
| POST | `/auth/logout` | auth | Cabut refresh token, hapus cookie |
| GET | `/auth/me` | auth | Profil + peran |
| GET | `/users` | KAPRODI | Daftar (`role`, `q`, `is_active`) |
| POST | `/users` | KAPRODI | Buat pengguna |
| PATCH | `/users/{id}` | KAPRODI | Ubah data/peran/`is_active`; reset password |
| GET | `/rooms` | auth | Daftar (`type`, `capacity_min`, `q`, `is_active`) |
| GET | `/rooms/{id}` | auth | Detail |
| POST | `/rooms` | TU_KALAB, KAPRODI | Tambah |
| PATCH | `/rooms/{id}` | TU_KALAB, KAPRODI | Ubah |
| DELETE | `/rooms/{id}` | TU_KALAB, KAPRODI | Soft delete (BR-07) |
| GET | `/rooms/{id}/schedule?from=&to=` | auth | Timeline gabungan: jadwal + booking `DISETUJUI`; tiap item punya `kind` (`SCHEDULE` / `BOOKING`) |
| POST | `/schedules` | TU_KALAB, KAPRODI | Buat jadwal; opsional `repeat: { freq: "WEEKLY", until: "YYYY-MM-DD" }` |
| PATCH | `/schedules/{id}` | TU_KALAB, KAPRODI | Ubah (cek bentrok) |
| DELETE | `/schedules/{id}` | TU_KALAB, KAPRODI | Hapus |
| GET | `/availability?start_at=&end_at=&type=&capacity_min=` | auth | Status per ruang untuk rentang |
| POST | `/bookings` | MAHASISWA | Ajukan |
| GET | `/bookings` | auth | MAHASISWA: milik sendiri; TU_KALAB/KAPRODI: semua. Filter `status`, `room_id`, `from`, `to`, `mine` |
| GET | `/bookings/{id}` | pemilik, TU_KALAB, KAPRODI | Detail |
| GET | `/bookings/{id}/events` | pemilik, TU_KALAB, KAPRODI | Jejak status |
| POST | `/bookings/{id}/approve` | TU_KALAB | Setujui |
| POST | `/bookings/{id}/reject` | TU_KALAB | Body `{ reason }` |
| POST | `/bookings/{id}/cancel` | pemilik / TU_KALAB | Sesuai 4.1; body `{ reason? }` |
| GET | `/notifications?unread=` | auth | Milik sendiri |
| POST | `/notifications/{id}/read` | pemilik | Tandai dibaca |
| POST | `/notifications/read-all` | auth | Tandai semua |
| GET | `/notifications/stream` | auth | SSE (opsional; fallback polling 30 dtk) |
| POST | `/reports` | auth | `multipart/form-data`: `room_id`, `description`, `photo` (opsional) |
| GET | `/reports` | auth | Pelapor: milik sendiri; TU_KALAB/KAPRODI: semua |
| GET | `/reports/{id}` | pelapor, TU_KALAB, KAPRODI | Detail + riwayat |
| PATCH | `/reports/{id}/status` | TU_KALAB | `{ status, note? }` |
| GET | `/health` | publik | Liveness + cek DB |

Riwayat penggunaan (WBS 1.3.7) dilayani `GET /bookings` dengan filter; tidak ada endpoint tulis/hapus riwayat.

### 6.3 Contoh payload

`GET /availability?start_at=2026-10-20T08:00:00+07:00&end_at=2026-10-20T10:00:00+07:00&type=LAB`

```json
{ "data": [
  { "room": { "id": "…", "code": "LAB-1", "name": "Lab Jaringan", "type": "LAB", "capacity": 30 },
    "status": "TERSEDIA", "conflicts": [] },
  { "room": { "id": "…", "code": "LAB-2", "name": "Lab RPL", "type": "LAB", "capacity": 40 },
    "status": "DIGUNAKAN",
    "conflicts": [ { "kind": "SCHEDULE", "title": "Basis Data",
                     "start_at": "2026-10-20T07:00:00+07:00", "end_at": "2026-10-20T09:00:00+07:00" } ] }
] }
```

`POST /bookings`

```json
// request
{ "room_id": "…", "start_at": "2026-10-20T13:00:00+07:00", "end_at": "2026-10-20T15:00:00+07:00",
  "purpose": "Latihan kelompok ManPro", "participants": 8, "notes": "Butuh proyektor" }
// 201
{ "id": "…", "status": "MENUNGGU", "room": { "id": "…", "code": "LAB-1" },
  "start_at": "2026-10-20T13:00:00+07:00", "end_at": "2026-10-20T15:00:00+07:00",
  "created_at": "2026-10-12T09:30:00+07:00" }
// 409
{ "error": { "code": "BOOKING_CONFLICT", "message": "Ruang sudah terpakai pada rentang waktu tersebut.",
  "details": [ { "kind": "BOOKING", "status": "DISETUJUI", "start_at": "…", "end_at": "…" } ],
  "request_id": "req_…" } }
```

`POST /bookings/{id}/reject`: `{ "reason": "Ruang dipakai untuk ujian praktikum." }`

---

## 7. Frontend

Dasar: template shadcn-admin. Bahasa UI: Indonesia.

### 7.1 Pembersihan template (Phase 0, satu PR sendiri)

Hapus Clerk (`@clerk/react`, `src/routes/clerk`, `src/assets/clerk-logo.tsx`, entri sidebar), fitur demo `apps`, `chats`, `help-center`, halaman `sign-up`, `otp`, `forgot-password`, dan data faker yang tidak dipakai. Pertahankan layout, komponen `ui`, pola data-table (`users`, `tasks`) sebagai referensi. Ganti `auth-store` sesuai 6.1 (access token di memori, peran tunggal).

### 7.2 Peta halaman

| Route | Peran | Isi | API |
|---|---|---|---|
| `/sign-in` | publik | Login | `/auth/login` |
| `/` | semua | Ketersediaan: filter tanggal, jam, tipe; kartu/ tabel ruang + status | `/availability` |
| `/rooms/$id` | semua | Timeline harian/mingguan ruang (grid jam, CSS grid) + tombol ajukan | `/rooms/{id}/schedule` |
| `/bookings/new` | MAHASISWA | Form pengajuan (validasi Zod = BR-01) | `POST /bookings` |
| `/bookings` | semua | MAHASISWA: pengajuan saya + riwayat; lainnya: semua, filter | `/bookings` |
| `/approvals` | TU_KALAB | Antrean `MENUNGGU`, detail, setuju/tolak (dialog alasan) | `/bookings/{id}/approve`, `/bookings/{id}/reject` |
| `/notifications` | semua | Daftar, tandai dibaca; badge di header | `/notifications*` |
| `/reports` | semua | Buat laporan; daftar; TU_KALAB ubah status | `/reports*` |
| `/admin/rooms` | TU_KALAB, KAPRODI | CRUD ruang | `/rooms*` |
| `/admin/schedules` | TU_KALAB, KAPRODI | CRUD jadwal + jadwal berulang | `/schedules*` |
| `/admin/users` | KAPRODI | CRUD pengguna | `/users*` |

Guard peran lewat `beforeLoad` di route; sidebar berbeda per peran. Tampilkan error 409 konflik dengan jelas (ruang, jam yang bentrok).

---

## 8. Non-functional

| NFR | Target |
|---|---|
| Usability | Pengajuan maksimal 3 langkah; error per field jelas |
| Performance | `/availability` p95 ≤ 3 dtk (target internal ≤ 500 ms pada seed data) |
| Availability | `/health`, restart otomatis (`restart: unless-stopped`) |
| Security | RBAC server, argon2id, HTTPS, rate limit login, query berparameter, validasi upload (jpg/png/webp, ≤ 5 MB, cek magic bytes) |
| Reliability | Transaksi untuk operasi multi-tabel; backup harian `pg_dump` (skrip + dokumentasi restore) |
| Maintainability | Modul terpisah, migrasi berversi, OpenAPI sebagai kontrak, tes otomatis |

---

## 9. Urutan kerja dan acceptance

Satu phase = satu PR (atau beberapa PR kecil). Jangan mulai phase berikutnya sebelum yang sebelumnya lolos `make lint test`.

| Phase | Isi | Selesai bila |
|---|---|---|
| 0 | Scaffold monorepo, `docker-compose.yml` (Postgres), `Makefile`, config env, `/health`, `api/openapi.yaml` draft, migrasi awal, pembersihan template FE | `make up` jalan; `GET /health` 200; FE `pnpm build` hijau; OpenAPI valid |
| 1 | Auth + users + RBAC middleware + rate limit + seed | Login/refresh/logout benar; kredensial salah → 401; endpoint terlarang → 403; tes rotasi dan reuse refresh lulus |
| 2 | Rooms, schedules (+ berulang), availability, timeline | Jadwal bentrok ditolak (409); `/availability` benar untuk 3 status; tes BR-02 batas `[)` |
| 3 | Bookings: create/list/detail/approve/reject/cancel/events + notifikasi dalam transaksi | Semua transisi 4.1 diuji; tes paralel 20 request = 1 sukses; riwayat tidak bisa diubah |
| 4 | Notifications API (+SSE), Reports + upload + status | Notifikasi sampai ke penerima benar; upload tervalidasi |
| 5 | Frontend sesuai 7.2 (bisa paralel sejak OpenAPI beku) | Alur ujung-ke-ujung: login → cek ketersediaan → ajukan → approve → notifikasi → riwayat |
| 6 | Hardening, `docker-compose.prod.yml` + Caddy, backup script, README, smoke test E2E | Deploy bersih di mesin baru dengan satu perintah terdokumentasi |

### Seed data (dev)

Unit "Rumpun TI"; 1 akun per peran + 5 mahasiswa; 6 ruang (3 `KELAS`, 3 `LAB`); jadwal kuliah mingguan contoh. Password seed hanya untuk dev dan harus ditolak/dinonaktifkan di mode produksi.

---

## 10. Keputusan terbuka (pakai default, jangan blokir)

| # | Pertanyaan | Default |
|---|---|---|
| 1 | Booking `MENUNGGU` menahan slot? | Ya (BR-03) |
| 2 | Batas durasi/jam operasional/H+ maksimal | Nilai di BR-01, via env |
| 3 | Sumber jadwal kuliah | Dimasukkan admin lewat UI; import CSV ditunda (nice-to-have setelah Phase 6) |
| 4 | Login | Akun lokal; lapisan auth dipisah agar SSO bisa ditambah nanti |
| 5 | Aksi khusus OB selain melapor | Tidak ada di MVP |
| 6 | Approver kelas vs lab berbeda? | Satu peran `TU_KALAB` untuk semua |
| 7 | Notifikasi email/WhatsApp | Tidak; in-app saja |
| 8 | Perluasan fakultas | `org_unit_id` ada; filter per unit belum diterapkan |
