# ClassLab.io: Master Task List & Progress Tracker

Dokumen ini melacak kemajuan implementasi proyek ClassLab.io per fase sesuai `brief.md` dan `agent.md`.

---

## 📌 Ringkasan Fase

- [x] **Phase 0: Scaffold Monorepo & Fondasi Dev** (Selesai)
- [ ] **Phase 1: Autentikasi, Manajemen Pengguna & RBAC**
- [ ] **Phase 2: Master Data Ruang, Jadwal Akademik & Ketersediaan**
- [ ] **Phase 3: Mesin Booking, Locking Konkurensi & Audit Trail**
- [ ] **Phase 4: Notifikasi In-App & Pelaporan Kerusakan**
- [ ] **Phase 5: Antarmuka Frontend (UI, Routing & Form)**
- [ ] **Phase 6: Hardening Produksi, Caddy, Backup & Dokumentasi**

---

## Detail Sub-Task

### Phase 0: Scaffold Monorepo & Fondasi Dev
- [x] **0.1 Inisialisasi Git & Workspace**
  - [x] `git init` repo lokal
  - [x] Buat file `.gitignore` komprehensif
  - [x] Setup struktur direktori proyek (`api/`, `web/`, `deploy/`, `docs/`)
- [x] **0.2 Setup Dokumentasi & Standar**
  - [x] Inisialisasi `docs/decisions.md`
  - [x] Inisialisasi `docs/tasks.md`
- [x] **0.3 Docker Compose & PostgreSQL Dev**
  - [x] Konfigurasi `docker-compose.yml` (Postgres 15+, btree_gist, healthcheck)
  - [x] Buat template `.env.example` dan `.env` lokal
- [x] **0.4 Backend Scaffold (Go)**
  - [x] Inisialisasi `api/go.mod`
  - [x] Setup dependensi dasar (`chi`, `pgx/v5`, `log/slog`, `jwt/v5`, `crypto`)
  - [x] Implementasi paket dasar di `api/internal/platform/` (db pool, logger JSON, clock)
  - [x] Implementasi endpoint `GET /health` (liveness + ping DB)
  - [x] Konfigurasi `sqlc.yaml`
- [x] **0.5 Skema Database & Migrasi Awal**
  - [x] Migrasi up `api/db/migrations/000001_init_schema.up.sql` (semua tabel DDL, constraint, trigger immutability)
  - [x] Migrasi down `api/db/migrations/000001_init_schema.down.sql`
  - [x] Verifikasi uji migrasi up dan down
- [x] **0.6 Kontrak API OpenAPI 3.1**
  - [x] Buat `api/openapi.yaml` mencakup seluruh endpoint, format error standar, dan skema request/response
- [x] **0.7 Frontend Scaffold & Pembersihan Template**
  - [x] Clone / setup template `satnaing/shadcn-admin` ke `web/`
  - [x] Pembersihan template: hapus Clerk, modul demo yang tidak terpakai
  - [x] Setup `auth-store` (access token in memory, refresh cookie)
  - [x] Integrasi generator tipe TypeScript dari OpenAPI (`pnpm run gen:api`)
  - [x] Verifikasi `pnpm build` hijau
- [x] **0.8 Automation & Tooling**
  - [x] Buat `Makefile` dan task helper script Windows (`scripts/task.ps1`)

---

### Phase 1: Autentikasi, Manajemen Pengguna & RBAC
- [ ] Implementasi argon2id password hashing
- [ ] Implementasi JWT token generator & verifier
- [ ] Implementasi refresh token rotation & reuse detection (family_id)
- [ ] Middleware HTTP: RequestID, slog JSON logger, auth bearer, RBAC checker
- [ ] Rate limiter login (5 attempts / 15 min per identifier + IP)
- [ ] Handler & Service: `POST /auth/login`, `POST /auth/refresh`, `POST /auth/logout`, `GET /auth/me`
- [ ] Handler & Service Pengguna: `GET /users`, `POST /users`, `PATCH /users/{id}`
- [ ] Seeder data dev (Rumpun TI, Kaprodi, TU/Kalab, OB, Mahasiswa)
- [ ] Automated tests: Auth, token lifecycle, RBAC matrix

