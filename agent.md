# agent.md: Aturan Kerja Agent untuk ClassLab.io

Dokumen ini mengatur **BAGAIMANA** kamu bekerja. **APA** yang dibangun ada di `brief.md`. Jika dua dokumen bertentangan, `brief.md` menang untuk perilaku produk dan `agent.md` menang untuk cara kerja. Jika kamu menemukan kontradiksi, laporkan, jangan menebak diam-diam.

## 1. Prinsip utama

1. **Benar dulu, baru cepat.** Aturan bentrok jadwal, RBAC, dan integritas riwayat tidak boleh dikorbankan demi kecepatan.
2. **Jangan perluas scope.** Daftar *out of scope* di `brief.md` bagian 1 bersifat mengikat.
3. **Jujur soal status.** Katakan apa yang sudah dijalankan dan dites, apa yang belum, dan apa yang kamu tebak. Jangan menyatakan "selesai" atas hal yang tidak kamu jalankan.
4. **Perubahan kecil dan bisa direview.** Satu tujuan per commit, satu phase per PR.
5. **Kontrak adalah janji.** `api/openapi.yaml` dipakai frontend. Mengubahnya diam-diam merusak pekerjaan orang lain.

## 2. Kapan harus bertanya (berhenti dan tanya)

Berhenti dan minta konfirmasi sebelum:

- mengubah kontrak API yang sudah disepakati (path, nama field, kode error, status code)
- mengubah migrasi yang sudah ter-merge, atau skema dengan cara yang merusak data
- menambah dependency yang tidak ada di `brief.md` bagian 2
- membangun sesuatu yang termasuk *out of scope*
- menghapus file/fitur di luar daftar pembersihan template
- menjalankan perintah destruktif (drop database, `rm -rf` di luar `node_modules`/build, force push)

Untuk hal lain, **ambil default di `brief.md` bagian 10, kerjakan, dan catat** di `docs/decisions.md` (tanggal, keputusan, alasan satu baris). Jangan menghentikan pekerjaan untuk pertanyaan yang sudah punya default.

## 3. Struktur repo

```
classlab/
├── api/                     # backend Go
│   ├── cmd/server/          # main.go (wiring saja)
│   ├── internal/
│   │   ├── config/          # env -> struct, validasi saat start
│   │   ├── httpx/           # router, middleware, error mapper, respons
│   │   ├── auth/ users/ rooms/ schedules/ availability/
│   │   ├── bookings/ notifications/ reports/
│   │   └── platform/        # db pool, tx helper, clock, logger
│   ├── db/
│   │   ├── migrations/      # golang-migrate (*.up.sql / *.down.sql)
│   │   └── queries/         # sqlc (*.sql)
│   ├── openapi.yaml         # (atau ../api/openapi.yaml; satu lokasi saja)
│   └── Dockerfile
├── web/                     # frontend (shadcn-admin yang dibersihkan)
├── deploy/                  # docker-compose.prod.yml, Caddyfile, backup.sh
├── docs/                    # decisions.md, runbook, ERD
├── docker-compose.yml       # dev
├── Makefile
├── brief.md
└── agent.md
```

Tiap modul backend punya tiga lapisan: `handler.go` (HTTP, decode/encode, tanpa logika bisnis), `service.go` (aturan bisnis, transaksi, RBAC per aksi), `repo.go` atau kode `sqlc` (akses data). Handler tidak boleh memanggil database langsung.

Jika struktur sedikit berbeda saat Phase 0, tidak masalah, asalkan konsisten dan didokumentasikan di `README.md`.

## 4. Perintah standar

Buat di Phase 0 dan **pertahankan agar selalu berfungsi**:

| Perintah | Fungsi |
|---|---|
| `make up` / `make down` | Jalankan/hentikan Postgres dan layanan dev |
| `make migrate` / `make migrate-down` | Terapkan/mundurkan migrasi |
| `make gen` | `sqlc generate` + generate tipe FE dari OpenAPI |
| `make lint` | `go vet`, `staticcheck` (atau `golangci-lint`), `eslint`, `prettier --check` |
| `make test` | Tes backend (dengan Postgres via testcontainers) + tes FE |
| `make seed` | Isi data dev |

**Sebelum menyatakan pekerjaan selesai:** `make lint test` harus hijau, dan `pnpm build` di `web/` harus lolos (`tsc -b` ikut berjalan).

## 5. Aturan backend (Go)

**Umum**

- Pakai library standar dulu. Jangan menambah framework web, ORM, atau DI container.
- `context.Context` adalah argumen pertama untuk fungsi yang melakukan I/O. Teruskan, jangan simpan di struct.
- Tidak ada variabel global mutable. Dependency dikirim lewat konstruktor.
- Error: bungkus dengan `fmt.Errorf("...: %w", err)`. Definisikan error domain (`ErrConflict`, `ErrInvalidState`, `ErrForbidden`, ...) di service, lalu **satu** pemetaan terpusat di `httpx` ke `error.code` dan HTTP status sesuai `brief.md` 6.1. Jangan membocorkan pesan error internal/SQL ke klien.
- Log dengan `log/slog` berformat JSON, sertakan `request_id`. **Jangan** log password, token, hash, atau isi cookie.
- Konfigurasi hanya dari environment variable, divalidasi saat start (gagal cepat). Sediakan `.env.example`; jangan commit `.env`.
- Waktu: simpan dan hitung dalam UTC, keluarkan dengan offset `+07:00`. Jangan memakai `time.Now()` langsung di service; pakai clock yang bisa disuntik agar bisa dites.
- ID memakai UUID. Jangan membuat ID dari input pengguna.

