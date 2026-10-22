import { Link } from '@tanstack/react-router'
import {
  attendanceRules,
  bookingChangeRules,
  violationRules,
} from '@/config/attendance-rules'
import { QrCode } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { ReportCard } from '@/components/report-card'
import { StatusPill, type PillStatus } from '@/components/status-pill'
import { SECTION_X } from './constants'
import { CountUp, Reveal } from './motion'

function Heading({
  id,
  overline,
  children,
}: {
  id: string
  overline: string
  children: React.ReactNode
}) {
  return (
    <div className='mb-12 max-w-3xl'>
      <p className='mb-3 type-overline text-fg-secondary'>{overline}</p>
      <h2
        id={id}
        className='text-[clamp(36px,5vw,56px)] leading-[1.07] font-semibold tracking-[-0.02em] text-fg'
      >
        {children}
      </h2>
    </div>
  )
}

const STEPS: {
  time: string
  title: string
  text: string
  status: PillStatus
  label: string
}[] = [
  {
    time: '07:30',
    title: 'Cari slot kosong',
    text: 'Buka Jadwal Ruang, pilih tanggal dan jam. Status tiap ruang terlihat langsung.',
    status: 'Tersedia',
    label: 'Tersedia',
  },
  {
    time: '08:15',
    title: 'Ajukan',
    text: 'Isi keperluan dan jumlah peserta. Hasilnya tampil sebelum kamu kirim: disetujui otomatis atau menunggu validasi.',
    status: 'Diajukan',
    label: 'Diajukan',
  },
  {
    time: '08:40',
    title: 'Dapat keputusan',
    text: 'Kelas yang kosong dan riwayat baik disetujui otomatis. Lab, auditorium, ujian, dan sidang menunggu TU atau Kalab.',
    status: 'Disetujui',
    label: 'Disetujui',
  },
  {
    time: '10:00',
    title: 'Check-in dengan QR',
    text: `Pindai QR di pintu dalam jendela ${attendanceRules.checkInEarlyMinutes} menit sebelum sampai ${attendanceRules.checkInLateMinutes} menit sesudah mulai.`,
    status: 'Digunakan',
    label: 'Berlangsung',
  },
  {
    time: '12:05',
    title: 'Check-out dengan foto',
    text: 'Foto kondisi ruang saat pulang. Ada yang rusak? Laporkan dari situ juga.',
    status: 'Selesai',
    label: 'Selesai',
  },
]

/** Cara kerja: one day in a room as a timeline; each row slides in as it scrolls into view. */
export function HowItWorks() {
  return (
    <section
      id='cara-kerja'
      aria-labelledby='cara-kerja-title'
      className={cn(SECTION_X, 'scroll-mt-24 py-24')}
    >
      <Reveal>
        <Heading id='cara-kerja-title' overline='Cara kerja'>
          Satu hari di R-302.
        </Heading>
      </Reveal>
      <ol>
        {STEPS.map((s, i) => (
          <Reveal as='li' key={s.time} delay={i * 80}>
            <div className='grid items-baseline gap-x-8 gap-y-2 border-t py-7 md:grid-cols-[160px_minmax(0,1fr)_auto]'>
              <span className='text-[clamp(32px,4vw,48px)] leading-[1.08] font-semibold tracking-[-0.02em] text-fg tabular-nums'>
                {s.time}
              </span>
              <div>
                <h3 className='type-section text-fg'>{s.title}</h3>
                <p className='mt-1 max-w-xl type-body text-fg-secondary'>
                  {s.text}
                </p>
              </div>
              <StatusPill status={s.status} label={s.label} />
            </div>
          </Reveal>
        ))}
      </ol>
    </section>
  )
}

function BentoCard({
  className,
  children,
  dark,
}: {
  className?: string
  children: React.ReactNode
  dark?: boolean
}) {
  return (
    <article
      className={cn(
        'flex h-full flex-col gap-4 rounded-3xl border p-7 transition-[transform,box-shadow] duration-300 motion-safe:hover:-translate-y-1 motion-safe:hover:shadow-lg',
        dark ? 'border-transparent bg-inverse text-fg-inverse' : 'bg-card',
        className
      )}
    >
      {children}
    </article>
  )
}

