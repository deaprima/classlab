import { useState } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { AlertCircle, ChevronLeft, ChevronRight } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type Booking, type BookingListQuery } from '@/lib/api-types'
import { formatDuration, toApiDateTime, todayWib } from '@/lib/datetime'
import { useNow } from '@/hooks/use-now'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/layout/page'
import {
  ResponsiveDataList,
  type DataColumn,
} from '@/components/responsive-data-list'
import { SegmentedControl } from '@/components/segmented-control'
import { StatCard } from '@/components/stat-card'
import { BookingStatusBadge } from '@/components/status-badge'
import { useBookings } from '@/features/bookings/api'
import { BookingDetailDialog } from './detail-dialog'
import { whenShort } from './format'
import { RowMenu } from './parts'

const route = getRouteApi('/_authenticated/history')

const PER_PAGE = 10
const ALL_TIME = 'ALL'

type Tab = 'ALL' | 'SELESAI' | 'DITOLAK'

function tabOf(status: string | undefined): Tab {
  return status === 'SELESAI' || status === 'DITOLAK' ? status : 'ALL'
}

/** The last 12 months, newest first, as `YYYY-MM`. */
function recentMonths(today: string): string[] {
  const [y, m] = today.split('-').map(Number)
  return Array.from({ length: 12 }, (_, i) => {
    const d = new Date(Date.UTC(y, m - 1 - i, 1))
    return d.toISOString().slice(0, 7)
  })
}

const monthLabel = (month: string) =>
  new Intl.DateTimeFormat('id-ID', {
    timeZone: 'UTC',
    month: 'short',
    year: 'numeric',
  }).format(new Date(`${month}-01T00:00:00Z`))

/** [from, to) of a month as API timestamps (WIB). */
function monthRange(month: string): { from: string; to: string } {
  const [y, m] = month.split('-').map(Number)
  const next = new Date(Date.UTC(y, m, 1)).toISOString().slice(0, 10)
  return {
    from: toApiDateTime(`${month}-01`, '00:00'),
    to: toApiDateTime(next, '00:00'),
  }
}