**Database**

- Skema hanya berubah lewat migrasi baru. **Jangan mengedit migrasi yang sudah ter-merge.** Setiap `up` punya `down`.
- Query ditulis di `db/queries` dan di-generate `sqlc`. Dilarang menyusun SQL dengan konkatenasi string dari input.
- Operasi multi-tabel selalu dalam satu transaksi lewat helper `WithTx`. Pembuatan booking/approval/jadwal mengikuti urutan lock di `brief.md` 4.4 (`FOR UPDATE` pada baris ruang, cek overlap, tulis).
- Jangan hanya mengandalkan cek aplikasi untuk bentrok; exclusion constraint harus tetap ada dan pelanggarannya (`SQLSTATE 23P01`) dipetakan ke `BOOKING_CONFLICT`.
- Tabel riwayat (`booking_events`, `report_events`) append-only. Jangan menulis kode yang meng-UPDATE/DELETE-nya.
- Pagination wajib di semua endpoint list. Hindari N+1 (join atau query batch).

**Keamanan**

- Password: argon2id dengan parameter yang dikonfigurasi. Jangan pernah menyimpan atau mengembalikan password/hash.
- Access token JWT 15 menit (algoritma ditetapkan eksplisit, validasi `exp`, `iss`). Refresh token acak berentropi tinggi, **hanya hash-nya yang disimpan**, dikirim lewat cookie `httpOnly; Secure; SameSite=Strict; Path=/api/v1/auth`, dirotasi tiap pakai, dengan deteksi reuse per `family_id`.
- RBAC: middleware memeriksa peran minimum, **dan** service memeriksa kepemilikan (mis. mahasiswa hanya boleh melihat booking miliknya). Untuk data privat milik orang lain kembalikan `404`, bukan `403`.
- Respons login tidak membedakan "pengguna tidak ada" dan "password salah".
- Rate limit login sesuai `brief.md` 6.1.
- Upload: whitelist tipe (jpg/png/webp) lewat pengecekan magic bytes, batas 5 MB, nama file disimpan sebagai UUID (abaikan nama dari klien), simpan di luar webroot.
- Semua input divalidasi di server, tanpa mengandalkan FE.
- CORS hanya untuk origin yang dikonfigurasi. Tidak ada `*` bila memakai kredensial.

## 6. Aturan frontend (TypeScript)

- TypeScript **strict**. Dilarang `any` dan `@ts-ignore` tanpa komentar alasan. Gunakan `unknown` + penyempitan tipe.
- Tipe API **di-generate** dari `openapi.yaml` (`make gen`). Jangan menulis ulang tipe respons secara manual.
- Routing: TanStack Router file-based (jangan edit `routeTree.gen.ts` secara manual). Guard peran di `beforeLoad`.
- Data server lewat TanStack Query (query key terstruktur, invalidasi eksplisit setelah mutasi). Jangan menyalin data server ke Zustand; Zustand hanya untuk state UI/sesi.
- Form: react-hook-form + Zod. Skema Zod untuk booking mencerminkan BR-01 (durasi, kelipatan 30 menit, jam operasional, bukan masa lalu); server tetap sumber kebenaran.
- Token: access token di memori (store), **bukan** localStorage/cookie JS. Refresh lewat cookie httpOnly. Interceptor axios menangani `401` dengan satu kali refresh lalu mengulang request; bila gagal, arahkan ke `/sign-in`.
- Waktu: satu modul helper untuk parse/format dengan zona `Asia/Jakarta`. Jangan memformat tanggal secara ad hoc di komponen.
- Komponen di `components/ui` (shadcn) jangan diubah kecuali perlu; bungkus/compose di komponen sendiri.
- Teks UI dalam **bahasa Indonesia**, singkat dan jelas. Tampilkan error server yang berarti (mis. konflik jadwal beserta jam yang bentrok), bukan "Terjadi kesalahan".
- Aksesibilitas dasar: label pada input, fokus terlihat, navigasi keyboard pada dialog dan form.
- Pembersihan template (brief 7.1) dikerjakan di PR terpisah **sebelum** fitur baru, agar diff fitur tidak bercampur dengan penghapusan.

## 7. Aturan testing

Tes bukan pelengkap; beberapa tes bersifat **wajib** karena risikonya tinggi:

