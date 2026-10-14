# Log Keputusan Teknis (Architecture Decision Record)

Sesuai aturan di `agent.md` Bagian 2, setiap keputusan teknis atau pemakaian nilai *default* dari `brief.md` dicatat di dokumen ini dengan format: tanggal, keputusan, dan alasan.

---

| Tanggal | Keputusan | Alasan / Rujukan |
|---|---|---|
| 2026-10-13 | Booking `MENUNGGU` menahan slot (BR-03) | Mencegah overbooking ganda saat pengajuan sedang direview oleh TU/Kalab (`brief.md` Bagian 10 #1). |
| 2026-10-13 | Durasi booking 30 menit s.d. 8 jam, jam operasional 07:00–21:00 WIB, maksimal H+30 via env | Standar operasional kampus yang fleksibel via konfigurasi runtime (`brief.md` BR-01 & Bagian 10 #2). |
| 2026-10-13 | Sumber jadwal kuliah dimasukkan manual oleh admin lewat UI | Menjaga fokus MVP; impor CSV ditunda pasca Phase 6 (`brief.md` Bagian 10 #3). |
| 2026-10-13 | Otentikasi menggunakan akun lokal dengan argon2id | Lapisan auth dipisah agar SSO/SIAKAD bisa diintegrasikan di kemudian hari tanpa migrasi akun rumit (`brief.md` Bagian 10 #4). |
| 2026-10-13 | Peran OB difokuskan untuk verifikasi fisik dan pembuatan laporan kerusakan | Tidak ada aksi kelola booking khusus untuk OB pada cakupan MVP (`brief.md` Bagian 10 #5). |
| 2026-10-13 | Peran approver disatukan dalam `TU_KALAB` untuk ruang kelas maupun lab | Simplifikasi hirarki approval di tingkat satu unit Rumpun TI (`brief.md` Bagian 10 #6). |
| 2026-10-13 | Notifikasi hanya in-app (polling / SSE opsional) | Email dan WhatsApp dinyatakan *out of scope* untuk MVP (`brief.md` Bagian 1 & Bagian 10 #7). |
| 2026-10-13 | Kolom `org_unit_id` diterapkan di skema namun scope dibatasi pada Rumpun TI | Menyiapkan skalabilitas multi-unit masa depan tanpa memperumit filter query saat ini (`brief.md` Bagian 10 #8). |
| 2026-10-13 | Backend menggunakan Go (chi, pgx/v5, sqlc, golang-migrate, log/slog) | Performa tinggi, kontrol transaksi eksplisit, konkurensi native, dan footprint memori kecil (`brief.md` Bagian 2). |
| 2026-10-13 | Frontend menggunakan template `satnaing/shadcn-admin` (Vite, React 19, TypeScript strict, TanStack, Tailwind v4) | Menyediakan fondasi dashboard modern yang siap pakai setelah dibersihkan dari fitur demo (`brief.md` Bagian 2 & 7.1). |
| 2026-10-14 | Access token JWT hanya memuat `sub`; peran dan status aktif dibaca dari DB di setiap request | Perubahan peran dan penonaktifan akun berlaku seketika, bukan setelah token 15 menit kedaluwarsa. Biayanya satu query ringan per request (Phase 1). |
| 2026-10-14 | Parameter argon2id default 19 MiB / 2 iterasi / 1 lane, bisa diubah lewat `ARGON2_*`; `Verify` membaca parameter dari hash tersimpan | Minimum rekomendasi OWASP. Mengubah konfigurasi tidak mengunci akun lama. |
| 2026-10-14 | Rate limit login in-memory (5 percobaan/15 menit, kunci `identifier|IP`), percobaan dihitung saat masuk dan direset saat login sukses | Dihitung di awal supaya tebakan paralel tidak lolos. State hilang saat restart dan tidak dibagi antar-instance; cukup untuk deployment satu instance MVP. |
| 2026-10-14 | IP klien diambil dari `RemoteAddr`, bukan `X-Forwarded-For` | Header itu bisa dipalsukan untuk menghindari limiter. **Phase 6:** di belakang Caddy semua request tampak dari IP proxy, sehingga perlu pengaturan trusted proxy. |
| 2026-10-14 | Refresh token dipakai ulang (reuse) dan token kedaluwarsa/user nonaktif mencabut seluruh `family_id` | Sesuai `brief.md` 6.1. Konsekuensi: dua tab yang me-refresh dengan cookie yang sama pada saat bersamaan akan saling mengeluarkan; frontend harus men-serialisasi refresh (Phase 5). |
| 2026-10-14 | Logout mencabut family dari cookie refresh yang dikirim, hanya bila milik pemanggil | Mencegah satu pengguna mencabut sesi pengguna lain dengan cookie curian. Tanpa cookie, logout hanya menghapus cookie di klien. |
| 2026-10-14 | Menonaktifkan pengguna atau mereset password mencabut semua refresh token pengguna itu | Penonaktifan harus mengakhiri sesi yang sedang berjalan, bukan hanya memblokir login baru. |
| 2026-10-14 | Kaprodi tidak boleh menonaktifkan atau menurunkan peran akunnya sendiri (422) | Mencegah sistem kehilangan satu-satunya admin. Tidak ada di `brief.md`; aturan tambahan yang tidak mengubah kontrak. |
| 2026-10-14 | Duplikat `identifier`/`email` dan `org_unit_id` tak dikenal dikembalikan sebagai `422 VALIDATION_ERROR` dengan `details.field` | `brief.md` 6.1 tidak punya kode 409 untuk duplikat akun; 422 per-field paling sesuai dengan tabel error yang ada. |
| 2026-10-14 | Password minimal 8, maksimal 128 karakter; `identifier` 3-64 karakter `[A-Za-z0-9._-]` | Brief tidak menetapkan kebijakan password. Batas atas mencegah hashing input raksasa. |
| 2026-10-14 | Seed dev memakai satu password publik (`classlab-dev-2026`); `cmd/seed` menolak jalan bila `APP_ENV=production`, dan server produksi menolak start bila akun seed masih aktif | Memenuhi larangan seed aktif di produksi (`agent.md` bagian 11). Pembuatan Kaprodi pertama di produksi belum ada jalurnya; harus ditangani di Phase 6. |
| 2026-10-14 | `COOKIE_SECURE=true` wajib saat `APP_ENV=production`; `7d` diterima sebagai durasi di env | `.env.example` mendokumentasikan `7d` yang tidak dikenali `time.ParseDuration`, sehingga nilainya diam-diam jatuh ke default. |
| 2026-10-14 | `sqlc` dijalankan lewat image Docker `sqlc/sqlc:1.30.0` dan kode hasil generate di-commit | Instalasi lokal sqlc gagal di Windows (butuh cgo). Image Docker membuat versi sama di semua mesin. |
| 2026-10-14 | Tambah `testcontainers-go` (+ modul `postgres`) dan `kin-openapi` | Keduanya sudah tercantum di `brief.md` bagian 2 (Test). |
| 2026-10-14 | `api/openapi.yaml` dilengkapi untuk Phase 1 (respons 400/401/403/404/422/429, skema `ErrorBody`, `required`) dan semua respons diberi `description` | Perubahan aditif, tidak mengubah path/field/kode yang sudah disepakati. Spec Phase 0 ternyata gagal validasi OpenAPI (respons tanpa `description`); sekarang dicek oleh contract test. |
| 2026-10-14 | `.gitignore` memakai pola yang di-anchor ke root (`/data/`, `/uploads/`, `/api/uploads/`, `/tmp/`, `/scratch/`) | Pola `data/` tanpa anchor ikut membuang `web/src/features/*/data/` dan `web/src/components/layout/data/` dari commit Phase 0, sehingga `pnpm build` gagal di checkout bersih. |
| 2026-10-14 | Menambah `.gitattributes` (`eol=lf`; migrasi `-text`; `.ps1` CRLF) dan menormalkan working tree ke LF | `core.autocrlf=true` menulis CRLF di Windows sehingga `prettier --check` (endOfLine lf) gagal di 205 file. Migrasi sengaja tidak diubah byte-nya. |
| 2026-10-14 | File `data/` template dipulihkan dari `satnaing/shadcn-admin`; `sidebar-data.ts` dibuat ulang minimal (rute yang ada, judul sesuai `search-provider.test.tsx`) | Sidebar ClassLab per peran dikerjakan di Phase 5. |
