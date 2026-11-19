# Runbook ClassLab

Untuk orang yang menjaga server produksi. Pemasangan awal, pembaruan, dan cadangan ada di
[README.md](README.md); berkas ini berisi apa yang dilakukan **saat ada yang tidak beres** dan pekerjaan
berkala. Semua perintah dijalankan dari folder `deploy/` di server.

```sh
alias dc='docker compose -f docker-compose.prod.yml'
```

## Gambaran singkat

| Layanan | Tugas | Data yang tidak bisa dibuat ulang |
|---|---|---|
| `caddy` | HTTPS, berkas aplikasi (SPA), meneruskan `/api` ke API | volume `caddy_data` (sertifikat) |
| `api` | Server Go; cek hidup `GET /health` di dalam jaringan Docker | volume `uploads` (surat, foto check-out) |
| `postgres` | Database | volume `postgres_data` |
| `migrate` | Menjalankan migrasi lalu berhenti | - |

Hanya port 80 dan 443 (Caddy) yang terbuka ke luar. API dan Postgres tidak punya port yang dipublikasikan.

## Pekerjaan berkala

| Kapan | Apa |
|---|---|
| Harian | `backup.sh` lewat cron, dan salinan cadangan keluar dari server (lihat README) |
| Bulanan | `./restore-test.sh`: bukti bahwa cadangan bisa dipulihkan |
| Tiap rilis | `git pull`, lalu `dc up -d --build`; periksa `dc ps` dan `dc logs --since 5m api` |
| Triwulan | Perbarui dependensi dan versi image (di bawah), lalu jalankan CI (`go test`, `govulncheck`, `pnpm audit --prod`) |
| Sesuai kebutuhan | Rotasi `JWT_SECRET` (di bawah), terutama bila bocor |

## Memeriksa kesehatan

```sh
dc ps                              # semua "running"/"healthy", migrate "exited (0)"
dc logs --since 10m api            # JSON per baris; cari "level":"ERROR"
dc logs --since 10m caddy          # sertifikat dan galat proxy
df -h                              # disk server (unggahan dan volume Docker tumbuh)
docker system df                   # ruang yang dipakai image dan volume
```

Setiap respons membawa `X-Request-Id` (juga ada di badan galat dan di tiap baris log). Kalau pengguna
melaporkan galat, minta nilai itu lalu cari: `dc logs api | grep req_...`.

## Gejala dan tindakan

**Situs tidak terbuka sama sekali.**
`dc ps`. Jika `caddy` mati: `dc logs caddy`. Penyebab umum: port 80/443 dipakai proses lain, atau sertifikat
gagal diambil (DNS belum menunjuk ke server, port 80 tertutup). Jika `caddy` tidak mau naik karena `api`:
lihat baris berikut.

**`api` tidak sehat atau terus restart.**
`dc logs --tail 100 api`. Server menolak menyala bila `JWT_SECRET` kurang dari 32 karakter, `COOKIE_SECURE`
tidak `true` di produksi, `CORS_ALLOWED_ORIGINS` salah bentuk, atau `TRUSTED_PROXIES` bukan IP/CIDR yang sah.
Pesannya menyebut nama variabel yang salah.

**Halaman tampil tetapi semua permintaan API gagal (502).**
Caddy tidak bisa menjangkau API: `dc ps api`, `dc logs api`. Mulai ulang hanya API: `dc restart api`.

**Pengguna mendapat 429 saat login.**
Batas percobaan login berlaku per alamat pengunjung. Itu perilaku normal setelah banyak salah password;
tunggu sesuai header `Retry-After`. Jika **banyak pengguna berbeda** terkena sekaligus, curigai alamat yang
terbaca salah (semua tampak dari satu alamat): lihat `remote_addr` di log API. Isinya harus alamat pengunjung,
bukan alamat internal Docker (`172.x`). Jika yang tampil alamat internal, periksa `header_up X-Real-IP` di
`Caddyfile` dan `TRUSTED_PROXIES` di `docker-compose.prod.yml`.

**Unggahan gagal.**
Jika jawabannya 429 `RATE_LIMITED`: satu pengguna dibatasi `UPLOAD_RATE_PER_MIN` (bawaan 10) unggahan surat dan
foto per menit, dan `IMPORT_RATE_PER_MIN` (bawaan 30) untuk impor CSV (TU dan admin; pratinjau tidak dihitung,
hanya langkah terapkan). Tunggu sesuai `Retry-After`, atau naikkan nilainya di `.env` lalu `dc up -d api`.
Batas satu berkas 5 MB (`MAX_UPLOAD_MB`), Caddy menolak badan di atas 8 MB. Jika semua unggahan gagal,
periksa `df -h` dan izin volume `uploads` (API berjalan sebagai pengguna `app`, uid 10001).

**Disk hampir penuh.**
Pemakai terbesar biasanya `uploads`, cadangan di `backups/`, dan log Docker. Cadangan lama dihapus
`backup.sh` (14 hari). Jangan memakai `docker system prune --volumes` dan jangan `docker compose down -v`:
keduanya menghapus data.

**Kamera tidak berfungsi di check-in/check-out.**
Kamera butuh HTTPS dan header `Permissions-Policy: camera=(self)`. Pastikan situs dibuka lewat `https://`
dan header itu tidak dihapus oleh proxy di depan Caddy.

