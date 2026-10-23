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
│   │   ├── auth/ users/ rooms/ schedules/ availability/
│   │   ├── bookings/ notifications/ reports/
│   │   ├── maintenance/ overlap/ seed/
│   │   └── platform/         # clock, db (pool + WithTx + kode sqlc), logger, password, testdb
│   ├── db/migrations/        # golang-migrate (*.up.sql / *.down.sql)
│   ├── db/queries/           # sqlc
│   └── openapi.yaml          # kontrak API (OpenAPI 3.1)
├── web/                      # frontend (Vite, React, TypeScript, TanStack, Tailwind v4)
├── docker-compose.yml        # Postgres untuk dev
├── deploy/                   # produksi: compose + Caddy (HTTPS), backup dan restore; lihat deploy/README.md
├── Makefile                  # perintah standar
└── .env.example              # contoh konfigurasi (salin ke .env)
```

Modul backend baru (`roomtypes`, `attachments`, `audit`, `imports`, `checkin`, `jobs`) dibuat pada phase yang membutuhkannya, masing-masing dengan lapisan `handler.go`, `service.go`, dan akses data (`sqlc`).

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
```

Dokumentasi lengkap (deploy, runbook, ERD) menyusul di Phase 7.
