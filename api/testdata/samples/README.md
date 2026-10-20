# Contoh berkas impor CSV

Berkas ini lolos pratinjau terhadap data seed (`go run ./cmd/seed`) dan dipakai tes integrasi.

- `schedule.csv` untuk `POST /imports/schedules` (jadwal akademik, BR-23). Kode ruang mengacu ke ruang seed dan slot-nya tidak bentrok dengan jadwal seed.
- `users.csv` untuk `POST /imports/users` (akun mahasiswa, BR-24). Kirim juga `semester`, mis. `2026-GANJIL`.

Pemisah `,` atau `;` dan BOM UTF-8 diterima. Isi file tidak pernah disimpan di server; hanya hash dan ringkasan.