/** Fitur: the bento. Each card carries a small piece of the real interface rather than an icon. */
export function Features() {
  return (
    <section
      id='fitur'
      aria-labelledby='fitur-title'
      className={cn(SECTION_X, 'scroll-mt-24 py-24')}
    >
      <Reveal>
        <Heading id='fitur-title' overline='Fitur'>
          Dibuat untuk pintu yang sama setiap hari.
        </Heading>
      </Reveal>
      <div className='grid gap-5 lg:grid-cols-[minmax(0,1fr)_400px]'>
        <Reveal>
          <BentoCard>
            <h3 className='type-card-title text-fg'>Disetujui otomatis</h3>
            <p className='type-body text-fg-secondary'>
              Kelas yang kosong dan pemohon dengan riwayat baik disetujui sistem
              seketika, tanpa antre di meja TU.
            </p>
            <Alert>
              <AlertTitle>R-302 · Kam, 23 Okt · 10:00 – 12:00</AlertTitle>
              <AlertDescription className='flex flex-wrap items-center gap-2'>
                <StatusPill status='Disetujui' label='Disetujui otomatis' />
                Peminjam diberi tahu lewat notifikasi.
              </AlertDescription>
            </Alert>
          </BentoCard>
        </Reveal>
        <Reveal delay={100}>
          <BentoCard dark>
            <QrCode aria-hidden className='size-8' />
            <h3 className='type-card-title'>QR di setiap pintu</h3>
            <p className='type-body text-fg-inverse/70'>
              Tiap ruang punya QR sendiri. Pindai dengan kamera ponsel untuk
              check-in; QR bocor? TU cukup mengganti tokennya.
            </p>
          </BentoCard>
        </Reveal>
      </div>
      <div className='mt-5 grid gap-5 md:grid-cols-3'>
        <Reveal>
          <BentoCard>
            <h3 className='type-card-title text-fg'>Anti bentrok</h3>
            <p className='type-body text-fg-secondary'>
              Slot yang sudah terisi tidak bisa dipilih. Bila ada yang
              mendahului, kamu diberi tahu ruang dan jam yang bentrok.
            </p>
            <Alert variant='error'>
              <AlertDescription>
                R-302 sudah terpakai pukul 10:00 – 12:00.
              </AlertDescription>
            </Alert>
          </BentoCard>
        </Reveal>
        <Reveal delay={100}>
          <BentoCard>
            <h3 className='type-card-title text-fg'>Booking beruntun</h3>
            <p className='type-body text-fg-secondary'>
              Satu pengajuan untuk beberapa tanggal, masing-masing dengan jam
              sendiri. Dibatalkan satu per satu atau sekaligus.
            </p>
            <ul className='flex flex-wrap gap-2'>
              {['Sel 21 Okt', 'Kam 23 Okt', 'Sel 28 Okt'].map((d) => (
                <li
                  key={d}
                  className='rounded-full border bg-subtle px-3 py-1 type-label text-fg'
                >
                  {d}
                </li>
              ))}
            </ul>
          </BentoCard>
        </Reveal>
        <Reveal delay={200}>
          <BentoCard>
            <h3 className='type-card-title text-fg'>Lapor kerusakan</h3>
            <p className='type-body text-fg-secondary'>
              Foto dan keterangan terkirim ke TU atau Kalab, dengan status yang
              bisa kamu pantau.
            </p>
            <ReportCard
              room='R-402'
              damage='Proyektor tidak menyala'
              status='Diproses'
              meta='2 jam yang lalu'
            />
          </BentoCard>
        </Reveal>
      </div>
    </section>
  )
}

const ROLES = [
  {
    name: 'Mahasiswa',
    text: 'Ajukan kelas dan lab, check-in dengan QR, check-out dengan foto, lapor kerusakan.',
  },
  {
    name: 'Organisasi',
    text: 'Semua milik mahasiswa, ditambah mengajukan auditorium dengan surat izin.',
  },
  {
    name: 'TU',
    text: 'Menyetujui kelas, auditorium, ujian, dan sidang; mengelola ruang, jadwal, dan akun mahasiswa.',
  },
  {
    name: 'Pengurus Lab',
    text: 'Menyetujui pengajuan lab dan memantau laporan kerusakannya.',
  },
  {
    name: 'OB',
    text: 'Memeriksa foto check-out dan membuat laporan kerusakan.',
  },
  {
    name: 'Admin',
    text: 'Mengelola pengguna, kebijakan tiap tipe ruang, dan membaca audit log.',
  },
] as const