---

### Phase 2: Master Data Ruang, Jadwal Akademik & Ketersediaan
- [ ] CRUD Ruang (`GET /rooms`, `GET /rooms/{id}`, `POST /rooms`, `PATCH /rooms/{id}`, `DELETE /rooms/{id}`)
- [ ] Soft delete ruang dengan validasi BR-07
- [ ] CRUD Jadwal Akademik (`/schedules`) dengan generator repeat mingguan (BR-09)
- [ ] Database exclusion constraints untuk pencegahan overlap jadwal
- [ ] Endpoint `GET /availability` (kalkulasi dinamis status TERSEDIA, DIGUNAKAN, DIAJUKAN)
- [ ] Endpoint `GET /rooms/{id}/schedule` (timeline gabungan jadwal + booking disetujui)
- [ ] Automated tests: Penanganan bentrok interval setengah-terbuka `[)`

---

### Phase 3: Mesin Booking, Locking Konkurensi & Audit Trail
- [ ] Handler & Service `POST /bookings` dengan validasi BR-01 & Idempotency-Key
- [ ] Concurrency Lock: `SELECT ... FOR UPDATE` pada tabel rooms dalam satu transaksi
- [ ] Verifikasi exclusion constraint database `bookings_no_overlap`
- [ ] Workflow Approval: Approve, Reject (alasan >= 5 huruf), Cancel
- [ ] Append-only `booking_events` terlindungi trigger database
- [ ] Notifikasi atomik dibuat dalam transaksi perubahan status
- [ ] Concurrency test: 20 request paralel hanya 1 berhasil dan 19 mendapat 409

---

### Phase 4: Notifikasi In-App & Pelaporan Kerusakan
- [ ] Endpoint notifikasi: list, unread filter, mark as read, mark all as read
- [ ] Endpoint streaming notifikasi SSE (`/notifications/stream`)
- [ ] Endpoint pelaporan kerusakan: multipart file upload, validasi magic bytes, batas 5MB
- [ ] Lifecycle status laporan kerusakan (BARU -> DITERIMA -> DIPROSES -> SELESAI)
- [ ] Append-only `report_events` & notifikasi status ke pelapor

---

### Phase 5: Antarmuka Frontend (UI, Routing & Form)
- [ ] Generate tipe TypeScript dari `api/openapi.yaml`
- [ ] Konfigurasi Axios client & refresh token interceptor
- [ ] Route tree TanStack Router & RBAC navigation guards
- [ ] Halaman login `/sign-in`
- [ ] Halaman utama `/` (ketersediaan ruang interaktif)
- [ ] Halaman timeline `/rooms/$id` (visual CSS grid)
- [ ] Halaman pengajuan booking `/bookings/new` (React Hook Form + Zod)
- [ ] Halaman riwayat booking `/bookings`
- [ ] Halaman approval `/approvals` (antrean & dialog penolakan)
- [ ] Halaman pelaporan kerusakan `/reports`
- [ ] Halaman admin ruang, jadwal, pengguna (`/admin/*`)
- [ ] Vitest unit & form tests, build check

---

### Phase 6: Hardening Produksi, Caddy, Backup & Dokumentasi
- [ ] Multi-stage production `Dockerfile` untuk backend Go
- [ ] Konfigurasi reverse proxy Caddy HTTPS otomatis (`deploy/Caddyfile`)
- [ ] `deploy/docker-compose.prod.yml`
- [ ] Script backup database `deploy/backup.sh` & runbook restore
- [ ] Smoke test end-to-end
- [ ] Dokumentasi lengkap di `README.md`
