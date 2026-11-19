## Ringkasan

<!-- Apa yang berubah, dan kenapa? Tautkan issue terkait, mis. "Closes #12". -->

## Cara menguji

<!-- Perintah atau langkah untuk memeriksa perubahan ini. -->

## Yang belum dikerjakan / keputusan yang diambil

<!-- Jujur soal bagian yang belum selesai, dan jelaskan keputusan desain, terutama bila mengubah kontrak API atau skema. -->

## Tangkapan layar

<!-- Untuk perubahan tampilan: lebar 360 px dan 1280 px. Hapus bagian ini bila tidak relevan. -->

## Checklist

- [ ] `make lint`, `make test`, dan `pnpm run build` hijau
- [ ] Ada tes baru untuk perilaku baru, termasuk kasus gagal dan hak akses
- [ ] `api/openapi.yaml` dan tipe hasil generate (`make gen`) diperbarui bila kontrak berubah
- [ ] Migrasi baru (bila ada) punya `up` dan `down`, dan migrasi lama tidak diedit
- [ ] Tidak ada rahasia, data pribadi, atau log sensitif yang ikut masuk
