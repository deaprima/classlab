# Checklist rilis production

Dipakai sebelum go-live pertama dan sebelum setiap rilis. Pemasangan ada di [README.md](README.md), penanganan gangguan di [RUNBOOK.md](RUNBOOK.md).

## A. Sebelum go-live pertama (sekali saja)

**Server dan domain**
- [ ] VPS Linux 64-bit, minimal 1 vCPU / 2 GB RAM / 25 GB SSD (1 GB hanya dengan swap). Docker dan Compose v2 terpasang.
- [ ] Domain menunjuk ke IP server; port **80 dan 443** terbuka, port lain tertutup (firewall).
- [ ] Login SSH memakai key, password SSH dimatikan; pembaruan keamanan OS otomatis aktif.

**Konfigurasi** (`deploy/.env`, tidak masuk git)
- [ ] `SITE_ADDRESS` berisi domain, `POSTGRES_PASSWORD` dan `JWT_SECRET` dibuat dengan `openssl rand -hex 32`.
- [ ] `APP_ENV=production`, `DEV_CLOCK_START` kosong.
- [ ] Salinan `.env` disimpan di password manager (backup **tidak** memuatnya).
- [ ] Akun seed tidak pernah dijalankan; admin dibuat dengan `createadmin`, password sementara sudah diganti.

**Cadangan**
- [ ] `backup.sh` dijadwalkan lewat cron (harian).
- [ ] Salinan keluar server aktif: rclone ke Google Drive dengan remote `crypt` (README, "Contoh: Google Drive"). Password crypt tersimpan di password manager, dan satu pemulihan dari Drive pernah dicoba.
- [ ] Pembersihan berkas lama di Drive dijadwalkan (cron mingguan).
- [ ] Pemantau backup aktif (`BACKUP_PING_URL`), dan sudah dicoba sekali dengan sengaja gagal.
- [ ] `./restore-test.sh` berhasil; satu kali pemulihan penuh pernah dicoba di server cadangan.

**Pemantauan**
- [ ] Pemantau uptime (UptimeRobot, Better Stack, dll.) pada `https://<domain>/api/v1/health`, dengan notifikasi ke ponsel.
- [ ] Peringatan disk hampir penuh (unggahan dan volume Docker tumbuh).

**Uji di perangkat nyata**
- [ ] iOS/Safari dan Android/Chrome tercatat di [docs/camera-manual-test.md](../docs/camera-manual-test.md) dan [docs/responsive-checklist.md](../docs/responsive-checklist.md).
- [ ] Satu alur penuh di production: ajukan, approve, check-in (scanner **dan** kamera ponsel), check-out dengan foto, riwayat.

## B. Setiap rilis

1. Pastikan CI `develop` hijau (`go test`, `govulncheck`, `pnpm audit --prod`, smoke e2e).
2. Perbarui [CHANGELOG.md](../CHANGELOG.md): pindahkan isi "Belum dirilis" ke versi baru dengan tanggal.
3. Merge `develop` ke `main` lewat PR, lalu beri tag:
   ```sh
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```
4. Di server:
   ```sh
   cd /opt/classlab && git fetch --tags && git checkout v0.1.0
   cd deploy && VERSION=v0.1.0 docker compose -f docker-compose.prod.yml up -d --build
   ```
5. Periksa: `dc ps` (semua `healthy`, `migrate` `exited (0)`), `dc logs --since 5m api` tanpa `"level":"ERROR"`, dan `GET /api/v1/health` menampilkan versi yang baru.
6. Cadangan manual **sebelum** rilis yang menyertakan migrasi: `./backup.sh`.

**Rollback:** kembalikan tag sebelumnya dan jalankan perintah langkah 4. Migrasi punya `down`; urutannya ada di RUNBOOK.

## C. Hal yang sengaja belum ada (batas MVP)

- Notifikasi hanya di dalam aplikasi (tanpa email/WhatsApp); lupa password ditangani admin.
- Satu unit organisasi (rumpun TI); belum multi-fakultas.
- Satu server tanpa failover; pemulihan bergantung pada backup di atas.
- Belum ada uji beban; pantau `docker stats` pada pemakaian nyata pertama lalu sesuaikan ukuran server.
