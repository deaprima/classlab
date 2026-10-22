import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { type Booking } from '@/lib/api-types'
import { formatTimeRange } from '@/lib/datetime'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useApproveBooking, useRejectBooking } from '@/features/bookings/api'
import { ReasonDialog } from '@/features/bookings/components/reason-dialog'
import { dayShort } from '@/features/my-bookings/format'

type DecisionButtonsProps = {
  booking: Booking
  /** Dates of the request when it spans several; approving or rejecting decides all of them. */
  dateCount?: number
  /** The slot is over: approving is no longer possible, rejecting still clears the queue. */
  ended?: boolean
  /** Called after a decision went through. */
  onDecided?: () => void
  /** `sm` for a row of a list, `md` for the detail panel. */
  size?: 'sm' | 'md'
  className?: string
}

/** Tolak and Setujui of a pending request, with the confirmation and the reason dialog they need. */
export function DecisionButtons({
  booking: b,
  dateCount = 1,
  ended = false,
  onDecided,
  size = 'md',
  className,
}: DecisionButtonsProps) {
  const [dialog, setDialog] = useState<'approve' | 'reject' | null>(null)
  const approve = useApproveBooking()
  const reject = useRejectBooking()
  const slot = `${b.room.code}, ${dayShort(b.start_at)} ${formatTimeRange(b.start_at, b.end_at)}`
  const done = () => {
    setDialog(null)
    onDecided?.()
  }
  const sizing = size === 'sm' ? 'h-8 px-4' : 'h-10 flex-1'

  return (
    <>
      <div className={cn('flex gap-2', className)}>
        <Button
          variant='secondary'
          size={size === 'sm' ? 'sm' : 'default'}
          className={sizing}
          onClick={() => setDialog('reject')}
        >
          Tolak
          <span className='sr-only'> pengajuan {slot}</span>
        </Button>
        <Button
          size={size === 'sm' ? 'sm' : 'default'}
          className={sizing}
          disabled={ended}
          onClick={() => setDialog('approve')}
        >
          Setujui
          <span className='sr-only'> pengajuan {slot}</span>
        </Button>
      </div>

      <ConfirmDialog
        open={dialog === 'approve'}
        onOpenChange={(o) => !o && setDialog(null)}
        title='Setujui pengajuan?'
        desc={
          <p>
            {b.requester.name} meminjam <strong>{slot}</strong>
            {dateCount > 1 ? ` dan ${dateCount - 1} tanggal lainnya` : ''} untuk
            “{b.purpose}”. Peminjam diberi notifikasi.
          </p>
        }
        confirmText={
          approve.isPending ? (
            <>
              <Loader2 className='animate-spin' />
              Menyetujui…
            </>
          ) : (
            'Setujui'
          )
        }
        isLoading={approve.isPending}
        handleConfirm={() =>
          approve.mutate(b.id, {
            onSuccess: done,
            onError: () => setDialog(null),
          })
        }
      />
      <ReasonDialog
        open={dialog === 'reject'}
        onOpenChange={(o) => !o && setDialog(null)}
        title='Tolak pengajuan?'
        description={
          <p>Pemohon menerima notifikasi beserta alasan penolakan.</p>
        }
        label='Alasan penolakan'
        placeholder='Minimal 5 karakter. Contoh: Bentrok dengan kegiatan jurusan.'
        confirmText='Tolak'
        required
        isPending={reject.isPending}
        onSubmit={(reason) =>
          reject.mutate({ id: b.id, reason }, { onSuccess: done })
        }
      />
    </>
  )
}
