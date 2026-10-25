# ClassLab

Sistem web peminjaman ruang kampus (ruang kelas, laboratorium, dan ruang khusus): ketersediaan ruang, pengajuan dengan validasi bentrok, persetujuan menurut tipe ruang, check-in QR, check-out foto, dan laporan kerusakan. Proyek Manajemen Proyek, Kelompok 2.

## Struktur repo

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
├── deploy/                   # produksi: compose + Caddy (HTTPS), backup dan restore; lihat deploy/README.md
├── Makefile                  # perintah standar
└── .env.example              # contoh konfigurasi (salin ke .env)
```

Setiap modul backend punya lapisan `handler.go`, `service.go`, dan akses data (`sqlc`). Bentrok ruang dijaga berlapis: kunci baris ruang di service, constraint `EXCLUDE` untuk booking dan untuk jadwal, serta trigger untuk booking vs jadwal (migrasi 000009).

## Prasyarat

Docker Desktop, Go (versi di `api/go.mod`), Node.js dan `pnpm`.

## Mulai cepat

Perintah di bawah dijalankan dari root repo. Bila `make` tersedia, `make up`, `make migrate`, `make seed`, `make dev-api`, `make dev-web`, `make lint`, `make test`, dan `make gen` setara dengan perintah manual.

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

Cek hidup: `GET http://localhost:8080/health`.

## Verifikasi sebelum selesai

```powershell
cd api ; go vet ./... ; go test ./... ; cd ..          # tes backend memakai Docker (testcontainers)
cd web ; pnpm run lint ; pnpm run format:check ; pnpm run test ; pnpm run build
cd web ; pnpm run smoke:e2e                            # alur pengajuan sampai riwayat lewat API dan Postgres sungguhan
```

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

Dokumentasi:

- [deploy/README.md](deploy/README.md): memasang dan memperbarui di server, cadangan dan pemulihan.
- [deploy/RUNBOOK.md](deploy/RUNBOOK.md): apa yang dilakukan saat ada gangguan, rotasi rahasia, rollback.
- [api/db/ERD.md](api/db/ERD.md): diagram tabel database.