/** Riwayat (Figma): how many bookings, how many hours used, and the full log with filters. */
export function History() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const now = useNow()
  const [detailId, setDetailId] = useState<string | null>(null)
  const today = todayWib(now)
  const months = recentMonths(today)
  const month =
    search.month && months.includes(search.month) ? search.month : undefined
  const tab = tabOf(search.status)
  const page = search.page ?? 1

  const range: Pick<BookingListQuery, 'from' | 'to'> = month
    ? monthRange(month)
    : {}
  const base = { mine: true, ...range }
  const list = useBookings({
    ...base,
    status: tab === 'ALL' ? undefined : tab,
    page,
    per_page: PER_PAGE,
  })
  // Counts and hours come from the API's totals, not from the page on screen.
  const total = useBookings({ ...base, per_page: 1 })
  const finished = useBookings({ ...base, status: 'SELESAI', per_page: 100 })
  const refused = useBookings({ ...base, status: 'DITOLAK', per_page: 1 })

  const finishedRows = finished.data?.data ?? []
  const finishedTotal = finished.data?.meta?.total ?? 0
  const minutesUsed = finishedRows.reduce(
    (sum, b) => sum + (Date.parse(b.end_at) - Date.parse(b.start_at)) / 60_000,
    0
  )
  const totalCount = total.data?.meta?.total
  const refusedCount = refused.data?.meta?.total

  const rows = list.data?.data ?? []
  const meta = list.data?.meta
  const from = rows.length > 0 ? (page - 1) * PER_PAGE + 1 : 0
  const to = (page - 1) * PER_PAGE + rows.length
  const pages = Math.max(1, Math.ceil((meta?.total ?? 0) / PER_PAGE))

  const go = (patch: Partial<typeof search>) =>
    navigate({ search: (prev) => ({ ...prev, ...patch }) })

  const columns: DataColumn<Booking>[] = [
    {
      key: 'room',
      header: 'Ruang',
      primary: true,
      cell: (b) => (
        <span className='type-card-title text-fg'>{b.room.code}</span>
      ),
    },
    {
      key: 'purpose',
      header: 'Kegiatan',
      cell: (b) => <span className='text-fg-secondary'>{b.purpose}</span>,
    },
    {
      key: 'when',
      header: 'Waktu',
      cell: (b) => (
        <span className='text-fg-secondary'>
          {whenShort(b.start_at, b.end_at)}
        </span>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      cell: (b) => <BookingStatusBadge status={b.status} />,
    },
    {
      key: 'actions',
      header: 'Aksi',
      actions: true,
      className: 'w-12 text-end',
      cell: (b) => (
        <RowMenu
          booking={b}
          now={now}
          withChanges={false}
          onDetail={(row) => setDetailId(row.id)}
        />
      ),
    },
  ]

  return (
    <Page title='Riwayat'>
      <div className='grid gap-5 sm:grid-cols-3'>
        <StatCard title='Total Booking' value={totalCount ?? '–'} />
        <StatCard
          title='Jam Terpakai'
          value={
            finished.isSuccess
              ? `${finishedTotal > finishedRows.length ? '≥ ' : ''}${formatDuration(Math.round(minutesUsed))}`
              : '–'
          }
          caption={
            finishedTotal > finishedRows.length
              ? `dari ${finishedRows.length} booking terbaru`
              : undefined
          }
        />
        <StatCard title='Ditolak' value={refusedCount ?? '–'} />
      </div>

      <section
        aria-label='Riwayat booking'
        className='mt-5 flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
      >
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <h2 className='type-section text-fg'>Riwayat Booking</h2>
          <Select
            value={month ?? ALL_TIME}
            onValueChange={(v) =>
              go({ month: v === ALL_TIME ? undefined : v, page: undefined })
            }
          >
            <SelectTrigger aria-label='Bulan' className='w-40'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL_TIME}>Semua waktu</SelectItem>
              {months.map((m) => (
                <SelectItem key={m} value={m}>
                  {monthLabel(m)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <SegmentedControl
          aria-label='Filter status'
          value={tab}
          onValueChange={(t) =>
            go({ status: t === 'ALL' ? undefined : t, page: undefined })
          }
          options={[
            { value: 'ALL', label: 'Semua', count: totalCount },
            {
              value: 'SELESAI',
              label: 'Selesai',
              count: finished.data ? finishedTotal : undefined,
            },
            { value: 'DITOLAK', label: 'Ditolak', count: refusedCount },
          ]}
        />

        {list.isError && (
          <Alert variant='error'>
            <AlertCircle />
            <AlertDescription>{getErrorMessage(list.error)}</AlertDescription>
          </Alert>
        )}

        {list.isPending ? (
          <Skeleton className='h-64' />
        ) : rows.length === 0 ? (
          <EmptyState
            title='Belum ada riwayat'
            description='Booking yang sudah selesai, ditolak, atau dibatalkan tampil di sini.'
          />
        ) : (
          <>
            <ResponsiveDataList
              caption='Riwayat booking'
              rows={rows}
              columns={columns}
              rowKey={(b) => b.id}
              onRowClick={(b) => setDetailId(b.id)}
            />
            <nav
              aria-label='Halaman'
              className='flex items-center justify-end gap-3'
            >
              <p className='type-body-sm text-fg-secondary'>
                {from}–{to} dari {meta?.total ?? 0}
              </p>
              <Button
                variant='secondary'
                size='icon'
                aria-label='Halaman sebelumnya'
                disabled={page <= 1}
                onClick={() => go({ page: page - 1 })}
              >
                <ChevronLeft />
              </Button>
              <Button
                variant='secondary'
                size='icon'
                aria-label='Halaman berikutnya'
                disabled={page >= pages}
                onClick={() => go({ page: page + 1 })}
              >
                <ChevronRight />
              </Button>
            </nav>
          </>
        )}
      </section>
      <BookingDetailDialog
        id={detailId}
        onOpenChange={(o) => !o && setDetailId(null)}
      />
    </Page>
  )
}
