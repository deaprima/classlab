import { Link } from '@tanstack/react-router'
import { AlertCircle, CalendarDays } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage, isApiErrorCode } from '@/lib/api-error'
import { type Booking, type BookingEvent } from '@/lib/api-types'
import { formatDateTime, formatSlot, toWibDate } from '@/lib/datetime'
import { BOOKING_STATUS_LABELS } from '@/lib/status-labels'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { BookingStatusBadge } from '@/components/status-badge'
import { ROOM_TYPE_LABELS } from '@/features/rooms/labels'
import { useBooking, useBookingEvents } from '../api'
import { BookingActions } from './booking-actions'

/** Booking detail with its append-only status trail (BR-06). Opened from lists by `?booking=<id>`. */
export function BookingDetailSheet({
  bookingId,
  onClose,
}: {
  bookingId: string | undefined
  onClose: () => void
}) {
  const booking = useBooking(bookingId)

  return (
    <Sheet open={!!bookingId} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className='w-full gap-0 overflow-y-auto sm:max-w-lg'>
        <SheetHeader>
          <SheetTitle>Detail peminjaman</SheetTitle>
          <SheetDescription>
            {booking.data
              ? `${booking.data.room.code} · ${formatSlot(booking.data.start_at, booking.data.end_at)}`
              : 'Memuat…'}
          </SheetDescription>
        </SheetHeader>

        <div className='px-4 pb-4'>
          {booking.isError && (
            <Alert variant='destructive'>
              <AlertCircle />
              <AlertDescription>
                {isApiErrorCode(booking.error, 'NOT_FOUND')
                  ? 'Peminjaman tidak ditemukan.'
                  : getErrorMessage(booking.error)}
              </AlertDescription>
            </Alert>
          )}
          {booking.isPending && bookingId && <Skeleton className='h-64' />}
          {booking.data && <BookingInfo booking={booking.data} />}
        </div>

        {booking.data && (
          <SheetFooter className='border-t'>
            <BookingActions booking={booking.data} size='default' />
          </SheetFooter>
        )}
      </SheetContent>
    </Sheet>
  )
}

function BookingInfo({ booking }: { booking: Booking }) {
  return (
    <div className='space-y-6'>
      <dl className='grid grid-cols-[8rem_1fr] gap-x-4 gap-y-2 text-sm'>
        <dt className='text-muted-foreground'>Status</dt>
        <dd>
          <BookingStatusBadge status={booking.status} />
        </dd>
        <dt className='text-muted-foreground'>Ruang</dt>
        <dd>
          {booking.room.code} · {booking.room.name} (
          {ROOM_TYPE_LABELS[booking.room.type]}, {booking.room.capacity} orang)
        </dd>
        <dt className='text-muted-foreground'>Waktu</dt>
        <dd>{formatSlot(booking.start_at, booking.end_at)} WIB</dd>
        <dt className='text-muted-foreground'>Peminjam</dt>
        <dd>
          {booking.requester.name} ({booking.requester.identifier})
        </dd>
        <dt className='text-muted-foreground'>Keperluan</dt>
        <dd className='break-words'>{booking.purpose}</dd>
        {booking.participants !== undefined && (
          <>
            <dt className='text-muted-foreground'>Peserta</dt>
            <dd>{booking.participants} orang</dd>
          </>
        )}
        {booking.notes && (
          <>
            <dt className='text-muted-foreground'>Catatan</dt>
            <dd className='break-words whitespace-pre-line'>{booking.notes}</dd>
          </>
        )}
        <dt className='text-muted-foreground'>Diajukan</dt>
        <dd>{formatDateTime(booking.created_at)}</dd>
        {booking.decision_reason && (
          <>
            <dt className='text-muted-foreground'>Alasan</dt>
            <dd className='break-words'>{booking.decision_reason}</dd>
          </>
        )}
      </dl>

      <Button variant='outline' size='sm' asChild>
        <Link
          to='/rooms/$id'
          params={{ id: booking.room_id }}
          search={{ date: toWibDate(booking.start_at) }}
        >
          <CalendarDays />
          Lihat jadwal ruang
        </Link>
      </Button>

      <EventTrail booking={booking} />
    </div>
  )
}

function EventTrail({ booking }: { booking: Booking }) {
  const events = useBookingEvents(booking.id)
  const me = useAuthStore((s) => s.auth.user?.id)

  const actorName = (e: BookingEvent) => {
    if (e.actor_id === me) return 'Anda'
    if (e.actor_id === booking.requester_id) return booking.requester.name
    return 'TU/Pengurus Lab'
  }

  return (
    <section>
      <h3 className='mb-3 text-sm font-semibold'>Riwayat status</h3>
      {events.isError && (
        <p className='text-sm text-destructive'>
          {getErrorMessage(events.error)}
        </p>
      )}
      {events.isPending && <Skeleton className='h-20' />}
      {events.data && (
        <ol className='relative space-y-4 border-s ps-4'>
          {events.data.map((e) => (
            <li key={e.id} className='text-sm'>
              <span
                className='absolute -start-1.5 mt-1.5 size-3 rounded-full border bg-background'
                aria-hidden
              />
              <p className='font-medium'>
                {e.from_status
                  ? `${BOOKING_STATUS_LABELS[e.from_status]} → ${BOOKING_STATUS_LABELS[e.to_status]}`
                  : `Diajukan (${BOOKING_STATUS_LABELS[e.to_status]})`}
              </p>
              <p className='text-muted-foreground'>
                {formatDateTime(e.created_at)} · {actorName(e)}
              </p>
              {e.reason && <p className='mt-1 break-words'>“{e.reason}”</p>}
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}
