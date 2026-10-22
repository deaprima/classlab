import { Link } from '@tanstack/react-router'
import {
  attendanceRules,
  bookingChangeRules,
  violationRules,
} from '@/config/attendance-rules'
import { BentoCard } from '@/components/bento-card'
import { Page } from '@/components/layout/page'

const STEPS = [
  {
    title: 'Cek jadwal dan ajukan ruang',
    body: 'Buka Jadwal Ruang, pilih ruang dan jam yang kosong, lalu isi form pengajuan. Kamu bisa memilih beberapa tanggal sekaligus untuk kegiatan beruntun.',
  },
  {
    title: 'Tunggu persetujuan',
    body: `Kelas yang kosong dan riwayatmu baik disetujui otomatis. Lab, auditorium, ujian, sidang, dan acara organisasi dicek TU atau Pengurus Lab. Pengajuan yang belum diputuskan kedaluwarsa dalam ${violationRules.pendingExpiryHours} jam.`,
  },
  {
    title: 'Check-in dengan QR di pintu',
    body: `Pindai QR yang menempel di pintu ruang dengan kamera ponsel. Check-in dibuka ${attendanceRules.checkInEarlyMinutes} menit sebelum mulai dan ditutup ${attendanceRules.checkInLateMinutes} menit sesudahnya.`,
  },
  {
    title: 'Check-out dengan foto',
    body: 'Sebelum pergi, buka Check-out di Dashboard, pilih kondisi ruang, lalu ambil foto langsung dari kamera. Kalau ada kerusakan, tulis catatannya: laporan dibuat otomatis untuk pengelola.',
  },
  {
    title: 'Lihat riwayat dan laporkan masalah',
    body: 'Booking yang selesai tercatat di Riwayat. Menemukan ruang rusak sebelum kamu pakai? Laporkan lewat Lapor Kerusakan supaya tidak dianggap ulahmu.',
  },
] as const

const POINTS = [
  {
    kind: 'Tidak hadir',
    when: `Tidak check-in sampai ${attendanceRules.checkInLateMinutes} menit setelah jam mulai.`,
    points: violationRules.pointsNoShow,
  },
  {
    kind: 'Tanpa check-out',
    when: 'Tidak check-out setelah jam selesai berlalu (ada tenggang beberapa menit).',
    points: violationRules.pointsNoCheckout,
  },
  {
    kind: 'Batal terlambat',
    when: `Membatalkan booking yang sudah disetujui kurang dari ${bookingChangeRules.lateCancelHours} jam sebelum mulai.`,
    points: violationRules.pointsLateCancel,
  },
] as const

function Section({
  id,
  title,
  children,
}: {
  id: string
  title: string
  children: React.ReactNode
}) {
  return (
    <div id={id} className='scroll-mt-24'>
      <BentoCard title={title}>{children}</BentoCard>
    </div>
  )
}

/** Panduan: how to use ClassLab step by step, and what the violation points and limits mean. */
export function Guide() {
  return (
    <Page
      title='Panduan'
      description='Cara memakai ClassLab dan aturan mainnya.'
    >
      <div className='grid items-start gap-5 lg:grid-cols-2'>
        <div className='flex flex-col gap-5'>
          <Section id='cara-pakai' title='Cara memakai ClassLab'>
            <ol className='flex flex-col gap-4'>
              {STEPS.map((step, i) => (
                <li key={step.title} className='flex gap-3'>
                  <span
                    aria-hidden
                    className='flex size-7 shrink-0 items-center justify-center rounded-full bg-inverse type-label text-fg-inverse'
                  >
                    {i + 1}
                  </span>
                  <div className='min-w-0'>
                    <h3 className='type-card-title text-fg'>{step.title}</h3>
                    <p className='type-body text-fg-secondary'>{step.body}</p>
                  </div>
                </li>
              ))}
            </ol>
          </Section>

          <Section id='blokir' title='Diblokir mengajukan'>
            <p className='type-body text-fg-secondary'>
              Saat poin aktifmu mencapai {attendanceRules.blockThresholdPoints},
              kamu tidak bisa mengajukan ruang baru selama{' '}
              {attendanceRules.blockDays} hari. Booking yang sudah disetujui
              tetap berlaku, dan kamu tetap bisa check-in, check-out, serta
              membatalkan.
            </p>
          </Section>
        </div>

        <div className='flex flex-col gap-5'>
          <Section id='poin' title='Poin pelanggaran'>
            <p className='type-body text-fg-secondary'>
              Poin dicatat otomatis bila booking tidak dipakai sebagaimana
              mestinya. Poin berlaku {violationRules.windowDays} hari sejak
              dicatat.
            </p>
            <ul className='flex flex-col divide-y rounded-xl border'>
              {POINTS.map((p) => (
                <li
                  key={p.kind}
                  className='flex items-start justify-between gap-4 p-4'
                >
                  <div className='min-w-0'>
                    <p className='type-card-title text-fg'>{p.kind}</p>
                    <p className='type-body-sm text-fg-secondary'>{p.when}</p>
                  </div>
                  <span className='shrink-0 type-label text-fg'>
                    {p.points} poin
                  </span>
                </li>
              ))}
            </ul>
            <p className='type-body-sm text-fg-secondary'>
              Pengguna yang tidak bisa check-out karena kendala perangkat dapat
              meminta Admin membatalkan (annul) poinnya.{' '}
              <Link to='/account' className='underline underline-offset-4'>
                Lihat poinmu di Akun Saya
              </Link>
              .
            </p>
          </Section>

          <Section id='aturan' title='Aturan lain yang perlu diketahui'>
            <ul className='flex list-disc flex-col gap-2 ps-5 type-body text-fg-secondary'>
              <li>
                Ubah pengajuan bisa sampai{' '}
                {bookingChangeRules.editCutoffMinutes} menit sebelum mulai.
                Mengubah booking yang sudah disetujui bisa meminta persetujuan
                ulang.
              </li>
              <li>
                Batal kurang dari {bookingChangeRules.lateCancelHours} jam
                sebelum mulai menambah {violationRules.pointsLateCancel} poin.
                Batal lebih awal tidak kena poin.
              </li>
              <li>
                Booking aktif (menunggu, disetujui, atau berlangsung) dibatasi{' '}
                {violationRules.maxActiveBookings} untuk mahasiswa dan{' '}
                {violationRules.maxActiveBookingsOrganization} untuk organisasi.
                Pengajuan beruntun dihitung satu.
              </li>
              <li>
                Pengajuan yang tidak diputuskan dalam{' '}
                {violationRules.pendingExpiryHours} jam kedaluwarsa tanpa poin
                dan slotnya dibuka lagi.
              </li>
              <li>
                Foto check-out hanya dilihat pengelola dan dihapus otomatis
                setelah {violationRules.photoRetentionDays} hari.
              </li>
            </ul>
          </Section>
        </div>
      </div>
    </Page>
  )
}
