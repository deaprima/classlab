import { useState } from 'react'
import { Inbox, Plus } from 'lucide-react'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { BentoCard } from '@/components/bento-card'
import { DateBadge } from '@/components/date-badge'
import { Dropzone } from '@/components/dropzone'
import { EmptyState } from '@/components/empty-state'
import { ReportCard } from '@/components/report-card'
import {
  ResponsiveDataList,
  type DataColumn,
} from '@/components/responsive-data-list'
import { RoomTile } from '@/components/room-tile'
import { SegmentedControl } from '@/components/segmented-control'
import { SlotBlock } from '@/components/slot-block'
import { StatCard } from '@/components/stat-card'
import { StatusPill, type PillStatus } from '@/components/status-pill'
import { Step } from '@/components/step'
import { ThemeSwitch } from '@/components/theme-switch'

const PILLS: PillStatus[] = [
  'Tersedia',
  'Digunakan',
  'Diajukan',
  'Terbatas',
  'Menunggu',
  'Disetujui',
  'Ditolak',
  'Diterima',
  'Diproses',
  'Selesai',
]

const SURFACES = [
  ['canvas', 'bg-canvas'],
  ['surface', 'bg-surface'],
  ['subtle', 'bg-subtle'],
  ['muted', 'bg-muted'],
  ['inverse', 'bg-inverse'],
  ['accent-subtle', 'bg-accent-subtle'],
  ['error-subtle', 'bg-error-subtle'],
  ['brand', 'bg-brand'],
] as const

const TEXTS = [
  ['primary', 'text-fg'],
  ['secondary', 'text-fg-secondary'],
  ['tertiary', 'text-fg-tertiary'],
  ['accent', 'text-fg-accent'],
  ['error', 'text-fg-error'],
] as const

const TYPES = [
  ['type-stat-xl', 'Stat XL 40/48'],
  ['type-stat-l', 'Stat L 32/40'],
  ['type-page-title', 'Page Title 24/32'],
  ['type-section', 'Section 18/26'],
  ['type-card-title', 'Card Title 14/20'],
  ['type-body-lg', 'Body Large 16/24'],
  ['type-body', 'Body Default 14/20'],
  ['type-body-sm', 'Body Small 13/18'],
  ['type-button', 'Label Button 14/20'],
  ['type-label', 'Label Default 12/16'],
  ['type-label-sm', 'Label Small 11/14'],
  ['type-overline', 'Overline 11/14'],
] as const

type Row = { room: string; who: string; time: string; status: PillStatus }
const ROWS: Row[] = [
  {
    room: 'R-301',
    who: 'Andi Pratama',
    time: '08:00 – 10:00',
    status: 'Menunggu',
  },
  {
    room: 'LAB-RPL-02',
    who: 'Bunga Lestari',
    time: '10:00 – 12:00',
    status: 'Disetujui',
  },
  {
    room: 'Auditorium',
    who: 'HIMA-TI',
    time: '13:00 – 16:00',
    status: 'Ditolak',
  },
]
const COLUMNS: DataColumn<Row>[] = [
  { key: 'room', header: 'Ruang', cell: (r) => r.room, primary: true },
  { key: 'who', header: 'Peminjam', cell: (r) => r.who },
  { key: 'time', header: 'Waktu', cell: (r) => r.time },
  {
    key: 'status',
    header: 'Status',
    cell: (r) => <StatusPill status={r.status} />,
  },
  {
    key: 'act',
    header: 'Aksi',
    actions: true,
    cell: () => (
      <Button size='sm' variant='secondary'>
        Detail
      </Button>
    ),
  },
]

function Section({
  title,
  children,
}: {
  title: string
  children: React.ReactNode
}) {
  return (
    <section className='flex flex-col gap-4'>
      <h2 className='type-overline text-fg-secondary'>{title}</h2>
      {children}
    </section>
  )
}

