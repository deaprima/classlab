import { useMemo } from 'react'
import { Link } from '@tanstack/react-router'
import {
  AlertCircle,
  CalendarDays,
  ChevronRight,
  ClipboardCheck,
  Inbox,
} from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage } from '@/lib/api-error'
import { type RoomType } from '@/lib/api-types'
import { formatClock, formatDate, formatTime } from '@/lib/datetime'
import { useNow } from '@/hooks/use-now'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { BentoCard } from '@/components/bento-card'
import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/layout/page'
import { StatCard } from '@/components/stat-card'
import { StatusPill } from '@/components/status-pill'
import { useUsers } from '@/features/admin/users/api'
import { DecisionButtons } from '@/features/approvals/decision-buttons'
import { groupQueue } from '@/features/approvals/rules'
import { useAvailability } from '@/features/availability/api'
import { useBookings } from '@/features/bookings/api'
import { hasEnded } from '@/features/bookings/transitions'
import { Dashboard } from '@/features/dashboard'
import { greetingName } from '@/features/dashboard/selectors'
import { dayShort } from '@/features/my-bookings/format'
import { useReports } from '@/features/reports/api'
import { useRoomTypes } from '@/features/room-types/api'
import { useCheckouts } from '@/features/verify/api'
import { useAuditCount } from './api'
import { InUseCard } from './in-use-card'
import { RoomStatusCard } from './room-status-card'
import {
  inUseNow,
  occupancyPercent,
  roomStatusCounts,
  todayRange,
} from './rules'

const QUEUE_LIMIT = 4
// The queue is folded by request (BR-27), so a few more rows than shown are loaded.
const QUEUE_LOAD = 20

type StaffRole = 'TU' | 'KALAB' | 'ADMIN' | 'OB'

const COPY: Record<
  StaffRole,
  { description: string; statusTitle: string; totalLabel: string }
> = {
  TU: {
    description: 'Ringkasan pengajuan dan kondisi ruang hari ini.',
    statusTitle: 'Status Ruang',
    totalLabel: 'Total Ruang',
  },
  KALAB: {
    description: 'Ringkasan pengajuan dan kondisi lab hari ini.',
    statusTitle: 'Status Lab',
    totalLabel: 'Total Lab',
  },
  ADMIN: {
    description: 'Ringkasan pengguna, kebijakan, dan aktivitas sistem.',
    statusTitle: 'Status Ruang',
    totalLabel: 'Total Ruang',
  },
  OB: {
    description: 'Ringkasan check-out dan laporan hari ini.',
    statusTitle: 'Status Ruang',
    totalLabel: 'Total Ruang',
  },
}

/** Beranda of every role: the requesters' Dashboard, or the staff overview below (docs/DESIGN.md "Beranda"). */
export function Home() {
  const role = useAuthStore((s) => s.auth.user?.role)
  if (!role) return null
  if (role === 'MAHASISWA' || role === 'ORGANISASI') return <Dashboard />
  return <StaffHome role={role} />
}

/** The time in big digits with today's date under it, in WIB; it alone re-renders every second. */
function LiveClock() {
  const now = useNow(1000)
  return (
    <div className='flex flex-wrap items-center gap-x-3 sm:flex-col sm:items-end sm:gap-0'>
      <time
        dateTime={now.toISOString()}
        aria-label='Waktu sekarang'
        className='type-stat-l text-fg tabular-nums'
      >
        {formatClock(now)}
        <span className='ms-1 type-body text-fg-secondary'>WIB</span>
      </time>
      <p className='type-body text-fg-secondary'>{formatDate(now)}</p>
    </div>
  )
}

/** Counts shown as a number, or a dash while loading and when the request failed. */
const num = (value: number | undefined) =>
  value === undefined ? '–' : value.toLocaleString('id-ID')

