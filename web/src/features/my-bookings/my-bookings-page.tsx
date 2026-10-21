import { useMemo, useState } from 'react'
import { Link, getRouteApi } from '@tanstack/react-router'
import { AlertCircle, CalendarDays } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type Booking, type BookingStatus } from '@/lib/api-types'
import { useMediaQuery } from '@/hooks/use-media-query'
import { useNow } from '@/hooks/use-now'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/layout/page'
import {
  ResponsiveDataList,
  type DataColumn,
} from '@/components/responsive-data-list'
import { SegmentedControl } from '@/components/segmented-control'
import { BookingStatusBadge } from '@/components/status-badge'
import {
  useBooking,
  useBookingEvents,
  useBookings,
} from '@/features/bookings/api'
import { CancelDialog } from './cancel-dialog'
import { EditDialog } from './edit-dialog'
import { whenShort } from './format'
import { BookingFacts, RowMenu, StatusTimeline } from './parts'
import { canCancel, canEdit } from './rules'

const route = getRouteApi('/_authenticated/bookings/')

/** What belongs on "Pengajuan Saya": requests that are waiting, approved, running or refused. */
const LISTED: readonly BookingStatus[] = [
  'MENUNGGU',
  'DISETUJUI',
  'BERLANGSUNG',
  'DITOLAK',
]

type Tab = 'ALL' | 'MENUNGGU' | 'DISETUJUI' | 'DITOLAK'

function tabOf(status: string | undefined): Tab {
  return status === 'MENUNGGU' || status === 'DISETUJUI' || status === 'DITOLAK'
    ? status
    : 'ALL'
}

function inTab(b: Booking, tab: Tab): boolean {
  if (tab === 'ALL') return true
  if (tab === 'DISETUJUI')
    return b.status === 'DISETUJUI' || b.status === 'BERLANGSUNG'
  return b.status === tab
}

/** Pengajuan Saya (Figma): the list of current requests with the selected one on the right. */
export function MyBookings() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const now = useNow()
  const wide = useMediaQuery('(min-width: 1024px)')
  // Bookings that have not ended; fixed per visit so the query key stays stable.
  const from = useMemo(() => new Date().toISOString(), [])
  const bookings = useBookings({ mine: true, from, per_page: 100 })
  const [editing, setEditing] = useState<Booking | null>(null)
  const [cancelling, setCancelling] = useState<Booking | null>(null)

  const tab = tabOf(search.status)
  const all = (bookings.data?.data ?? [])
    .filter((b) => LISTED.includes(b.status))
    .sort((a, b) => Date.parse(a.start_at) - Date.parse(b.start_at))
  const shown = all.filter((b) => inTab(b, tab))
  const count = (t: Tab) => all.filter((b) => inTab(b, t)).length
  const selectedId = search.booking ?? shown[0]?.id

  const select = (booking: Booking) => {
    // Beside the list there is a detail panel; on a narrow screen the row opens the full page.
    if (wide) {
      navigate({ search: (prev) => ({ ...prev, booking: booking.id }) })
    } else {
      navigate({ to: '/bookings/$id', params: { id: booking.id } })
    }
  }

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
          withChanges
          onEdit={setEditing}
          onCancel={setCancelling}
        />
      ),
    },
  ]

  return (
    <Page title='Pengajuan Saya'>
      {bookings.isError && (
        <Alert variant='error' className='mb-5'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(bookings.error)}</AlertDescription>
        </Alert>
      )}

      {bookings.isPending ? (
        <Skeleton className='h-96 rounded-2xl' />
      ) : all.length === 0 ? (
        <section className='rounded-2xl border bg-card p-5'>
          <EmptyState
            icon={CalendarDays}
            title='Belum ada pengajuan'
            description='Pengajuan ruang yang kamu buat akan muncul di sini.'
            className='py-24'
            action={
              <Button asChild>
                <Link to='/schedule'>Ajukan Ruang</Link>
              </Button>
            }
          />
        </section>
      ) : (
        <div className='grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_340px]'>
          <section
            aria-label='Daftar pengajuan'
            className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
          >
            <h2 className='type-section text-fg'>Daftar Pengajuan</h2>
            <SegmentedControl
              aria-label='Filter status'
              value={tab}
              onValueChange={(t) =>
                navigate({
                  search: (prev) => ({
                    ...prev,
                    status: t === 'ALL' ? undefined : t,
                  }),
                })
              }
              options={[
                { value: 'ALL', label: 'Semua', count: count('ALL') },
                {
                  value: 'MENUNGGU',
                  label: 'Menunggu',
                  count: count('MENUNGGU'),
                },
                {
                  value: 'DISETUJUI',
                  label: 'Disetujui',
                  count: count('DISETUJUI'),
                },
                { value: 'DITOLAK', label: 'Ditolak', count: count('DITOLAK') },
              ]}
            />
            {shown.length === 0 ? (
              <p className='type-body text-fg-secondary'>
                Tidak ada pengajuan dengan status ini.
              </p>
            ) : (
              <ResponsiveDataList
                caption='Daftar pengajuan'
                rows={shown}
                columns={columns}
                rowKey={(b) => b.id}
                onRowClick={select}
                isRowActive={wide ? (b) => b.id === selectedId : undefined}
              />
            )}
          </section>

          {wide && selectedId && (
            <SidePanel
              key={selectedId}
              id={selectedId}
              now={now}
              onEdit={setEditing}
              onCancel={setCancelling}
            />
          )}
        </div>
      )}

      <EditDialog
        booking={editing}
        onOpenChange={(o) => !o && setEditing(null)}
      />
      <CancelDialog
        booking={cancelling}
        onOpenChange={(o) => !o && setCancelling(null)}
      />
    </Page>
  )
}

function SidePanel({
  id,
  now,
  onEdit,
  onCancel,
}: {
  id: string
  now: Date
  onEdit: (b: Booking) => void
  onCancel: (b: Booking) => void
}) {
  const booking = useBooking(id)
  const events = useBookingEvents(id)
  const b = booking.data

  return (
    <aside
      aria-label='Detail pengajuan'
      className='flex h-fit min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
    >
      <h2 className='type-section text-fg'>Detail Pengajuan</h2>
      {booking.isError && (
        <p className='type-body text-fg-error'>
          {getErrorMessage(booking.error)}
        </p>
      )}
      {booking.isPending && <Skeleton className='h-48' />}
      {b && (
        <>
          <div className='flex items-start justify-between gap-3'>
            <div className='min-w-0'>
              <p className='type-section text-fg'>{b.room.code}</p>
              <p className='type-body text-fg-secondary'>{b.purpose}</p>
            </div>
            <BookingStatusBadge status={b.status} />
          </div>
          <BookingFacts booking={b} />
          <div className='border-t pt-4'>
            <p className='mb-3 type-overline text-fg-secondary'>Status</p>
            <StatusTimeline booking={b} events={events.data ?? []} />
          </div>
          <div className='flex flex-col gap-2'>
            {canEdit(b, now) && (
              <Button variant='secondary' onClick={() => onEdit(b)}>
                Ubah Pengajuan
              </Button>
            )}
            {canCancel(b, now) && (
              <Button variant='secondary' onClick={() => onCancel(b)}>
                Batalkan Pengajuan
              </Button>
            )}
            <Button variant='ghost' asChild>
              <Link to='/bookings/$id' params={{ id: b.id }}>
                Lihat detail
              </Link>
            </Button>
          </div>
        </>
      )}
    </aside>
  )
}