**Halaman kosong setelah rilis, konsol penuh galat CSP.**
`Content-Security-Policy` ada di `Caddyfile`. Skrip dari domain lain, skrip inline, dan `eval` diblokir
sengaja. Jika fitur baru butuh sumber lain, tambahkan ke kebijakan itu; jangan melonggarkan `script-src`.

## Rotasi `JWT_SECRET`

Rahasia ini hanya menandatangani access token (berumur 15 menit). Refresh token disimpan di database
sebagai hash dan tidak terpengaruh, jadi pengguna tidak perlu login ulang: aplikasi meminta access token
baru sendiri.

```sh
openssl rand -hex 32          # tempel ke JWT_SECRET di .env
dc up -d api                  # hanya API yang dibuat ulang
```

Access token lama langsung ditolak dan diganti otomatis dalam beberapa detik.

## Mengganti password database

Mengubah `POSTGRES_PASSWORD` di `.env` **tidak** mengubah password pengguna yang sudah ada di database (nilai
itu hanya dipakai saat volume dibuat pertama kali). Urutannya:

```sh
# 1. ubah di dalam Postgres
dc exec postgres psql -U classlab -d classlab -c "ALTER USER classlab PASSWORD 'PASSWORD_BARU_HEX'"
# 2. ubah POSTGRES_PASSWORD di .env ke nilai yang sama, lalu
dc up -d api
```

Gunakan hex saja (`openssl rand -hex 16`): password ini berada di dalam URL koneksi.

## Memperbarui versi image

Image dipasang pada versi persis supaya build hari ini dan setahun lagi sama. Konsekuensinya, patch
keamanan tidak masuk sendiri; naikkan dengan sengaja:

| Image | Ada di |
|---|---|
| `golang:1.26.x-alpine` (samakan dengan `go` di `api/go.mod`) | `api/Dockerfile` |
| `alpine:3.21.x` | `api/Dockerfile` |
| `node:22.x-alpine` | `web/Dockerfile` |
| `caddy:2.x-alpine` | `web/Dockerfile` |
| `postgres:16.x-alpine` (tetap di versi mayor 16 supaya data lama terbaca) | `docker-compose.yml`, `deploy/docker-compose.prod.yml`, `deploy/restore-test.sh`, `api/internal/platform/testdb/testdb.go` |
| `migrate/migrate:vX.Y.Z` | `deploy/docker-compose.prod.yml` |

Cari tag terbaru di Docker Hub, ganti, jalankan CI, lalu `dc up -d --build`. Versi Go yang rentan
ditunjukkan `govulncheck`; versi lain ikuti pengumuman rilis keamanannya.

## Membatalkan rilis (rollback)

Kode:

```sh
git log --oneline -5
git checkout <commit-sebelumnya>
dc up -d --build
```

Kalau rilis itu menambah migrasi, kode lama berjalan di skema yang lebih baru. Biasanya aman bila migrasinya
hanya menambah kolom atau tabel; jika tidak, kembalikan skema juga:

1. **Cadangkan dulu:** `./backup.sh`.
2. Turunkan satu migrasi (isi variabel dari `.env` terlebih dahulu):

   ```sh
   set -a; . ./.env; set +a
   dc run --rm migrate -path=/migrations \
     -database="postgres://classlab:${POSTGRES_PASSWORD}@postgres:5432/classlab?sslmode=disable" down 1
   ```

Migrasi `down` bisa menghapus data di kolom atau tabel yang dibuang. Bila ragu, pulihkan dari cadangan
(bagian berikut) daripada menurunkan migrasi.

## Pemulihan

Server atau disk hilang, atau data rusak: ikuti **Memulihkan** di [README.md](README.md). Latih
sebulan sekali dengan `./restore-test.sh`; cadangan yang belum pernah dicoba dipulihkan belum bisa dianggap
cadangan.

## Insiden keamanan

- **Dugaan password seorang pengguna bocor:** admin lain mereset password-nya dari aplikasi (password
  sementara dibuat, pengguna wajib menggantinya) atau menonaktifkan akunnya; keduanya mencabut **semua**
  sesi pengguna itu. Jika pengguna mengganti password sendiri, hanya sesi di perangkat lain yang dicabut;
  sesi yang dipakai mengganti tetap berjalan. Periksa **Audit Log** (menu Admin) untuk aksi yang tidak
  dikenal. Log mencatat alamat IP dan user agent.
- **Satu-satunya admin terkunci:** buat admin baru dengan `createadmin` (lihat README, "Akun admin pertama").
- **`JWT_SECRET` atau `.env` bocor:** rotasi `JWT_SECRET` dan password database (bagian di atas), lalu
  periksa Audit Log.
- **Cadangan bocor:** cadangan memuat hash password dan seluruh data. Anggap semua password harus diganti
  dan hapus cadangan itu dari tempat yang tidak seharusnya.
- Simpan `X-Request-Id` dan potongan log sebelum memulai ulang layanan; log kontainer hilang bila
  kontainernya dibuat ulang.

## Menambah Cloudflare di depan situs

Belum dipakai. Saat domain sudah ada, lihat catatan di README (bagian "Alamat IP pengunjung"): Caddy harus
mempercayai rentang IP Cloudflare dan membaca `CF-Connecting-IP`, kalau tidak rate limit login melihat alamat
Cloudflare, bukan alamat pengunjung.
