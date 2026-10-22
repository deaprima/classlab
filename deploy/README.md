# Deploy ClassLab ke server

Satu server Linux dengan Docker (Compose v2), satu domain, dan satu perintah. Isi folder ini:

| Berkas | Fungsi |
|---|---|
| `docker-compose.prod.yml` | Postgres, migrasi, API, dan Caddy (HTTPS otomatis) |
| `Caddyfile` | Satu alamat untuk aplikasi dan `/api`, header keamanan, batas unggahan |
| `.env.example` | Pengaturan yang harus diisi (`.env` tidak ikut git) |
| `backup.sh` | Cadangan database dan berkas unggahan |
| `restore-test.sh` | Membuktikan cadangan bisa dipulihkan (tanpa menyentuh aplikasi) |
| `restore.sh` | Memulihkan cadangan ke server |

Nama proyek Compose-nya `classlab-prod`, terpisah dari `docker compose` di root repo (dev), jadi keduanya
tidak saling menimpa. Volume: `classlab-prod_postgres_data`, `classlab-prod_uploads`, `classlab-prod_caddy_data`.

## Prasyarat

- DNS domain sudah menunjuk ke server; port **80 dan 443** terbuka (Caddy butuh keduanya untuk mengambil sertifikat).
- Docker dan Compose v2 terpasang.
- Repo ini ada di server (`git clone`), semua perintah di bawah dijalankan dari folder `deploy/`.

## Pertama kali

```sh
cp .env.example .env
# isi SITE_ADDRESS, lalu buat dua rahasia dan tempelkan ke .env:
openssl rand -hex 32     # POSTGRES_PASSWORD
openssl rand -hex 32     # JWT_SECRET

docker compose -f docker-compose.prod.yml up -d --build
docker compose -f docker-compose.prod.yml ps        # semua "running"/"healthy", migrate "exited (0)"
```

Migrasi database berjalan sendiri (layanan `migrate`) sebelum API menyala.

**Akun admin pertama.** Seed dev ditolak di produksi, jadi buat admin dengan perintah ini (sekali saja):

```sh
docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/createadmin api \
  -identifier ADMIN-01 -name "Nama Admin" -email admin@kampus.ac.id
```

Password sementara dicetak **sekali** di terminal dan harus diganti saat masuk pertama. Admin lalu
membuat akun TU, Kalab, OB, dan mahasiswa dari aplikasi, atau lewat Impor CSV.

Buka `https://<SITE_ADDRESS>`: halaman landing tampil untuk tamu.

## Memperbarui

```sh
git pull
docker compose -f docker-compose.prod.yml up -d --build
```

Migrasi baru ikut berjalan. Data tidak terhapus selama volume tidak dihapus: **jangan pernah** memakai
`docker compose down -v` di server ini.

## Cadangan

Yang tidak bisa dibuat ulang ada dua: database dan berkas unggahan (surat dan foto check-out).
`backup.sh` menyimpan keduanya ke `deploy/backups/` (atau `BACKUP_DIR`) dan menghapus yang lebih tua
dari 14 hari (`RETENTION_DAYS`). Cadangan hanya disimpan setelah berhasil dibaca ulang.

Jadwalkan tiap hari, mis. `crontab -e`:

```cron
0 2 * * * /opt/classlab/deploy/backup.sh >> /var/log/classlab-backup.log 2>&1
```

**Salin cadangan keluar dari server** (rsync/rclone ke penyimpanan lain). Cadangan yang hanya ada di
disk yang sama ikut hilang bersama disknya.

### Menguji pemulihan

```sh
./restore-test.sh                 # cadangan terbaru
./restore-test.sh backups/db-20261022-020000.dump
```

Dump dimuat ke Postgres sementara, versi migrasi dan jumlah baris tiap tabel utama dicetak, lalu
container sementara dihapus. Aplikasi yang berjalan tidak tersentuh. Jalankan sebulan sekali dan
setiap kali skema berubah besar.

### Memulihkan (server atau disk hilang)

1. Siapkan server baru sampai langkah "Pertama kali" (tanpa membuat admin), dengan `.env` yang sama.
2. Letakkan berkas cadangan di `deploy/backups/`.
3. Jalankan:

   ```sh
   ./restore.sh backups/db-<stempel>.dump backups/uploads-<stempel>.tar.gz
   ```

   Skrip meminta kamu mengetik `restore`, menghentikan aplikasi, **mengganti** database dan unggahan,
   lalu menyalakan lagi.
4. Masuk, buka satu pengajuan dan satu foto check-out untuk memastikan.

## Memeriksa

```sh
docker compose -f docker-compose.prod.yml logs -f api
docker compose -f docker-compose.prod.yml ps
```

API menjawab cek hidup di `GET /health` dari dalam jaringan Docker (tidak dibuka ke luar).
