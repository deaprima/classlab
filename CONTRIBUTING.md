# Contributing Guide

Terima kasih atas minat Anda untuk berkontribusi ke ClassLab. Dokumen ini menjelaskan cara menyiapkan environment, aturan yang kami pegang, dan hal yang diharapkan dari sebuah pull request.

Dengan berpartisipasi, Anda setuju untuk mematuhi [Code of Conduct](CODE_OF_CONDUCT.md). Untuk vulnerability, jangan membuka issue; ikuti panduan di [SECURITY.md](SECURITY.md).

## Memulai

1. Fork repositori ini, lalu clone fork Anda.
2. Ikuti bagian [Quick start](README.md#quick-start) pada README hingga API dan web berjalan di lokal.
3. Unduh browser untuk frontend test (cukup sekali):
   ```sh
   cd web && pnpm exec playwright install chromium chromium-headless-shell
   ```
4. Buat branch dari `main`.

## Sebelum mulai mengerjakan

- **Perubahan besar:** buka issue terlebih dahulu agar arah pengerjaan disepakati sebelum kode ditulis.
- **Bug fix atau perbaikan kecil:** langsung kirim pull request.
- **Fitur baru:** pastikan fitur tersebut tidak termasuk di luar lingkup proyek, yaitu aplikasi mobile native, CCTV, SSO, notifikasi email/WhatsApp, pembayaran, dan inventaris alat.

## Alur kerja

- Jangan commit langsung ke `main`. Format nama branch: `feat/<modul>-<ringkas>`, `fix/<ringkas>`, `chore/<ringkas>`, `docs/<ringkas>`.
- Satu pull request memiliki satu tujuan. Pisahkan refactor, cleanup, dan fitur ke commit atau PR yang berbeda.
- Pesan commit mengikuti [Conventional Commits](https://www.conventionalcommits.org/) dalam bahasa Inggris, contoh: `feat(bookings): reject requires a reason`.
- Jalankan perintah berikut sebelum mengirim, dan pastikan semuanya lulus:
  ```sh
  make lint
  make test
  cd web && pnpm run build
  ```
  Jika perubahan menyentuh alur pengajuan, check-in, atau check-out, jalankan juga `cd web && pnpm run smoke:e2e`.

## Standar kode

**Umum**

- Identifier, komentar kode, nama field API, dan pesan commit: **bahasa Inggris**.
- Teks antarmuka, pesan error untuk pengguna, dan dokumentasi: **bahasa Indonesia**. Kode error (`error.code`) tetap berupa konstanta bahasa Inggris.
- Komentar menjelaskan *mengapa*, bukan *apa*.
- Jangan membuat abstraksi baru hanya karena ada dua tempat pemakaian.

**Backend (Go)**

- Utamakan standard library. Jangan menambahkan web framework, ORM, atau DI container.
- `context.Context` menjadi argumen pertama pada fungsi yang melakukan I/O.
- Bungkus error dengan `fmt.Errorf("...: %w", err)`. Pemetaan error ke HTTP status terpusat di `internal/httpx`, dan pesan internal maupun SQL tidak boleh bocor ke klien.
- Simpan dan hitung waktu dalam UTC. Jangan memanggil `time.Now()` langsung di service; gunakan clock yang dapat di-inject agar mudah diuji.
- Jangan menulis password, token, hash, atau isi cookie ke log.

**Database**

- Skema hanya berubah melalui **migrasi baru**. Jangan mengedit migrasi yang sudah di-merge. Setiap `up` harus memiliki `down`, dan keduanya diuji maju-mundur.
- Query ditulis di `api/db/queries` dan di-generate dengan `sqlc` (`make gen`). Dilarang menyusun SQL dari string input pengguna.
- Operasi yang menyentuh beberapa tabel dijalankan dalam satu transaksi (`WithTx`).
- Exclusion constraint untuk mencegah bentrok harus dipertahankan. Pengecekan di level kode saja tidak cukup.
- Tabel riwayat (`booking_events`, `report_events`, `audit_logs`) bersifat append-only: tidak boleh di-`UPDATE` atau `DELETE`.

**Kontrak API**

- [api/openapi.yaml](api/openapi.yaml) adalah *single source of truth*. Jalankan `make gen` setiap kali kontrak berubah dan sertakan hasilnya. CI akan gagal jika tipe frontend tidak sinkron.
- Mengubah path, nama field, kode error, atau status code yang sudah digunakan akan merusak klien lain (*breaking change*). Jelaskan alasannya pada deskripsi PR.

**Frontend (TypeScript)**

- Gunakan mode `strict`. Jangan memakai `any` atau `@ts-ignore` tanpa komentar yang menjelaskan alasannya.
- Tipe response API di-generate; jangan menulis ulang secara manual. Jangan mengedit `routeTree.gen.ts` dan `src/types/api.ts` secara langsung.
- Server state dikelola melalui TanStack Query. Jangan menyalinnya ke store lokal.
- Access token disimpan di memori, bukan di `localStorage`.
- Gunakan modul waktu bersama dengan zona `Asia/Jakarta`; jangan memformat tanggal sendiri di dalam komponen.
- Antarmuka harus nyaman di perangkat mobile: touch target minimal 44 px dan tanpa horizontal scroll.

## Testing

Test bukan pelengkap. Bagian berikut **wajib** memiliki test apabila diubah:

- Bentrok dan konkurensi: 20 request paralel pada slot yang sama menghasilkan tepat satu yang berhasil, dan batas `[mulai, selesai)` dihormati.
- Transisi status booking dan laporan, baik yang valid maupun yang tidak.
- Matriks hak akses: setiap endpoint untuk setiap peran.
- Autentikasi: kredensial salah, token kedaluwarsa, serta rotasi dan penggunaan ulang refresh token.
- Notifikasi dibuat secara atomik bersama perubahan statusnya.

Aturan umum:

- Backend test menggunakan PostgreSQL asli (testcontainers), bukan database tiruan, untuk semua hal yang menyangkut constraint dan transaksi. Integration test berada di [api/test/integration/](api/test/integration/).
- Test harus deterministik: tanpa `sleep` dan tidak bergantung pada urutan eksekusi.
- Menemukan bug? Tulis test yang gagal terlebih dahulu, lalu perbaiki.
- Jangan menghapus atau melemahkan test hanya agar build lulus. Jika sebuah test memang keliru, jelaskan alasannya pada PR.

## Pull request

Deskripsi PR sebaiknya memuat:

- apa yang berubah dan alasannya,
- cara menjalankan atau menguji perubahan,
- bagian yang belum dikerjakan,
- keputusan desain yang diambil, terutama jika mengubah kontrak API atau skema.

Untuk perubahan tampilan, sertakan screenshot pada lebar 360 px dan 1280 px.

Checklist yang akan kami periksa:

- [ ] `make lint`, `make test`, dan `pnpm run build` lulus
- [ ] Test baru untuk perilaku baru, termasuk skenario gagal dan hak akses
- [ ] `openapi.yaml` dan tipe hasil generate diperbarui jika kontrak berubah
- [ ] Migrasi baru (jika ada) memiliki `up` dan `down`
- [ ] Tidak ada secret, data pribadi, atau log sensitif yang ikut ter-commit

## Melaporkan bug

Gunakan template issue **Laporan bug** dan sertakan langkah reproduksi, expected result, actual result, serta versi browser dan sistem operasi. Jangan menempelkan token, password, atau data pengguna yang sebenarnya.
