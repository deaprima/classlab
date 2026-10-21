import { useState } from 'react'
import { Link, getRouteApi } from '@tanstack/react-router'
import { AlertCircle, ArrowLeft, Info } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage, isApiErrorCode } from '@/lib/api-error'
import { type Booking } from '@/lib/api-types'
import { formatDate, formatTimeRange } from '@/lib/datetime'
import { cn } from '@/lib/utils'
import { useNow } from '@/hooks/use-now'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Page } from '@/components/layout/page'
import { BookingStatusBadge } from '@/components/status-badge'
import {
  useBooking,
  useBookingEvents,
  useBookings,
} from '@/features/bookings/api'
import { CancelDialog } from './cancel-dialog'
import { EditDialog } from './edit-dialog'
import { dayShort } from './format'
import { BookingFacts, StatusTimeline } from './parts'
import { canCancel, canEdit, statusNote } from './rules'
import { useApprover } from './use-approver'

const route = getRouteApi('/_authenticated/bookings/$id')

/** Detail Pengajuan (Figma): status and steps on the left, the facts and the actions on the right. */
export function BookingDetailPage() {
  const { id } = route.useParams()
  const booking = useBooking(id)
  const events = useBookingEvents(id)
  const user = useAuthStore((state) => state.auth.user)
  const now = useNow()
  const [editing, setEditing] = useState<Booking | null>(null)
  const [cancelling, setCancelling] = useState<Booking | null>(null)
  const b = booking.data
  const approver = useApprover(b?.room.type ?? 'KELAS')

  // The other dates of a multi-date request (BR-27), including this one.
  const group = useBookings(
    {
      mine: true,
      group_id: b?.group_id ?? undefined,
      per_page: 100,
    },
    !!b?.group_id && b.occurrence_count > 1
  )

  if (booking.isError) {
    return (
      <Page title='Detail Pengajuan'>
        <Alert variant='error'>
          <AlertCircle />
          <AlertDescription>
            {isApiErrorCode(booking.error, 'NOT_FOUND')
              ? 'Pengajuan tidak ditemukan.'
              : getErrorMessage(booking.error)}
          </AlertDescription>
        </Alert>
        <Button variant='secondary' className='mt-4' asChild>
          <Link to='/bookings'>
            <ArrowLeft />
            Kembali ke pengajuan
          </Link>
        </Button>
      </Page>
    )
  }
  if (!b) {
    return (
      <Page title='Detail Pengajuan'>
        <Skeleton className='h-96 rounded-2xl' />
      </Page>
    )
  }

  const owner = !!user && b.requester_id === user.id
  const note = statusNote(b, now, approver)
  const dates = (group.data?.data ?? []).sort(
    (x, y) => Date.parse(x.start_at) - Date.parse(y.start_at)
  )

  return (
    <Page title='Detail Pengajuan'>
      <div className='grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_340px]'>
        <section
          aria-label='Status pengajuan'
          className='flex min-w-0 flex-col gap-5 rounded-2xl border bg-card p-5'
        >
          <h2 className='type-section text-fg'>Status Pengajuan</h2>
          <div className='flex items-start justify-between gap-3'>
            <div className='min-w-0'>
              <p className='type-page-title text-fg sm:type-stat-l'>
                {b.room.code}
              </p>
              <p className='type-body text-fg-secondary'>
                {b.purpose} · {formatDate(b.start_at)} ·{' '}
                {formatTimeRange(b.start_at, b.end_at)}
              </p>
            </div>
            <BookingStatusBadge status={b.status} />
          </div>

          <div
            role='status'
            className={cn(
              'flex gap-3 rounded-xl border p-4',
              note.tone === 'error'
                ? 'border-line-error bg-error-subtle text-fg-error'
                : 'bg-subtle text-fg'
            )}
          >
            <Info aria-hidden className='mt-0.5 size-5 shrink-0' />
            <div>
              <p className='type-card-title'>{note.title}</p>
              <p className='type-body-sm opacity-80'>{note.description}</p>
            </div>
          </div>

          <StatusTimeline booking={b} events={events.data ?? []} />

          {dates.length > 1 && (
            <div className='flex flex-col gap-3'>
              <p className='type-overline text-fg-secondary'>
                Kejadian ({dates.length} tanggal)
              </p>
              <ul className='flex flex-col gap-3'>
                {dates.map((d) => (
                  <li
                    key={d.id}
                    className='flex flex-wrap items-center justify-between gap-3 rounded-xl border p-4'
                  >
                    <div className='min-w-0'>
                      <p className='type-card-title text-fg'>
                        {dayShort(d.start_at)}
                      </p>
                      <p className='type-body-sm text-fg-secondary'>
                        {formatTimeRange(d.start_at, d.end_at)}
                      </p>
                    </div>
                    <div className='flex items-center gap-2'>
                      <BookingStatusBadge status={d.status} />
                      {owner && canEdit(d, now) && (
                        <Button
                          size='sm'
                          variant='ghost'
                          aria-label={`Ubah ${dayShort(d.start_at)}`}
                          onClick={() => setEditing(d)}
                        >
                          Ubah
                        </Button>
                      )}
                      {owner && canCancel(d, now) && (
                        <Button
                          size='sm'
                          variant='ghost'
                          aria-label={`Batalkan ${dayShort(d.start_at)}`}
                          onClick={() => setCancelling(d)}
                        >
                          Batalkan
                        </Button>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
              <p className='type-body-sm text-fg-secondary'>
                Approve atau tolak berlaku untuk seluruh grup. Check-in dan
                check-out per kejadian.
              </p>
            </div>
          )}
        </section>

        <aside
          aria-label='Ringkasan'
          className='flex h-fit flex-col gap-4 rounded-2xl border bg-card p-5'
        >
          <h2 className='type-section text-fg'>Ringkasan</h2>
          <BookingFacts booking={b} />
          {owner && (canEdit(b, now) || canCancel(b, now)) && (
            <div className='flex flex-col gap-2'>
              {canEdit(b, now) && (
                <Button onClick={() => setEditing(b)}>Ubah Pengajuan</Button>
              )}
              {canCancel(b, now) && (
                <Button variant='secondary' onClick={() => setCancelling(b)}>
                  Batalkan Pengajuan
                </Button>
              )}
            </div>
          )}
          <Button variant='ghost' asChild>
            <Link to='/bookings'>
              <ArrowLeft />
              Pengajuan Saya
            </Link>
          </Button>
        </aside>
      </div>

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