function StaffHome({ role }: { role: StaffRole }) {
  const user = useAuthStore((s) => s.auth.user)
  const policies = useRoomTypes()
  const now = useNow(60_000)
  const copy = COPY[role]

  // TU and Kalab see the room types they approve; Admin and OB see every room.
  const types = useMemo<RoomType[] | undefined>(() => {
    if (role !== 'TU' && role !== 'KALAB') return undefined
    return (policies.data ?? [])
      .filter((p) => p.approver_roles.includes(role))
      .map((p) => p.type)
  }, [policies.data, role])

  const minute = Math.floor(now.getTime() / 60_000) * 60_000
  const availability = useAvailability({
    start_at: new Date(minute).toISOString(),
    end_at: new Date(minute + 60_000).toISOString(),
  })
  // TU and Kalab wait for their room types; until then nothing is counted.
  const scoped =
    availability.data && (types || !(role === 'TU' || role === 'KALAB'))
      ? availability.data
      : undefined
  const counts = useMemo(
    () => (scoped ? roomStatusCounts(scoped, types) : undefined),
    [scoped, types]
  )
  const inUse = useMemo(
    () => (scoped ? inUseNow(scoped, new Date(minute), types) : undefined),
    [scoped, types, minute]
  )

  if (!user) return null
  const occupancy = counts ? `${counts.inUse} / ${counts.total}` : '–'
  const occupancyCaption = counts
    ? `${occupancyPercent(counts)}% sedang terpakai`
    : ''

  return (
    <Page
      title={`Hai, ${greetingName(user)}`}
      barTitle='Beranda'
      description={copy.description}
      aside={<LiveClock />}
    >
      {availability.isError && (
        <Alert variant='error' className='mb-5'>
          <AlertCircle />
          <AlertDescription>
            {getErrorMessage(availability.error)}
          </AlertDescription>
        </Alert>
      )}
      <div className='mb-5 grid grid-cols-2 gap-4 lg:grid-cols-4'>
        {role === 'OB' ? (
          <ObStats now={now} occupancy={occupancy} caption={occupancyCaption} />
        ) : (
          <ApproverStats
            role={role}
            now={now}
            occupancy={occupancy}
            caption={occupancyCaption}
          />
        )}
      </div>
      <div className='grid grid-cols-1 items-start gap-5 lg:grid-cols-[minmax(0,1fr)_340px]'>
        <div className='flex min-w-0 flex-col gap-5'>
          {role === 'OB' ? (
            <CheckoutQueue now={now} />
          ) : (
            <ApprovalQueue role={role} />
          )}
          <InUseCard
            items={inUse}
            isPending={
              availability.isPending || (!inUse && !availability.isError)
            }
            isError={availability.isError}
          />
        </div>
        <RoomStatusCard
          title={copy.statusTitle}
          totalLabel={copy.totalLabel}
          counts={counts}
          action={
            <Button variant='ghost' size='sm' asChild>
              <Link to='/today'>
                <CalendarDays />
                Lihat jadwal
              </Link>
            </Button>
          }
          isPending={
            availability.isPending || (!counts && !availability.isError)
          }
          isError={availability.isError}
        />
      </div>
    </Page>
  )
}

function ApproverStats({
  role,
  now,
  occupancy,
  caption,
}: {
  role: 'TU' | 'KALAB' | 'ADMIN'
  now: Date
  occupancy: string
  caption: string
}) {
  const waiting = useBookings({ status: 'MENUNGGU', per_page: 1 })
  const running = useBookings({ status: 'BERLANGSUNG', per_page: 1 })
  const reports = useReports({ status: 'BARU', per_page: 1 })
  const policies = useRoomTypes()
  const users = useUsers({ is_active: true, per_page: 1 })
  const range = useMemo(() => todayRange(now), [now])
  const audit = useAuditCount(range)
  const waitingTotal = waiting.data?.meta.total

  if (role === 'ADMIN') {
    return (
      <>
        <StatCard
          title='Pengguna Aktif'
          value={num(users.data?.meta.total)}
          caption='akun aktif'
          showDot
        />
        <StatCard
          title='Approval Menunggu'
          value={num(waitingTotal)}
          caption='menunggu keputusan'
        />
        <StatCard
          title='Tipe Ruang'
          value={num(policies.data?.length)}
          caption='dengan kebijakan aktif'
        />
        <StatCard
          title='Log Hari Ini'
          value={num(audit.data)}
          caption='aktivitas tercatat'
        />
      </>
    )
  }
  const lab = role === 'KALAB'
  return (
    <>
      <StatCard
        title={lab ? 'Lab Menunggu' : 'Pengajuan Menunggu'}
        value={num(waitingTotal)}
        caption='menunggu keputusan'
        showDot={!!waitingTotal}
      />
      <StatCard
        title={lab ? 'Lab Dipakai' : 'Ruang Dipakai'}
        value={occupancy}
        caption={caption}
      />
      <StatCard
        title={lab ? 'Laporan Lab' : 'Laporan Baru'}
        value={num(reports.data?.meta.total)}
        caption='belum diterima'
      />
      <StatCard
        title='Berlangsung Sekarang'
        value={num(running.data?.meta.total)}
        caption='sudah check-in'
      />
    </>
  )
}

function ObStats({
  now,
  occupancy,
  caption,
}: {
  now: Date
  occupancy: string
  caption: string
}) {
  const range = useMemo(() => todayRange(now), [now])
  const checkouts = useCheckouts({ ...range, per_page: 100 })
  const mine = useReports({ per_page: 1 })
  const rows = checkouts.data?.data
  const problems = rows?.filter((c) => c.condition === 'ADA_MASALAH').length

  return (
    <>
      <StatCard
        title='Check-out Hari Ini'
        value={num(checkouts.data?.meta.total)}
        caption='ruang ditinggalkan'
      />
      <StatCard
        title='Ada Masalah'
        value={num(problems)}
        caption='dilaporkan peminjam'
        showDot={!!problems}
      />
      <StatCard title='Ruang Dipakai' value={occupancy} caption={caption} />
      <StatCard
        title='Laporan Saya'
        value={num(mine.data?.meta.total)}
        caption='laporan kerusakan'
      />
    </>
  )
}

