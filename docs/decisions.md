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
