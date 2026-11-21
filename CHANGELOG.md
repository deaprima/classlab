# Changelog

Format mengikuti [Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/), versi mengikuti [Semantic Versioning](https://semver.org/lang/id/).

## [Belum dirilis]

### Ditambahkan
- Pemindai QR di dalam aplikasi untuk check-in (tombol Check-in di beranda). Memindai dengan kamera ponsel tetap didukung.
- `/health` (internal, untuk healthcheck kontainer) menampilkan versi rilis yang diberikan saat build (`VERSION`).
- `GET /api/v1/health`: alamat publik untuk pemantau uptime; hanya `status`, 503 bila database tidak terjangkau.
- Dependabot mingguan untuk Go, npm, GitHub Actions, dan image Docker.
- Opsi `backup.sh`: salinan keluar server (`BACKUP_REMOTE`, rclone) dan ping pemantau (`BACKUP_PING_URL`).
- [deploy/CHECKLIST.md](deploy/CHECKLIST.md): checklist go-live dan rilis.

### Diperbaiki
- Skrip `backup.sh`, `restore.sh`, dan `restore-test.sh` kini executable; sebelumnya `./backup.sh` dan cron gagal dengan `permission denied` setelah `git clone`.

### Diubah
- Test integrasi backend dipindah dari `api/internal/app` ke `api/test/integration`.
- README, SECURITY, CONTRIBUTING, dan CODE_OF_CONDUCT ditulis ulang dengan bahasa yang lebih profesional.

## [0.1.0] - belum ditandai

Kandidat rilis pertama (MVP rumpun TI). Isi:
- Login dan hak akses untuk enam peran (Mahasiswa, Organisasi, TU, Kalab, OB, Admin).
- Pengelolaan ruang, jadwal akademik, dan ketersediaan real-time.
- Pengajuan booking dengan pencegahan bentrok berlapis, multi-tanggal, dan ubah/batal.
- Persetujuan per tipe ruang (otomatis dan manual), kedaluwarsa otomatis 24 jam.
- Check-in QR dan check-out dengan foto kondisi ruang; no-show dilepas otomatis.
- Kuota booking, poin pelanggaran, dan blokir sementara.
- Laporan kerusakan dengan papan status, notifikasi in-app, dan pengingat.
- Impor CSV jadwal akademik dan akun mahasiswa (pratinjau, terapkan, rollback).
- Audit log append-only.
- Deploy Docker Compose dengan Caddy (HTTPS otomatis), backup dan restore.
