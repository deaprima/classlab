# Panduan Kontribusi

Terima kasih sudah tertarik berkontribusi ke ClassLab. Dokumen ini menjelaskan cara menyiapkan lingkungan, aturan yang kami pegang, dan apa yang diharapkan dari sebuah pull request.

Dengan ikut serta, Anda setuju mengikuti [Kode Etik](CODE_OF_CONDUCT.md). Untuk celah keamanan, jangan buka issue; lihat [SECURITY.md](SECURITY.md).

## Memulai

1. Fork repo ini, lalu clone fork Anda.
2. Ikuti bagian [Mulai cepat](README.md#mulai-cepat) di README sampai API dan web berjalan di lokal.
3. Unduh browser untuk tes frontend, sekali saja:
   ```sh
   cd web && pnpm exec playwright install chromium chromium-headless-shell
   ```
4. Buat cabang dari `main`.

## Sebelum mulai mengerjakan

- **Perubahan besar:** buka issue dulu supaya arahnya disepakati sebelum Anda menulis kode.
- **Bug atau perbaikan kecil:** langsung kirim pull request.
- **Fitur baru:** cek dulu bahwa fiturnya tidak termasuk di luar lingkup proyek: aplikasi mobile native, CCTV, SSO, notifikasi email/WhatsApp, pembayaran, dan inventaris alat.

## Alur kerja

- Jangan commit langsung ke `main`. Nama cabang: `feat/<modul>-<ringkas>`, `fix/<ringkas>`, `chore/<ringkas>`, `docs/<ringkas>`.
- Satu pull request punya satu tujuan. Pisahkan refactor, pembersihan, dan fitur ke commit atau PR berbeda.
- Pesan commit memakai [Conventional Commits](https://www.conventionalcommits.org/) dalam bahasa Inggris, misalnya `feat(bookings): reject requires a reason`.
- Jalankan ini sebelum mengirim, dan pastikan semuanya hijau:
  ```sh
  make lint
  make test
  cd web && pnpm run build
  ```
  Bila perubahan menyentuh alur pengajuan, check-in, atau check-out, jalankan juga `cd web && pnpm run smoke:e2e`.

## Standar kode

**Umum**

- Identifier, komentar kode, nama field API, dan pesan commit: **bahasa Inggris**.
- Teks antarmuka, pesan error ke pengguna, dan dokumentasi: **bahasa Indonesia**. Kode error (`error.code`) tetap konstanta bahasa Inggris.
- Komentar menjelaskan *kenapa*, bukan *apa*.
- Jangan membuat abstraksi baru hanya karena ada dua pemakaian.

**Backend (Go)**

- Utamakan library standar. Jangan menambah framework web, ORM, atau DI container.
- `context.Context` menjadi argumen pertama fungsi yang melakukan I/O.
- Bungkus error dengan `fmt.Errorf("...: %w", err)`. Pemetaan error ke status HTTP ada di satu tempat (`internal/httpx`), dan pesan internal atau SQL tidak boleh bocor ke klien.
- Tulis dan hitung waktu dalam UTC, dan jangan memanggil `time.Now()` langsung di service; pakai clock yang bisa disuntik supaya bisa dites.
- Jangan mencatat password, token, hash, atau isi cookie ke log.

**Database**

- Skema hanya berubah lewat **migrasi baru**. Jangan mengedit migrasi yang sudah ter-merge. Setiap `up` punya `down`, dan keduanya dicoba maju-mundur.
- Query ditulis di `api/db/queries` dan di-generate dengan `sqlc` (`make gen`). Dilarang menyusun SQL dari string input pengguna.
- Operasi yang menyentuh beberapa tabel berjalan dalam satu transaksi (`WithTx`).
- Constraint exclusion untuk bentrok tetap dipertahankan. Cek di kode saja tidak cukup.
- Tabel riwayat (`booking_events`, `report_events`, `audit_logs`) hanya boleh ditambah, tidak boleh di-UPDATE atau DELETE.

**Kontrak API**

- [api/openapi.yaml](api/openapi.yaml) adalah sumber kebenaran. Jalankan `make gen` setiap kali kontrak berubah dan sertakan hasilnya. CI gagal bila tipe frontend tidak sinkron.
- Mengubah path, nama field, kode error, atau status code yang sudah dipakai merusak klien lain. Jelaskan alasannya di deskripsi PR.

**Frontend (TypeScript)**

- Mode `strict`. Jangan memakai `any` atau `@ts-ignore` tanpa komentar alasan.
- Tipe respons API di-generate; jangan menulis ulang secara manual. Jangan edit `routeTree.gen.ts` dan `src/types/api.ts` dengan tangan.
- Data server lewat TanStack Query. Jangan menyalinnya ke store lokal.
- Access token disimpan di memori, bukan di `localStorage`.
- Pakai modul waktu bersama dengan zona `Asia/Jakarta`; jangan memformat tanggal sendiri di komponen.
- Antarmuka harus nyaman di ponsel: area sentuh minimal 44 px dan tidak ada scroll horizontal.

## Pengujian

Tes bukan pelengkap. Bagian berikut wajib punya tes bila Anda mengubahnya:

- Bentrok dan konkurensi: 20 permintaan paralel pada slot yang sama menghasilkan tepat satu yang berhasil, dan batas `[mulai, selesai)` dihormati.
- Transisi status booking dan laporan, baik yang sah maupun yang tidak.
- Matriks hak akses: setiap endpoint untuk setiap peran.
- Autentikasi: kredensial salah, token kedaluwarsa, rotasi dan penggunaan ulang refresh token.
- Notifikasi dibuat atomik bersama perubahan statusnya.

Aturan umum:

- Tes backend memakai PostgreSQL asli (testcontainers), bukan database palsu, untuk apa pun yang menyangkut constraint dan transaksi.
- Tes harus deterministik: tanpa `sleep` dan tanpa bergantung pada urutan.
- Menemukan bug? Tulis tes yang gagal dulu, lalu perbaiki.
- Jangan menghapus atau melemahkan tes agar build hijau. Bila sebuah tes memang salah, jelaskan alasannya di PR.

## Pull request

Deskripsi PR sebaiknya memuat:

- apa yang berubah dan kenapa,
- cara menjalankan atau mengujinya,
- bagian yang belum dikerjakan,
- keputusan desain yang diambil, terutama bila mengubah kontrak API atau skema.

Untuk perubahan tampilan, sertakan tangkapan layar pada lebar 360 px dan 1280 px.

Checklist yang akan kami periksa:

- [ ] `make lint`, `make test`, dan `pnpm run build` hijau
- [ ] Tes baru untuk perilaku baru, termasuk kasus gagal dan hak akses
- [ ] `openapi.yaml` dan tipe hasil generate diperbarui bila kontrak berubah
- [ ] Migrasi baru (bila ada) punya `up` dan `down`
- [ ] Tidak ada rahasia, data pribadi, atau log sensitif yang ikut masuk

## Melaporkan bug

Gunakan template issue **Laporan bug** dan sertakan langkah untuk mengulanginya, hasil yang diharapkan, hasil yang terjadi, serta versi browser dan sistem operasi. Jangan menempelkan token, password, atau data pengguna asli.
