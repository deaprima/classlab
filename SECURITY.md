# Kebijakan Keamanan

## Versi yang didukung

Perbaikan keamanan diberikan untuk versi terbaru di cabang `main`. Bila Anda menjalankan ClassLab sendiri, perbarui ke `main` terbaru sebelum melaporkan.

## Melaporkan celah keamanan

**Jangan membuka issue atau pull request publik untuk celah keamanan.**

Gunakan pelaporan privat GitHub:

1. Buka tab **Security** pada repositori ini.
2. Pilih **Report a vulnerability**.
3. Isi laporan dengan sejelas mungkin.

Sertakan:

- jenis celah dan dampaknya (misalnya akses data orang lain, melewati batas peran, atau melewati validasi bentrok),
- langkah untuk mengulanginya, atau bukti konsep yang minimal,
- versi atau commit yang terdampak,
- konfigurasi yang relevan (jangan sertakan rahasia asli).

Jangan menyertakan password, token, atau data pengguna yang sebenarnya.

## Yang bisa Anda harapkan

- Kami berusaha membalas dan mengonfirmasi laporan dalam **7 hari**.
- Kami akan memberi kabar perkembangan penanganan, dan memberi tahu Anda saat perbaikan dirilis.
- Bila Anda mau, kami mencantumkan nama Anda sebagai pelapor setelah celah diperbaiki.
- Mohon beri kami waktu yang wajar untuk memperbaiki sebelum Anda mengungkapkannya ke publik.

## Cakupan

Termasuk cakupan: kode di repositori ini (`api/`, `web/`, `deploy/`), termasuk autentikasi, otorisasi per peran, unggahan berkas, impor CSV, QR check-in, dan konfigurasi deploy.

Di luar cakupan: celah pada dependensi pihak ketiga yang tidak dipanggil oleh kode kami (laporkan ke proyek asalnya), serangan yang butuh akses fisik ke server, dan serangan DoS volumetrik.

## Untuk yang menjalankan ClassLab sendiri

- Ganti semua nilai bawaan di `.env` dan `deploy/.env`. Pakai `JWT_SECRET` acak minimal 32 karakter dan `POSTGRES_PASSWORD` yang kuat.
- Jangan pernah menjalankan `cmd/seed` di produksi. Server menolak start di mode produksi bila masih ada akun seed aktif.
- Pastikan `APP_ENV=production` dan `DEV_CLOCK_START` kosong di server produksi.
- Jangan membuka port Postgres ke internet. Compose produksi sudah tidak membukanya.
- Ikuti [deploy/RUNBOOK.md](deploy/RUNBOOK.md) untuk rotasi rahasia, cadangan, dan pemulihan.
