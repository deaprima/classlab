<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg">
  <img src="assets/logo-light.svg" alt="Logo ClassLab" width="112" height="112">
</picture>

# ClassLab

**Sistem peminjaman ruang kampus yang bebas bentrok, transparan dalam persetujuan, dan akuntabel dalam pemakaian.**

Ruang kelas, laboratorium, dan ruang khusus dalam satu aplikasi web: real-time availability, validasi bentrok saat pengajuan, approval berdasarkan tipe ruang, check-in via QR, check-out dengan foto, serta pelaporan kerusakan.

[![CI](https://github.com/deaprima/classlab/actions/workflows/ci.yml/badge.svg)](https://github.com/deaprima/classlab/actions/workflows/ci.yml)
[![Lisensi: MIT](https://img.shields.io/badge/lisensi-MIT-EA580C.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](api/go.mod)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](web/package.json)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-4169E1?logo=postgresql&logoColor=white)](docker-compose.yml)
[![OpenAPI](https://img.shields.io/badge/OpenAPI-3.1-6BA539?logo=openapiinitiative&logoColor=white)](api/openapi.yaml)

[Quick start](#quick-start) · [Fitur](#fitur) · [Arsitektur](#arsitektur) · [Deployment](#deployment) · [Kontribusi](#kontribusi)

</div>

---

## Latar belakang

Peminjaman ruang kampus umumnya masih dilakukan secara manual. Informasi ketersediaan tersebar di chat dan spreadsheet, jadwal akademik sering tidak sesuai dengan kondisi aktual di ruangan, dan tidak ada jejak pengajuan yang dapat ditelusuri. ClassLab menggantikan proses tersebut dengan satu sistem yang menjamin empat hal:

- **Bebas bentrok.** Dua pengajuan tidak dapat memegang ruang dan jam yang sama, dan pengajuan tidak dapat menimpa jadwal akademik.
- **Persetujuan yang jelas.** Setiap tipe ruang memiliki aturan dan penyetuju masing-masing, dan status pengajuan selalu dapat dipantau.
- **Pemakaian aktual tercatat.** Ruang yang dipesan tetapi tidak dipakai dilepas secara otomatis, sedangkan ruang yang dipakai memiliki bukti check-in dan check-out.
- **Akuntabilitas.** Kerusakan dan pelanggaran dapat ditelusuri hingga ke pengajuan asalnya.

## Fitur

<table>
  <tr>
    <td width="33%" valign="top">
      <h3>Real-time availability</h3>
      Status <code>TERSEDIA</code>, <code>DIAJUKAN</code>, dan <code>DIGUNAKAN</code> per ruang dan per jam, ditampilkan sebagai timeline di desktop dan daftar slot di perangkat mobile.
    </td>
    <td width="33%" valign="top">
      <h3>Pencegahan bentrok berlapis</h3>
      Row lock pada ruang di service layer, constraint <code>EXCLUDE</code> di PostgreSQL, dan trigger yang memeriksa booking terhadap jadwal akademik. Diuji dengan 20 request paralel pada slot yang sama.
    </td>
    <td width="33%" valign="top">
      <h3>Approval per tipe ruang</h3>
      Auto-approve untuk mahasiswa di ruang kelas dengan riwayat yang baik, manual approval untuk laboratorium dan ruang khusus. Penyetuju, kuota, dan jam operasional dikelola admin sebagai policy, tanpa perlu redeploy.
    </td>
  </tr>
  <tr>
    <td width="33%" valign="top">
      <h3>Check-in QR, check-out foto</h3>
      Mahasiswa memindai QR di pintu ruangan untuk check-in dan memotret kondisi ruangan saat selesai. Pemesan yang tidak hadir dilepas otomatis oleh scheduled job.
    </td>
    <td width="33%" valign="top">
      <h3>Pelanggaran dan pelaporan</h3>
      Poin pelanggaran, suspensi sementara, laporan kerusakan dengan papan status, serta audit log bersifat append-only yang tidak dapat diubah.
    </td>
    <td width="33%" valign="top">
      <h3>Impor CSV</h3>
      Impor jadwal akademik dan akun mahasiswa dalam dua tahap (preview lalu apply), dilengkapi rollback. Jadwal akademik selalu diprioritaskan di atas pengajuan.
    </td>
  </tr>
  <tr>
    <td width="33%" valign="top">
      <h3>Enam peran</h3>
      Mahasiswa, Organisasi (dengan surat tugas), TU, Kalab, OB, dan Admin. Setiap peran memiliki halaman beranda dan menu tersendiri.
    </td>
    <td width="33%" valign="top">
      <h3>Mobile-first</h3>
      Diuji pada empat ukuran layar, dari 360 px hingga 1280 px, termasuk fitur kamera, dengan dukungan light dan dark mode.
    </td>
    <td width="33%" valign="top">
      <h3>Production-ready</h3>
      Docker Compose dengan Caddy (HTTPS otomatis), backup dan restore database, runbook penanganan gangguan, serta pembuatan admin pertama melalui satu perintah.
    </td>
  </tr>
</table>

## Tech stack

| Layer | Teknologi |
|---|---|
| **Frontend** | React 19, TypeScript (strict), Vite, TanStack Router / Query / Table, Tailwind CSS v4, shadcn/ui, react-hook-form, Zod |
| **Backend** | Go, `net/http` + `chi`, `pgx`, `sqlc`, autentikasi JWT dengan password hashing argon2id |
| **Database** | PostgreSQL 15+ (ekstensi `btree_gist`), migrasi dengan `golang-migrate` |
| **Kontrak API** | OpenAPI 3.1 (`api/openapi.yaml`); tipe frontend di-generate dari kontrak ini |
| **Deployment** | Docker, Docker Compose, Caddy |
| **Testing** | `testcontainers-go` (PostgreSQL asli), Vitest browser mode, Playwright |

## Arsitektur

ClassLab dibangun sebagai **modular monolith** dengan kontrak bertipe di antara kedua sisinya.

```
classlab/
├── api/                      # backend Go (net/http + chi, pgx, sqlc)
│   ├── cmd/server/           # main.go: hanya wiring
│   ├── cmd/seed/             # data development (ditolak di production)
│   ├── cmd/createadmin/      # admin pertama pada database baru (production)
│   ├── Dockerfile            # image API (multi-stage)
│   ├── internal/
│   │   ├── app/              # perakitan router dan dependency
│   │   ├── config/           # env -> struct, divalidasi saat startup
│   │   ├── httpx/            # middleware, pemetaan error, response
│   │   ├── auth/ users/ audit/ bootstrap/
│   │   ├── roomtypes/ rooms/ schedules/ availability/ imports/
│   │   ├── bookings/         # pengajuan, approval, ubah/batal, check-in dan check-out
│   │   ├── attachments/ notifications/ reports/ violations/
│   │   ├── jobs/ maintenance/ overlap/ seed/
│   │   └── platform/         # clock, db (pool + WithTx + kode sqlc), logger, password, ratelimit, testdb
│   ├── test/integration/     # integration test API terhadap PostgreSQL asli
│   ├── db/migrations/        # golang-migrate (*.up.sql / *.down.sql)
│   ├── db/queries/           # sqlc
│   └── openapi.yaml          # kontrak API (OpenAPI 3.1)
├── web/                      # frontend (Vite, React, TypeScript, TanStack, Tailwind v4)
│   └── e2e/                  # smoke test Playwright: responsif (API di-mock) dan end-to-end
├── docker-compose.yml        # Postgres untuk development
├── deploy/                   # production: compose + Caddy (HTTPS), backup dan restore
├── Makefile                  # perintah standar
└── .env.example              # contoh konfigurasi (salin ke .env)
```

Setiap modul backend menerapkan layering yang sama: `handler.go` (HTTP), `service.go` (business rules dan transaksi), dan data access (`sqlc`). Handler tidak mengakses database secara langsung.

Bentrok ruang dicegah secara berlapis: row lock pada ruang di service, constraint `EXCLUDE` untuk booking dan jadwal, serta trigger yang memeriksa booking terhadap jadwal (migrasi `000009`). Diagram tabel tersedia di [api/db/ERD.md](api/db/ERD.md).

## Quick start

### Prasyarat

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (untuk Postgres dan backend test)
- [Go](https://go.dev/dl/) sesuai versi pada [api/go.mod](api/go.mod)
- [Node.js](https://nodejs.org/) dan [pnpm](https://pnpm.io/installation)
- `make` (opsional, tetapi menyederhanakan seluruh langkah di bawah)

### Menjalankan di lokal

```sh
git clone https://github.com/deaprima/classlab.git
cd classlab

cp .env.example .env        # lalu isi JWT_SECRET (minimal 32 karakter), contoh: openssl rand -hex 32

make up                     # Postgres via Docker
make migrate                # terapkan migrasi
make seed                   # isi data development
make dev-api                # API di http://localhost:8080          (biarkan tetap berjalan)

# di terminal lain
cd web && pnpm install
pnpm run dev                # web di http://localhost:5173
```

Pastikan API berjalan dengan: `curl http://localhost:8080/health`.

<details>
<summary><b>Tanpa <code>make</code> (Windows PowerShell)</b></summary>

```powershell
Copy-Item .env.example .env             # lalu sesuaikan JWT_SECRET (minimal 32 karakter)
docker compose up -d                    # Postgres

# migrasi (golang-migrate via Docker)
$DB  = "postgres://classlab:classlab_secret@localhost:5432/classlab?sslmode=disable"
$MIG = "$PWD/api/db/migrations"
docker run --rm --network container:classlab-postgres -v "${MIG}:/migrations" migrate/migrate:v4.18.3 -path=/migrations -database $DB up

cd api ; go run ./cmd/seed ; go run ./cmd/server     # API di http://localhost:8080
cd web ; pnpm install ; pnpm run dev                  # web di http://localhost:5173 (terminal lain)
```

</details>

### Perintah standar

| Perintah | Fungsi |
|---|---|
| `make up` / `make down` | Menyalakan / mematikan Postgres development |
| `make migrate` / `make migrate-down` | Menerapkan / mengembalikan satu migrasi |
| `make seed` | Mengisi data development |
| `make dev-api` / `make dev-web` | Menjalankan API / frontend |
| `make gen` | Generate kode `sqlc` dan tipe frontend dari OpenAPI |
| `make lint` | `go vet`, `eslint`, `prettier --check` |
| `make test` | Backend test (PostgreSQL asli via testcontainers) dan frontend test |

## Verifikasi sebelum membuat pull request

```sh
make lint
make test                       # backend test membutuhkan Docker (testcontainers)
cd web && pnpm run build
cd web && pnpm run smoke:e2e    # alur pengajuan hingga riwayat pada API dan Postgres sungguhan
```

Frontend test berjalan di browser sungguhan. Pada penggunaan pertama, unduh Chromium: `cd web && pnpm exec playwright install chromium chromium-headless-shell`.

`smoke:e2e` membutuhkan container `classlab-postgres` yang aktif dan Go. Skrip ini membuat ulang database terpisah `classlab_e2e` (data development tidak terpengaruh), menjalankan API di port 8091 dan web di port 5199, kemudian mengeksekusi satu alur booking ruang Lab: pengajuan, approval Kalab, check-in melalui link QR, check-out dengan foto dari fake camera Chromium, dan riwayat. Jam server dan browser dikunci ke besok pukul 08:50 WIB melalui `DEV_CLOCK_START`, sehingga tes dapat dijalankan kapan saja. Screenshot dari langkah yang gagal disimpan di `SMOKE_SHOTS` (default: folder temp OS, `classlab-e2e`).

`DEV_CLOCK_START` (lihat `.env.example`) juga dapat dipakai untuk pengujian manual, misalnya untuk mencoba check-in di luar jam operasional. Server menolak untuk start apabila nilai ini diisi saat `APP_ENV=production`.

## Deployment

Pada server Linux dengan Docker, setelah mengisi `deploy/.env` (salin dari `deploy/.env.example`; isi `SITE_ADDRESS`, `POSTGRES_PASSWORD`, dan `JWT_SECRET`):

```sh
cd deploy
docker compose -f docker-compose.prod.yml up -d --build   # Postgres, migrasi, API, dan Caddy (HTTPS otomatis)

# admin pertama, cukup dijalankan sekali; password sementara dicetak satu kali dan wajib diganti saat login pertama
docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/createadmin api \
  -identifier ADMIN-01 -name "Nama Admin" -email admin@kampus.ac.id
```

Panduan lengkap, termasuk update, backup, dan restore, tersedia di [deploy/README.md](deploy/README.md).

## Dokumentasi

| Dokumen | Isi |
|---|---|
| [deploy/README.md](deploy/README.md) | Instalasi dan update di server, backup dan restore |
| [deploy/RUNBOOK.md](deploy/RUNBOOK.md) | Penanganan gangguan, rotasi secret, rollback |
| [api/db/ERD.md](api/db/ERD.md) | Diagram relasi tabel database |
| [api/openapi.yaml](api/openapi.yaml) | Kontrak API (OpenAPI 3.1) |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Panduan berkontribusi |
| [SECURITY.md](SECURITY.md) | Prosedur pelaporan celah keamanan |

## Kontribusi

Kontribusi sangat diterima. Baca [CONTRIBUTING.md](CONTRIBUTING.md) untuk alur kerja, standar kode, dan daftar tes yang wajib disertakan. Dengan berpartisipasi, Anda setuju untuk mematuhi [Code of Conduct](CODE_OF_CONDUCT.md).

Menemukan celah keamanan? Mohon jangan membuka issue publik; ikuti petunjuk pada [SECURITY.md](SECURITY.md).

## Acknowledgements

ClassLab dikembangkan sebagai proyek mata kuliah Manajemen Proyek (Kelompok 2). Kerangka antarmuka web berawal dari [shadcn-admin](https://github.com/satnaing/shadcn-admin) karya Sat Naing (lisensi MIT, lihat [web/LICENSE](web/LICENSE)), dan komponen dasarnya berasal dari [shadcn/ui](https://ui.shadcn.com).

## Lisensi

Dirilis di bawah lisensi [MIT](LICENSE).