/** "Perlu Persetujuan": the first pending requests, decided right here. */
function ApprovalQueue({ role }: { role: 'TU' | 'KALAB' | 'ADMIN' }) {
  const now = useNow(60_000)
  const queue = useBookings({ status: 'MENUNGGU', per_page: QUEUE_LOAD })
  const items = useMemo(
    () => groupQueue(queue.data?.data ?? []).slice(0, QUEUE_LIMIT),
    [queue.data]
  )

  return (
    <BentoCard
      title={role === 'KALAB' ? 'Perlu Persetujuan Lab' : 'Perlu Persetujuan'}
    >
      {queue.isError && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(queue.error)}</AlertDescription>
        </Alert>
      )}
      {queue.isPending ? (
        <Skeleton className='h-48' />
      ) : items.length === 0 ? (
        <EmptyState
          icon={ClipboardCheck}
          title='Tidak ada yang menunggu'
          description='Semua pengajuan sudah diputuskan.'
          className='py-4'
        />
      ) : (
        <ul className='flex flex-col gap-3'>
          {items.map(({ lead: b, count }) => (
            <li
              key={b.id}
              className='flex flex-wrap items-center justify-between gap-3 rounded-xl bg-subtle p-3'
            >
              <div className='min-w-0'>
                <p className='truncate type-card-title text-fg'>
                  {b.room.code}{' '}
                  <span className='font-normal text-fg-secondary'>
                    {b.purpose}
                  </span>
                </p>
                <p className='truncate type-body-sm text-fg-secondary'>
                  {b.requester.name} · {dayShort(b.start_at)} ·{' '}
                  {formatTime(b.start_at)}–{formatTime(b.end_at)}
                  {count > 1 && ` · +${count - 1} tanggal`}
                </p>
              </div>
              <DecisionButtons
                booking={b}
                dateCount={count}
                ended={hasEnded(b, now.getTime())}
                size='sm'
              />
            </li>
          ))}
        </ul>
      )}
      <Button variant='ghost' className='w-fit' asChild>
        <Link to='/approvals'>
          Lihat semua pengajuan
          <ChevronRight />
        </Link>
      </Button>
    </BentoCard>
  )
}

/** OB's list: the latest check-outs of today and how each room was left. */
function CheckoutQueue({ now }: { now: Date }) {
  const range = useMemo(() => todayRange(now), [now])
  const checkouts = useCheckouts({ ...range, per_page: 100 })
  const rows = (checkouts.data?.data ?? []).slice(0, QUEUE_LIMIT)

  return (
    <BentoCard title='Check-out Terbaru'>
      {checkouts.isError && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertDescription>
            {getErrorMessage(checkouts.error)}
          </AlertDescription>
        </Alert>
      )}
      {checkouts.isPending ? (
        <Skeleton className='h-48' />
      ) : rows.length === 0 ? (
        <EmptyState
          icon={Inbox}
          title='Belum ada check-out hari ini'
          description='Ruang yang baru ditinggalkan tampil di sini.'
          className='py-4'
        />
      ) : (
        <ul className='flex flex-col gap-3'>
          {rows.map((c) => (
            <li
              key={c.booking_id}
              className='flex flex-wrap items-center justify-between gap-3 rounded-xl bg-subtle p-3'
            >
              <div className='min-w-0'>
                <p className='truncate type-card-title text-fg'>
                  {c.room_code}{' '}
                  <span className='font-normal text-fg-secondary'>
                    {c.room_name}
                  </span>
                </p>
                <p className='truncate type-body-sm text-fg-secondary'>
                  selesai {formatTime(c.checked_out_at)}
                  {c.note && ` · ${c.note}`}
                </p>
              </div>
              <div className='flex items-center gap-2'>
                <StatusPill
                  status={
                    c.condition === 'ADA_MASALAH' ? 'Diajukan' : 'Selesai'
                  }
                  label={
                    c.condition === 'ADA_MASALAH' ? 'Ada masalah' : 'Normal'
                  }
                />
                <Button variant='secondary' size='sm' className='h-8' asChild>
                  <Link to='/reports'>
                    Lapor
                    <span className='sr-only'> kerusakan {c.room_code}</span>
                  </Link>
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
      <Button variant='ghost' className='w-fit' asChild>
        <Link to='/verify'>
          Lihat semua verifikasi
          <ChevronRight />
        </Link>
      </Button>
    </BentoCard>
  )
}