1. **Bentrok dan konkurensi.** 20 request paralel membooking slot yang sama → tepat 1 sukses, sisanya `409`. Termasuk batas `[start, end)`: 08:00–10:00 dan 10:00–12:00 tidak bentrok.
2. **State machine.** Setiap transisi sah dan setiap transisi tidak sah (`INVALID_STATE`) untuk booking dan laporan.
3. **Matriks RBAC.** Tabel-driven test: tiap endpoint × tiap peran → status yang diharapkan (200/403/404).
4. **Auth.** Kredensial salah, token kedaluwarsa, rotasi refresh, reuse refresh mencabut family, rate limit login.
5. **Riwayat.** Percobaan UPDATE/DELETE pada `booking_events` gagal; setiap perubahan status menambah satu event.
6. **Contract test.** Respons nyata divalidasi terhadap `openapi.yaml`.
7. **Notifikasi.** Dibuat atomik bersama perubahan status (rollback transaksi → tidak ada notifikasi).

Aturan tes:

- Gunakan Postgres asli (testcontainers). Jangan memalsukan database untuk tes yang menyangkut constraint/transaksi.
- Tes harus deterministik: tanpa `sleep`, tanpa ketergantungan urutan, clock disuntik.
- Bug yang ditemukan → tulis tes yang gagal dulu, lalu perbaiki.
- Jangan menghapus atau melemahkan tes agar build hijau. Jika tes salah, jelaskan alasannya di PR.
- FE: Vitest untuk util (helper waktu, skema Zod) dan alur kunci (form pengajuan, guard peran). Tidak perlu mengejar angka coverage; yang penting jalur kritis tercakup.

## 8. Git dan alur kerja

- Jangan commit langsung ke `main`. Cabang: `feat/<modul>-<ringkas>`, `fix/...`, `chore/...`.
- Pesan commit mengikuti **Conventional Commits** (`feat(bookings): reject requires reason`). Bahasa Inggris.
- Satu PR = satu phase atau satu bagian yang koheren. Deskripsi PR memuat: apa yang berubah, cara menjalankan/menguji, hal yang belum dikerjakan, dan keputusan yang diambil.
- Jangan mencampur refactor, pembersihan, dan fitur dalam satu commit.
- Jangan commit rahasia, `.env`, dump database, atau file build (`dist`, binary).
- Jangan `force push` ke cabang bersama.

## 9. Gaya kode dan bahasa

- Identifier, komentar kode, nama field API, dan pesan commit: **bahasa Inggris**.
- Teks UI, pesan error ke pengguna (`error.message`), dokumentasi `docs/` dan `README`: **bahasa Indonesia**. Kode error (`error.code`) tetap konstanta Inggris.
- Format otomatis: `gofmt`/`goimports` di Go, Prettier di FE (konfigurasi template dipakai). Jangan memperdebatkan gaya; jalankan formatter.
- Komentar menjelaskan *kenapa*, bukan *apa*. Fungsi yang melakukan hal tidak jelas (mis. urutan lock) wajib diberi komentar alasan.
- Hindari abstraksi prematur: dua pemakaian bukan alasan untuk membuat interface atau generic baru.

## 10. Cara melaporkan hasil

Setelah selesai suatu phase atau tugas, ringkas dengan format ini:

```
Selesai: <apa yang dikerjakan, 1-3 kalimat>
Diuji: <perintah yang dijalankan dan hasilnya>
Belum diuji / belum dikerjakan: <daftar jujur>
Keputusan diambil: <dengan default apa, dan dicatat di docs/decisions.md>
Perlu keputusan: <pertanyaan konkret, bila ada>
```

Jangan menulis klaim seperti "semua berfungsi" tanpa menyebut bukti. Jika sebuah perintah gagal dan kamu tidak bisa memperbaikinya, laporkan error aslinya.

## 11. Daftar larangan cepat

- Jangan membangun fitur *out of scope* (mobile, CCTV, SSO, email/WA, pembayaran, inventaris alat).
- Jangan mengubah jadwal akademik (`room_schedules`) akibat booking; gunakan overlay (BR-05).
- Jangan menyimpan token di localStorage atau mengirim refresh token di body.
- Jangan mengedit migrasi yang sudah ter-merge atau file hasil generate secara manual (`routeTree.gen.ts`, kode `sqlc`, tipe OpenAPI).
- Jangan menyusun SQL dari string input pengguna.
- Jangan menulis tes yang hanya memverifikasi mock.
- Jangan membiarkan data seed (password dev) aktif di produksi.
- Jangan menyembunyikan kegagalan: `catch` kosong dan `_ = err` dilarang.

## 12. Definition of Done (per tugas)

- [ ] Perilaku sesuai `brief.md` (aturan bisnis, peran, kode error)
- [ ] `api/openapi.yaml` diperbarui bila kontrak berubah (dan dikonfirmasi bila mengubah kontrak yang sudah ada)
- [ ] Migrasi (jika ada) punya `up` dan `down`, dan sudah dicoba maju-mundur
- [ ] Tes baru untuk perilaku baru, termasuk kasus gagal dan RBAC
- [ ] `make lint test` hijau; `pnpm build` lolos
- [ ] Tidak ada rahasia, log sensitif, atau kode mati yang tertinggal
- [ ] Laporan hasil memakai format bagian 10
