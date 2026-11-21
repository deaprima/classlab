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
| `CHECKLIST.md` | Checklist go-live dan langkah tiap rilis |

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

**Salin cadangan keluar dari server.** Cadangan yang hanya ada di disk yang sama ikut hilang bersama
disknya. `backup.sh` bisa melakukannya sendiri lewat [rclone](https://rclone.org/) (S3, Cloudflare R2,
Google Drive, SFTP, dan lainnya). Setelah `rclone config` membuat sebuah remote:

```cron
0 2 * * * BACKUP_REMOTE=r2:classlab-backups BACKUP_PING_URL=https://hc-ping.com/<uuid> /opt/classlab/deploy/backup.sh >> /var/log/classlab-backup.log 2>&1
```

- `BACKUP_REMOTE`: tujuan rclone. Berkas lama di tujuan **tidak** dihapus oleh skrip; atur aturan lifecycle
  di bucket, atau jalankan `rclone delete --min-age 30d <remote>` secara berkala.
- `BACKUP_PING_URL`: alamat pemantau bergaya healthchecks.io. Skrip memanggilnya saat berhasil dan
  `<alamat>/fail` saat gagal; kalau tidak ada ping masuk, pemantau memberi tahu Anda. Keduanya opsional.
- `deploy/.env` tidak termasuk cadangan. Simpan salinannya sendiri (password manager).

#### Contoh: Google Drive (sementara)

Server biasanya tidak punya browser, jadi login Google dilakukan di laptop Anda, lalu tokennya dipindahkan.

1. **Di server**: `curl https://rclone.org/install.sh | sudo bash` (atau `apt install rclone`), lalu `rclone config`.
2. Pilih `n` (new remote), beri nama `gdrive`, storage `drive`.
3. `client_id` dan `client_secret`: buat sendiri di Google Cloud Console (OAuth client jenis *Desktop app*). Kalau dikosongkan, rclone memakai kredensial bersama yang sering kena batas laju. Rinciannya: <https://rclone.org/drive/#making-your-own-client-id>.
4. `scope`: pilih **`drive.file`** (rclone hanya bisa melihat berkas yang dibuatnya sendiri, tidak seluruh isi Drive Anda).
5. Pada pertanyaan *Use web browser to automatically authenticate?* jawab **`n`**. Rclone menampilkan perintah `rclone authorize "drive" "<kode>"`. Jalankan perintah itu di **laptop** (rclone terpasang di sana juga), login Google, lalu tempel token yang dihasilkan ke server.
6. Uji: `rclone mkdir gdrive:classlab-backups && rclone lsd gdrive:`

**Enkripsi (sangat disarankan).** Cadangan berisi data pengguna dan foto ruang, dan Google Drive adalah pihak ketiga. Tambahkan remote `crypt` di atas Drive supaya isinya terenkripsi sebelum diunggah: di `rclone config` buat remote baru bertipe `crypt` dengan tujuan `gdrive:classlab-backups`, nama `gdrive-crypt`, dan buat dua password (password dan salt). **Simpan password itu di password manager**: tanpa itu cadangan tidak bisa dibuka lagi. Lalu pakai `BACKUP_REMOTE=gdrive-crypt:`.

**Pembersihan.** Skrip tidak menghapus berkas lama di Drive, dan setiap hari satu arsip unggahan penuh ikut terkirim, jadi kuota Drive (15 GB gratis) cepat terpakai. Tambahkan cron mingguan:

```cron
0 3 * * 0 rclone delete --min-age 30d gdrive-crypt: >> /var/log/classlab-backup.log 2>&1
```

**Pulihkan dari Drive:** `rclone copy gdrive-crypt: deploy/backups/ --include "db-<stempel>.dump" --include "uploads-<stempel>.tar.gz"`, lalu ikuti langkah "Memulihkan".

Token Drive tersimpan di `~/.config/rclone/rclone.conf` di server (`chmod 600`). Siapa pun yang memegangnya bisa mengakses cadangan; karena itu enkripsi di atas penting.

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

## Alamat IP pengunjung

Caddy mengisi header `X-Real-IP` dengan alamat pengunjung, dan API hanya mempercayainya dari rentang
`TRUSTED_PROXIES` (sudah diisi di `docker-compose.prod.yml`). Tanpa ini semua pengunjung tampak berasal dari
satu alamat (Caddy), sehingga batas percobaan login dipakai bersama. Alamat ini dipakai untuk batas login,
log audit, dan log permintaan.

Kalau nanti memakai Cloudflare, Caddy harus diberi tahu rentang IP Cloudflare (`trusted_proxies` dan
`client_ip_headers CF-Connecting-IP`) supaya `{client_ip}` berisi alamat pengunjung, bukan alamat Cloudflare.

## Memeriksa

```sh
docker compose -f docker-compose.prod.yml logs -f api
docker compose -f docker-compose.prod.yml ps
```

API menjawab cek hidup di `GET /health` dari dalam jaringan Docker (tidak dibuka ke luar).