/** Development-only catalog of the Phase 6.0 foundation (tokens, components); see docs/DESIGN.md. */
export function DesignCatalog() {
  const [view, setView] = useState('pengajuan')
  const [files, setFiles] = useState<string[]>([])

  return (
    <div className='min-h-svh bg-canvas'>
      <header className='sticky top-0 z-40 flex h-18 items-center justify-between border-b bg-surface px-4 md:px-6'>
        <div>
          <p className='type-section text-fg'>Katalog desain</p>
          <p className='type-body-sm text-fg-secondary'>
            Fondasi Phase 6.0. Hanya tampil di mode dev.
          </p>
        </div>
        <ThemeSwitch />
      </header>

      <main className='mx-auto flex max-w-6xl flex-col gap-10 px-4 py-8 md:px-6'>
        <Section title='Warna (bg)'>
          <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
            {SURFACES.map(([name, cls]) => (
              <div key={name} className='flex flex-col gap-2'>
                <div
                  className={`h-16 rounded-xl border border-line-strong ${cls}`}
                />
                <p className='type-label text-fg-secondary'>{name}</p>
              </div>
            ))}
          </div>
          <div className='flex flex-wrap gap-4 rounded-2xl border bg-card p-5'>
            {TEXTS.map(([name, cls]) => (
              <p key={name} className={`type-card-title ${cls}`}>
                text/{name}
              </p>
            ))}
          </div>
        </Section>

        <Section title='Tipografi (DM Sans)'>
          <div className='flex flex-col gap-3 rounded-2xl border bg-card p-5'>
            {TYPES.map(([cls, label]) => (
              <p key={cls} className={`${cls} text-fg`}>
                {label}
              </p>
            ))}
          </div>
        </Section>

        <Section title='Tombol'>
          <div className='flex flex-wrap items-center gap-3 rounded-2xl border bg-card p-5'>
            <Button>Primary</Button>
            <Button variant='secondary'>Secondary</Button>
            <Button variant='accent'>Accent</Button>
            <Button variant='ghost'>Ghost</Button>
            <Button size='sm'>
              <Plus /> Small
            </Button>
            <Button disabled>Disabled</Button>
            <Button size='icon' variant='secondary' aria-label='Tambah'>
              <Plus />
            </Button>
          </div>
        </Section>

        <Section title='Status Pill'>
          <div className='flex flex-wrap gap-3 rounded-2xl border bg-card p-5'>
            {PILLS.map((s) => (
              <StatusPill key={s} status={s} />
            ))}
          </div>
        </Section>

        <Section title='Stat Card dan Card bento'>
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
            <StatCard
              title='Booking hari ini'
              value={12}
              trend='+3'
              caption='dari kemarin'
            />
            <StatCard
              title='Menunggu'
              value={4}
              showDot
              caption='perlu keputusan'
            />
            <StatCard
              title='Terpakai'
              value='78%'
              caption='minggu ini'
              tone='subtle'
            />
            <StatCard title='Laporan' value={2} caption='belum selesai' />
          </div>
          <div className='grid gap-4 lg:grid-cols-[2fr_1fr]'>
            <BentoCard
              title='Jadwal hari ini'
              action={
                <Button size='sm' variant='ghost'>
                  Lihat semua
                </Button>
              }
            >
              <div className='grid grid-cols-3 gap-2'>
                <SlotBlock status='Tersedia' time='07:00' />
                <SlotBlock
                  status='Digunakan'
                  time='08:00'
                  label='Pemrograman Web'
                />
                <SlotBlock status='Diajukan' time='10:00' label='Rapat HIMA' />
                <SlotBlock status='Terbatas' time='12:00' />
                <SlotBlock status='Tersedia' time='13:00' />
                <SlotBlock status='Digunakan' time='15:00' label='Basis Data' />
              </div>
            </BentoCard>
            <BentoCard title='Tanggal' tone='subtle'>
              <div className='flex gap-3'>
                <DateBadge date='2026-10-19' />
                <DateBadge date='2026-10-20' />
                <DateBadge date='2026-10-21' />
              </div>
            </BentoCard>
          </div>
        </Section>

        <Section title='Alert dan toast'>
          <div className='grid gap-3 md:grid-cols-2'>
            <Alert variant='info'>
              <AlertTitle>Info</AlertTitle>
              <AlertDescription>
                Check-in dibuka 15 menit sebelum mulai.
              </AlertDescription>
            </Alert>
            <Alert variant='warning'>
              <AlertTitle>Menunggu persetujuan</AlertTitle>
              <AlertDescription>Kedaluwarsa dalam 23 jam.</AlertDescription>
            </Alert>
            <Alert variant='success'>
              <AlertTitle>Disetujui</AlertTitle>
              <AlertDescription>
                R-301, Sen 19 Okt 2026, 08:00 – 10:00.
              </AlertDescription>
            </Alert>
            <Alert variant='error'>
              <AlertTitle>Bentrok</AlertTitle>
              <AlertDescription>
                R-302 sudah terisi. Pilih jam lain.
              </AlertDescription>
            </Alert>
          </div>
          <div className='flex gap-3'>
            <Button
              variant='secondary'
              onClick={() => toast('Pengajuan terkirim')}
            >
              Toast
            </Button>
            <Button
              variant='secondary'
              onClick={() => toast.error('Gagal menyimpan. Coba lagi.')}
            >
              Toast error
            </Button>
          </div>
        </Section>

        <Section title='Form'>
          <div className='grid gap-4 rounded-2xl border bg-card p-5 md:grid-cols-2'>
            <div className='grid gap-2'>
              <Label htmlFor='d-nim'>NIM / NIP</Label>
              <Input id='d-nim' placeholder='2021101001' />
            </div>
            <div className='grid gap-2'>
              <Label htmlFor='d-err'>Alasan (error)</Label>
              <Input id='d-err' aria-invalid defaultValue='abc' />
              <p className='type-body-sm text-fg-error'>Minimal 5 karakter.</p>
            </div>
            <div className='grid gap-2 md:col-span-2'>
              <Label htmlFor='d-ta'>Keperluan</Label>
              <Textarea id='d-ta' placeholder='Tulis keperluan' />
            </div>
            <div className='flex items-center gap-2'>
              <Checkbox id='d-cb' defaultChecked />
              <Label htmlFor='d-cb'>Setuju aturan ruang</Label>
            </div>
            <div className='flex items-center gap-2'>
              <Switch id='d-sw' />
              <Label htmlFor='d-sw'>Berulang tiap minggu</Label>
            </div>
          </div>
        </Section>

        <Section title='Segmented, Dropzone, Stepper'>
          <div className='grid gap-4 lg:grid-cols-2'>
            <div className='flex flex-col gap-4 rounded-2xl border bg-card p-5'>
              <SegmentedControl
                aria-label='Tampilan'
                value={view}
                onValueChange={setView}
                options={[
                  { value: 'pengajuan', label: 'Pengajuan', count: 3 },
                  { value: 'riwayat', label: 'Riwayat' },
                ]}
              />
              <Dropzone
                accept='application/pdf,image/jpeg'
                hint='PDF atau JPEG, maks 5 MB'
                onFiles={(f) => setFiles(f.map((x) => x.name))}
              />
              {files.length > 0 && (
                <p className='type-body-sm text-fg-secondary'>
                  Dipilih: {files.join(', ')}
                </p>
              )}
            </div>
            <div className='rounded-2xl border bg-card p-5'>
              <ol>
                <Step
                  state='done'
                  title='Diajukan'
                  description='Sen, 19 Okt 2026'
                  meta='08:00'
                  hasNext
                />
                <Step
                  state='current'
                  title='Menunggu persetujuan'
                  description='TU memeriksa pengajuan'
                  hasNext
                />
                <Step state='upcoming' title='Check-in' hasNext />
                <Step state='upcoming' title='Check-out' />
              </ol>
            </div>
          </div>
        </Section>

        <Section title='Room Tile dan Report Card'>
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
            <RoomTile code='R-301' capacity={40} status='Tersedia' />
            <RoomTile code='LAB-RPL-02' capacity={30} status='Digunakan' />
            <ReportCard
              room='R-302'
              damage='Proyektor tidak menyala'
              status='Diproses'
              meta='Dilaporkan Agus, 2 jam lalu'
            />
            <ReportCard
              room='LAB-RPL-02'
              damage='Kursi patah'
              status='Selesai'
            />
          </div>
        </Section>

        <Section title='Daftar data (tabel di desktop, kartu di mobile)'>
          <ResponsiveDataList
            caption='Contoh pengajuan'
            rows={ROWS}
            columns={COLUMNS}
            rowKey={(r) => r.room}
          />
        </Section>

        <Section title='Empty State dan Dialog'>
          <div className='rounded-2xl border bg-card'>
            <EmptyState
              icon={Inbox}
              title='Belum ada pengajuan'
              description='Pengajuan yang kamu buat akan muncul di sini.'
              action={<Button variant='accent'>Ajukan Ruang</Button>}
            />
          </div>
          <div>
            <Dialog>
              <DialogTrigger asChild>
                <Button variant='secondary'>Buka dialog</Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Batalkan pengajuan</DialogTitle>
                  <DialogDescription>
                    Pembatalan kurang dari 2 jam sebelum mulai menambah 1 poin
                    pelanggaran.
                  </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                  <Button variant='secondary'>Kembali</Button>
                  <Button>Batalkan</Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>
        </Section>
      </main>
    </div>
  )
}