/** Peran: six numbered cards. */
export function Roles() {
  return (
    <section
      id='peran'
      aria-labelledby='peran-title'
      className={cn(SECTION_X, 'scroll-mt-24 py-24')}
    >
      <Reveal>
        <Heading id='peran-title' overline='Peran'>
          Enam peran, satu papan.
        </Heading>
      </Reveal>
      <ul className='grid gap-5 md:grid-cols-2 lg:grid-cols-3'>
        {ROLES.map((r, i) => (
          <Reveal as='li' key={r.name} delay={(i % 3) * 90}>
            <article className='flex h-full flex-col gap-3 rounded-3xl border bg-card p-7 transition-[transform,box-shadow] duration-300 motion-safe:hover:-translate-y-1 motion-safe:hover:shadow-lg'>
              <span className='type-label text-fg-tertiary tabular-nums'>
                {String(i + 1).padStart(2, '0')}
              </span>
              <h3 className='type-section text-fg'>{r.name}</h3>
              <p className='type-body text-fg-secondary'>{r.text}</p>
            </article>
          </Reveal>
        ))}
      </ul>
    </section>
  )
}

const RULES = [
  {
    value: attendanceRules.checkInEarlyMinutes,
    unit: 'menit',
    text: 'Check-in dibuka sebelum jam mulai, dan ditutup sama lamanya sesudahnya.',
  },
  {
    value: bookingChangeRules.editCutoffMinutes,
    unit: 'menit',
    text: 'Batas terakhir mengubah booking sebelum jam mulai.',
  },
  {
    value: bookingChangeRules.lateCancelHours,
    unit: 'jam',
    text: 'Batal lebih dekat dari ini ke jam mulai dihitung satu poin.',
  },
  {
    value: violationRules.pendingExpiryHours,
    unit: 'jam',
    text: 'Pengajuan yang tak diputuskan dalam waktu ini kedaluwarsa.',
  },
  {
    value: attendanceRules.blockThresholdPoints,
    unit: 'poin',
    text: `Poin pelanggaran aktif yang memblokir pengajuan baru selama ${attendanceRules.blockDays} hari.`,
  },
] as const

/** Aturan main: the five numbers that matter, counting up on their way into view. */
export function Rules() {
  return (
    <section
      id='aturan'
      aria-labelledby='aturan-title'
      className={cn(SECTION_X, 'scroll-mt-24 py-24')}
    >
      <Reveal>
        <div className='rounded-[32px] bg-inverse p-8 text-fg-inverse sm:p-14'>
          <p className='mb-3 type-overline text-fg-inverse/60'>Aturan main</p>
          <h2
            id='aturan-title'
            className='mb-12 max-w-2xl text-[clamp(36px,5vw,56px)] leading-[1.07] font-semibold tracking-[-0.02em]'
          >
            Lima angka yang perlu diingat.
          </h2>
          <dl className='grid gap-x-10 gap-y-10 sm:grid-cols-2 lg:grid-cols-5'>
            {RULES.map((r) => (
              <div key={r.text} className='border-t border-fg-inverse/20 pt-5'>
                <dt className='text-[clamp(56px,7vw,88px)] leading-[1] font-semibold tracking-[-0.03em] tabular-nums'>
                  <CountUp to={r.value} />
                  <span className='ms-2 type-section text-fg-inverse/60'>
                    {r.unit}
                  </span>
                </dt>
                <dd className='mt-3 type-body text-fg-inverse/70'>{r.text}</dd>
              </div>
            ))}
          </dl>
        </div>
      </Reveal>
    </section>
  )
}

/** CTA: one white card; the button is Primary because Accent is already spent in the hero. */
export function Cta() {
  return (
    <section aria-labelledby='cta-title' className={cn(SECTION_X, 'py-24')}>
      <Reveal>
        <div className='mx-auto flex max-w-[1200px] flex-col items-center gap-6 rounded-[32px] border bg-card px-6 py-20 text-center'>
          <h2
            id='cta-title'
            className='max-w-3xl text-[clamp(40px,6vw,72px)] leading-[1.05] font-semibold tracking-[-0.03em] text-fg'
          >
            Pintu itu menunggu jadwalmu.
          </h2>
          <p className='max-w-md type-body-lg text-fg-secondary'>
            Masuk dengan NIM atau NIP yang dibuat TU kampus, lalu pilih slot
            kosong pertamamu.
          </p>
          <Button size='lg' className='h-12 px-8' asChild>
            <Link to='/sign-in'>Masuk</Link>
          </Button>
        </div>
      </Reveal>
    </section>
  )
}
