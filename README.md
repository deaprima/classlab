<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg">
  <img src="assets/logo-light.svg" alt="Logo ClassLab" width="112" height="112">
</picture>

# ClassLab

**Sistem peminjaman ruang kampus yang tidak bentrok, jelas persetujuannya, dan tercatat pemakaiannya.**

Ruang kelas, laboratorium, dan ruang khusus dalam satu aplikasi web: ketersediaan langsung, pengajuan dengan validasi bentrok, persetujuan menurut tipe ruang, check-in QR, check-out foto, dan laporan kerusakan.

[![CI](https://github.com/deaprima/classlab/actions/workflows/ci.yml/badge.svg)](https://github.com/deaprima/classlab/actions/workflows/ci.yml)
[![Lisensi: MIT](https://img.shields.io/badge/lisensi-MIT-EA580C.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](api/go.mod)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](web/package.json)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-4169E1?logo=postgresql&logoColor=white)](docker-compose.yml)
[![OpenAPI](https://img.shields.io/badge/OpenAPI-3.1-6BA539?logo=openapiinitiative&logoColor=white)](api/openapi.yaml)

[Mulai cepat](#mulai-cepat) · [Fitur](#fitur) · [Arsitektur](#arsitektur) · [Deploy](#deploy) · [Kontribusi](#kontribusi)

</div>

---

## Kenapa ClassLab

Peminjaman ruang kampus sering masih manual: ketersediaan tersebar di chat dan spreadsheet, jadwal akademik tidak selalu cocok dengan kenyataan di ruangan, dan tidak ada jejak pengajuan yang bisa ditelusuri. ClassLab mengganti semua itu dengan satu sistem yang menjaga empat hal:

- **Tidak ada bentrok.** Dua pengajuan tidak bisa memegang ruang dan jam yang sama, dan pengajuan tidak bisa menimpa jadwal akademik.
- **Persetujuan jelas.** Setiap tipe ruang punya aturan dan penyetujunya sendiri, dan status pengajuan selalu terlihat.
- **Pemakaian nyata tercatat.** Ruang yang dipesan tapi tidak dipakai dilepas otomatis, dan yang dipakai punya bukti check-in dan check-out.
- **Ada pertanggungjawaban.** Kerusakan dan pelanggaran bisa ditelusuri sampai ke pengajuannya.

## Fitur

<table>
  <tr>
    <td width="33%" valign="top">
      <h3>Ketersediaan langsung</h3>
      Status <code>TERSEDIA</code>, <code>DIAJUKAN</code>, dan <code>DIGUNAKAN</code> per ruang dan per jam, dengan tampilan timeline di desktop dan daftar slot di ponsel.
    </td>
    <td width="33%" valign="top">
      <h3>Anti bentrok berlapis</h3>
      Kunci baris ruang di service, constraint <code>EXCLUDE</code> di PostgreSQL, dan trigger untuk booking melawan jadwal. Diuji dengan 20 permintaan paralel pada slot yang sama.
    </td>
    <td width="33%" valign="top">
      <h3>Persetujuan per tipe ruang</h3>
      Otomatis untuk mahasiswa di ruang kelas bila riwayatnya baik, manual untuk lab dan ruang khusus. Penyetuju, kuota, dan jam operasional diatur admin sebagai kebijakan, tanpa deploy ulang.
    </td>
  </tr>
  <tr>
    <td width="33%" valign="top">
      <h3>Check-in QR, check-out foto</h3>
      Mahasiswa memindai QR di pintu untuk check-in dan memotret ruangan saat selesai. Pemesan yang tidak hadir dilepas otomatis oleh job terjadwal.
    </td>
    <td width="33%" valign="top">
      <h3>Pelanggaran dan laporan</h3>
      Poin pelanggaran, blokir sementara, laporan kerusakan dengan papan status, dan audit log yang hanya bisa ditambah, tidak bisa diubah.
    </td>
    <td width="33%" valign="top">
      <h3>Impor CSV</h3>
      Impor jadwal akademik dan akun mahasiswa dalam dua langkah (pratinjau lalu terapkan), dengan rollback. Jadwal akademik selalu didahulukan di atas pengajuan.
    </td>
  </tr>
  <tr>
    <td width="33%" valign="top">
      <h3>Enam peran</h3>
      Mahasiswa, Organisasi (dengan surat tugas), TU, Kalab, OB, dan Admin. Setiap peran punya beranda dan menu sendiri.
    </td>
    <td width="33%" valign="top">
      <h3>Mobile-first</h3>
      Diuji pada empat ukuran layar dari 360 px sampai 1280 px, termasuk kamera, dengan mode terang dan gelap.
    </td>
    <td width="33%" valign="top">
      <h3>Siap produksi</h3>
      Docker Compose dengan Caddy (HTTPS otomatis), cadangan dan pemulihan database, runbook gangguan, dan pembuatan admin pertama lewat satu perintah.
    </td>
  </tr>
</table>

## Teknologi

| Lapisan | Yang dipakai |
|---|---|
| **Frontend** | React 19, TypeScript (strict), Vite, TanStack Router / Query / Table, Tailwind CSS v4, shadcn/ui, react-hook-form, Zod |
| **Backend** | Go, `net/http` + `chi`, `pgx`, `sqlc`, JWT dengan hash password argon2id |
| **Database** | PostgreSQL 15+ (ekstensi `btree_gist`), migrasi dengan `golang-migrate` |
| **Kontrak** | OpenAPI 3.1 (`api/openapi.yaml`), tipe frontend di-generate dari kontrak ini |
| **Deploy** | Docker, Docker Compose, Caddy |
| **Pengujian** | `testcontainers-go` (PostgreSQL asli), Vitest browser mode, Playwright |

## Arsitektur

ClassLab adalah **monolit modular** dengan kontrak bertipe antara dua sisinya.

```
classlab/
├── api/                      # backend Go (net/http + chi, pgx, sqlc)
│   ├── cmd/server/           # main.go: hanya wiring
│   ├── cmd/seed/             # data dev (ditolak di produksi)
│   ├── cmd/createadmin/      # admin pertama di database baru (produksi)
│   ├── Dockerfile            # image API (multi-stage)
│   ├── internal/
│   │   ├── app/              # perakitan router, dependency, dan tes integrasi
│   │   ├── config/           # env -> struct, divalidasi saat start
│   │   ├── httpx/            # middleware, pemetaan error, respons
│   │   ├── auth/ users/ audit/ bootstrap/
│   │   ├── roomtypes/ rooms/ schedules/ availability/ imports/
│   │   ├── bookings/         # pengajuan, approval, ubah/batal, check-in dan check-out
│   │   ├── attachments/ notifications/ reports/ violations/
│   │   ├── jobs/ maintenance/ overlap/ seed/
│   │   └── platform/         # clock, db (pool + WithTx + kode sqlc), logger, password, ratelimit, testdb
│   ├── db/migrations/        # golang-migrate (*.up.sql / *.down.sql)
│   ├── db/queries/           # sqlc
│   └── openapi.yaml          # kontrak API (OpenAPI 3.1)
├── web/                      # frontend (Vite, React, TypeScript, TanStack, Tailwind v4)
│   └── e2e/                  # smoke test Playwright: responsif (API di-mock) dan ujung ke ujung
├── docker-compose.yml        # Postgres untuk dev
├── deploy/                   # produksi: compose + Caddy (HTTPS), backup dan restore
├── Makefile                  # perintah standar
└── .env.example              # contoh konfigurasi (salin ke .env)
```

Setiap modul backend punya lapisan `handler.go` (HTTP), `service.go` (aturan bisnis dan transaksi), dan akses data (`sqlc`). Handler tidak memanggil database langsung.

Bentrok ruang dijaga berlapis: kunci baris ruang di service, constraint `EXCLUDE` untuk booking dan untuk jadwal, serta trigger untuk booking melawan jadwal (migrasi `000009`). Diagram tabelnya ada di [api/db/ERD.md](api/db/ERD.md).

## Mulai cepat

### Prasyarat

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (untuk Postgres dan tes backend)
- [Go](https://go.dev/dl/) sesuai versi di [api/go.mod](api/go.mod)
- [Node.js](https://nodejs.org/) dan [pnpm](https://pnpm.io/installation)
- `make` (opsional, tapi menyederhanakan semua langkah di bawah)

### Menjalankan di lokal

```sh
git clone https://github.com/deaprima/classlab.git
cd classlab

cp .env.example .env        # lalu isi JWT_SECRET (minimal 32 karakter), misalnya: openssl rand -hex 32

make up                     # Postgres lewat Docker
make migrate                # terapkan migrasi
make seed                   # isi data dev
make dev-api                # API di http://localhost:8080          (biarkan berjalan)

# terminal lain
cd web && pnpm install
pnpm run dev                # web di http://localhost:5173
```

Cek bahwa API hidup: `curl http://localhost:8080/health`.

<details>
<summary><b>Tanpa <code>make</code> (Windows PowerShell)</b></summary>

```powershell
Copy-Item .env.example .env             # lalu sesuaikan JWT_SECRET (min. 32 karakter)
docker compose up -d                    # Postgres

# migrasi (golang-migrate lewat Docker)
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
| `make up` / `make down` | Menyalakan / mematikan Postgres dev |
| `make migrate` / `make migrate-down` | Menerapkan / memundurkan satu migrasi |
| `make seed` | Mengisi data dev |
| `make dev-api` / `make dev-web` | Menjalankan API / frontend |
| `make gen` | Generate kode `sqlc` dan tipe frontend dari OpenAPI |
| `make lint` | `go vet`, `eslint`, `prettier --check` |
| `make test` | Tes backend (PostgreSQL asli lewat testcontainers) dan tes frontend |

## Verifikasi sebelum mengirim perubahan

```sh
make lint
make test                       # tes backend memakai Docker (testcontainers)
cd web && pnpm run build
cd web && pnpm run smoke:e2e    # alur pengajuan sampai riwayat lewat API dan Postgres sungguhan
```

Tes frontend berjalan di browser sungguhan. Untuk pertama kali, unduh Chromium: `cd web && pnpm exec playwright install chromium chromium-headless-shell`.

`smoke:e2e` butuh container `classlab-postgres` yang menyala dan Go. Skrip ini membuat ulang database terpisah `classlab_e2e` (data dev tidak tersentuh), menjalankan API di port 8091 dan web di port 5199, lalu menjalankan satu booking ruang Lab: ajukan, approve Kalab, check-in lewat tautan QR, check-out dengan foto dari kamera palsu Chromium, dan riwayat. Jam server dan browser diset ke besok 08:50 WIB lewat `DEV_CLOCK_START`, jadi tes bisa dijalankan kapan saja. Screenshot langkah yang gagal disimpan di `SMOKE_SHOTS` (bawaan: folder temp OS, `classlab-e2e`).

`DEV_CLOCK_START` (lihat `.env.example`) juga bisa dipakai saat mencoba aplikasi secara manual, misalnya agar check-in bisa dicoba di luar jam operasional. Server menolak start bila nilai ini diisi saat `APP_ENV=production`.

## Deploy

Di server Linux dengan Docker, setelah mengisi `deploy/.env` (salin dari `deploy/.env.example`; isi `SITE_ADDRESS`, `POSTGRES_PASSWORD`, `JWT_SECRET`):

```sh
cd deploy
docker compose -f docker-compose.prod.yml up -d --build   # Postgres, migrasi, API, dan Caddy (HTTPS otomatis)

# admin pertama, sekali saja; password sementara dicetak sekali dan wajib diganti saat masuk
docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/createadmin api \
  -identifier ADMIN-01 -name "Nama Admin" -email admin@kampus.ac.id
```

Langkah lengkap, termasuk memperbarui, cadangan, dan pemulihan, ada di [deploy/README.md](deploy/README.md).

## Dokumentasi

| Dokumen | Isi |
|---|---|
| [deploy/README.md](deploy/README.md) | Memasang dan memperbarui di server, cadangan dan pemulihan |
| [deploy/RUNBOOK.md](deploy/RUNBOOK.md) | Apa yang dilakukan saat ada gangguan, rotasi rahasia, rollback |
| [api/db/ERD.md](api/db/ERD.md) | Diagram tabel database |
| [api/openapi.yaml](api/openapi.yaml) | Kontrak API (OpenAPI 3.1) |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Cara berkontribusi |
| [SECURITY.md](SECURITY.md) | Cara melaporkan celah keamanan |

## Kontribusi

Kontribusi sangat diterima. Baca [CONTRIBUTING.md](CONTRIBUTING.md) untuk alur kerja, standar kode, dan daftar tes yang wajib ada. Dengan ikut serta, Anda setuju mengikuti [Kode Etik](CODE_OF_CONDUCT.md).

Menemukan celah keamanan? Jangan buka issue publik; ikuti petunjuk di [SECURITY.md](SECURITY.md).

## Penghargaan

ClassLab dibuat sebagai proyek mata kuliah Manajemen Proyek (Kelompok 2). Kerangka antarmuka web berawal dari [shadcn-admin](https://github.com/satnaing/shadcn-admin) karya Sat Naing (lisensi MIT, lihat [web/LICENSE](web/LICENSE)), dan komponen dasarnya berasal dari [shadcn/ui](https://ui.shadcn.com).

## Lisensi

Dirilis dengan lisensi [MIT](LICENSE).
