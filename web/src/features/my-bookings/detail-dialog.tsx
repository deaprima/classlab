import { AlertCircle, Info, X } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage, isApiErrorCode } from '@/lib/api-error'
import { type Booking } from '@/lib/api-types'
import { formatTimeRange } from '@/lib/datetime'
import { cn } from '@/lib/utils'
import { useNow } from '@/hooks/use-now'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogClose,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { BookingStatusBadge } from '@/components/status-badge'
import {
  useBooking,
  useBookingEvents,
  useBookings,
} from '@/features/bookings/api'
import { dayShort } from './format'
import { BookingFacts, StatusTimeline } from './parts'
import { canCancel, canEdit, statusNote } from './rules'
import { useApprover } from './use-approver'

type DetailDialogProps = {
  /** The booking to show; null keeps the dialog closed. */
  id: string | null
  onOpenChange: (open: boolean) => void
  /** Without these the dialog only shows the booking (the history). */
  onEdit?: (booking: Booking) => void
  onCancel?: (booking: Booking) => void
}

/** "Detail Pengajuan": everything about one booking in a popup, with its steps and every date of a group. */
export function BookingDetailDialog({
  id,
  onOpenChange,
  onEdit,
  onCancel,
}: DetailDialogProps) {
  return (
    <Dialog open={!!id} onOpenChange={onOpenChange}>
      <DialogContent
        showCloseButton={false}
        className='flex max-h-[90dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-lg'
      >
        {id && (
          <DetailBody
            key={id}
            id={id}
            onClose={() => onOpenChange(false)}
            onEdit={onEdit}
            onCancel={onCancel}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

function DetailBody({
  id,
  onClose,
  onEdit,
  onCancel,
}: {
  id: string
  onClose: () => void
  onEdit?: (booking: Booking) => void
  onCancel?: (booking: Booking) => void
}) {
  const booking = useBooking(id)
  const events = useBookingEvents(id)
  const user = useAuthStore((state) => state.auth.user)
  const now = useNow()
  const b = booking.data
  const approver = useApprover(b?.room.type ?? 'KELAS')

  // The other dates of a multi-date request (BR-27), including this one.
  const group = useBookings(
    { mine: true, group_id: b?.group_id ?? undefined, per_page: 100 },
    !!b?.group_id && b.occurrence_count > 1
  )

  // The dialog that opens next takes over, so this one closes first.
  const change = (fn: ((d: Booking) => void) | undefined, d: Booking) => {
    onClose()
    fn?.(d)
  }

  if (booking.isError) {
    return (
      <>
        <Head description='Detail pengajuan' />
        <div className='p-6'>
          <Alert variant='error'>
            <AlertCircle />
            <AlertDescription>
              {isApiErrorCode(booking.error, 'NOT_FOUND')
                ? 'Pengajuan tidak ditemukan.'
                : getErrorMessage(booking.error)}
            </AlertDescription>
          </Alert>
        </div>
      </>
    )
  }
  if (!b) {
    return (
      <>
        <Head description='Memuat detail pengajuan' />
        <div className='p-6'>
          <Skeleton className='h-72' />
        </div>
      </>
    )
  }

  const owner = !!user && b.requester_id === user.id
  const note = statusNote(b, now, approver)
  const dates = [...(group.data?.data ?? [])].sort(
    (x, y) => Date.parse(x.start_at) - Date.parse(y.start_at)
  )
  const mayEdit = owner && !!onEdit && canEdit(b, now)
  const mayCancel = owner && !!onCancel && canCancel(b, now)

  return (
    <>
      <Head description={`${b.room.code}, ${b.purpose}`} />

      <div className='flex flex-col gap-5 overflow-y-auto p-6'>
        <div className='flex items-start justify-between gap-3'>
          <div className='min-w-0'>
            <p className='type-section text-fg'>{b.room.code}</p>
            <p className='type-body-sm text-fg-secondary'>{b.purpose}</p>
          </div>
          <BookingStatusBadge status={b.status} />
        </div>

        <div
          role='status'
          className={cn(
            'flex gap-3 rounded-xl border p-3',
            note.tone === 'error'
              ? 'border-line-error bg-error-subtle text-fg-error'
              : 'bg-subtle text-fg'
          )}
        >
          <Info aria-hidden className='mt-0.5 size-4 shrink-0' />
          <div>
            <p className='type-card-title'>{note.title}</p>
            <p className='type-body-sm opacity-80'>{note.description}</p>
          </div>
        </div>

        <BookingFacts booking={b} />

        <div className='border-t pt-5'>
          <p className='mb-3 type-overline text-fg-secondary'>Status</p>
          <StatusTimeline booking={b} events={events.data ?? []} stackMeta />
        </div>

        {dates.length > 1 && (
          <div className='flex flex-col gap-3 border-t pt-5'>
            <p className='type-overline text-fg-secondary'>
              Kejadian ({dates.length} tanggal)
            </p>
            <ul className='flex flex-col gap-2'>
              {dates.map((d) => (
                <li
                  key={d.id}
                  className='flex flex-wrap items-center justify-between gap-2 rounded-xl border p-3'
                >
                  <div className='min-w-0'>
                    <p className='type-card-title text-fg'>
                      {dayShort(d.start_at)}
                    </p>
                    <p className='type-body-sm text-fg-secondary'>
                      {formatTimeRange(d.start_at, d.end_at)}
                    </p>
                  </div>
                  <div className='flex items-center gap-1'>
                    <BookingStatusBadge status={d.status} />
                    {owner && onEdit && canEdit(d, now) && (
                      <Button
                        size='sm'
                        variant='ghost'
                        aria-label={`Ubah ${dayShort(d.start_at)}`}
                        onClick={() => change(onEdit, d)}
                      >
                        Ubah
                      </Button>
                    )}
                    {owner && onCancel && canCancel(d, now) && (
                      <Button
                        size='sm'
                        variant='ghost'
                        aria-label={`Batalkan ${dayShort(d.start_at)}`}
                        onClick={() => change(onCancel, d)}
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
      </div>

      <DialogFooter className='border-t px-6 py-4'>
        <Button type='button' variant='secondary' onClick={onClose}>
          Tutup
        </Button>
        {mayCancel && (
          <Button
            type='button'
            variant='secondary'
            onClick={() => change(onCancel, b)}
          >
            Batalkan Pengajuan
          </Button>
        )}
        {mayEdit && (
          <Button type='button' onClick={() => change(onEdit, b)}>
            Ubah Pengajuan
          </Button>
        )}
      </DialogFooter>
    </>
  )
}

function Head({ description }: { description: string }) {
  return (
    <DialogHeader className='flex-row items-center justify-between gap-3 border-b px-6 py-4 text-start'>
      <DialogTitle className='type-section'>Detail Pengajuan</DialogTitle>
      <DialogDescription className='sr-only'>{description}</DialogDescription>
      <DialogClose asChild>
        <Button
          type='button'
          variant='ghost'
          size='icon'
          className='-me-2 size-8 shrink-0'
          aria-label='Tutup'
        >
          <X />
        </Button>
      </DialogClose>
    </DialogHeader>
  )
}
