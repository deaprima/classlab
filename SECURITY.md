# Security Policy

## Versi yang didukung

Security patch diberikan untuk versi terbaru pada branch `main`. Jika Anda menjalankan ClassLab sendiri, perbarui ke `main` terbaru sebelum mengirim laporan.

## Melaporkan vulnerability

**Jangan membuka issue atau pull request publik untuk melaporkan vulnerability.**

Gunakan fitur private vulnerability reporting dari GitHub:

1. Buka tab **Security** pada repositori ini.
2. Pilih **Report a vulnerability**.
3. Lengkapi laporan dengan sejelas mungkin.

Laporan sebaiknya memuat:

- jenis vulnerability dan dampaknya (misalnya akses ke data pengguna lain, bypass pembatasan peran, atau bypass validasi bentrok),
- langkah untuk mereproduksi, atau proof of concept yang minimal,
- versi atau commit yang terdampak,
- konfigurasi yang relevan (jangan menyertakan secret yang sebenarnya).

Jangan menyertakan password, token, atau data pengguna yang sebenarnya.

## Proses penanganan

- Kami berupaya mengonfirmasi laporan dalam waktu **7 hari**.
- Kami akan menyampaikan perkembangan penanganan dan memberi tahu Anda ketika perbaikan dirilis.
- Atas permintaan Anda, nama Anda akan dicantumkan sebagai pelapor setelah vulnerability diperbaiki.
- Mohon berikan waktu yang wajar bagi kami untuk memperbaiki sebelum informasi diungkapkan ke publik (*coordinated disclosure*).

## Cakupan

**Dalam cakupan:** seluruh kode pada repositori ini (`api/`, `web/`, `deploy/`), termasuk autentikasi, otorisasi per peran, file upload, impor CSV, QR check-in, dan konfigurasi deployment.

**Di luar cakupan:**

- vulnerability pada dependency pihak ketiga yang tidak dipanggil oleh kode kami (laporkan ke proyek asalnya),
- serangan yang membutuhkan akses fisik ke server,
- serangan DoS volumetrik.

## Panduan bagi self-hoster

- Ganti seluruh nilai default pada `.env` dan `deploy/.env`. Gunakan `JWT_SECRET` acak minimal 32 karakter dan `POSTGRES_PASSWORD` yang kuat.
- Jangan menjalankan `cmd/seed` di production. Server menolak start pada mode production jika masih terdapat akun seed yang aktif.
- Pastikan `APP_ENV=production` dan `DEV_CLOCK_START` kosong di server production.
- Jangan mengekspos port Postgres ke internet. Compose production sudah tidak membukanya.
- Ikuti [deploy/RUNBOOK.md](deploy/RUNBOOK.md) untuk rotasi secret, backup, dan restore.
