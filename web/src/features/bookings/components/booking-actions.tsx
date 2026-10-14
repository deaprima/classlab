import { useState } from 'react'
import { Check, Loader2, X } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { type Booking } from '@/lib/api-types'
import { formatSlot } from '@/lib/datetime'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useApproveBooking, useCancelBooking, useRejectBooking } from '../api'
import { availableActions } from '../transitions'
import { ReasonDialog } from './reason-dialog'

type Dialog = 'approve' | 'reject' | 'cancel' | null

export function BookingActions({
  booking,
  onDone,
  size = 'sm',
}: {
  booking: Booking
  onDone?: () => void
  size?: 'sm' | 'default'
}) {
  const user = useAuthStore((s) => s.auth.user)
  const [dialog, setDialog] = useState<Dialog>(null)
  // Fixed at mount. A booking that ends while the page is open is still refused by the server.
  const [now] = useState(() => Date.now())
  const actions = availableActions(booking, user, now)
  const approve = useApproveBooking()
  const reject = useRejectBooking()
  const cancel = useCancelBooking()

  const close = () => setDialog(null)
  const done = () => {
    close()
    onDone?.()
  }
  const slot = `${booking.room.code}, ${formatSlot(booking.start_at, booking.end_at)}`

  if (!Object.values(actions).some(Boolean)) return null

  return (
    <div className='flex flex-wrap gap-2'>
      {actions.approve && (
        <Button size={size} onClick={() => setDialog('approve')}>
          <Check />
          Setujui
        </Button>
      )}
      {actions.reject && (
        <Button
          size={size}
          variant='outline'
          onClick={() => setDialog('reject')}
        >
          <X />
          Tolak
        </Button>
      )}
      {(actions.cancelOwn || actions.cancelApproved) && (
        <Button
          size={size}
          variant='outline'
          onClick={() => setDialog('cancel')}
        >
          <X />
          {actions.cancelOwn ? 'Batalkan pengajuan' : 'Batalkan peminjaman'}
        </Button>
      )}

      <ConfirmDialog
        open={dialog === 'approve'}
        onOpenChange={(o) => !o && close()}
        title='Setujui pengajuan?'
        desc={
          <p>
            {booking.requester.name} ({booking.requester.identifier}) meminjam{' '}
            <strong>{slot}</strong> untuk “{booking.purpose}”. Slot akan
            ditandai digunakan dan peminjam diberi notifikasi.
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
          approve.mutate(booking.id, { onSuccess: done, onError: close })
        }
      />

      <ReasonDialog
        open={dialog === 'reject'}
        onOpenChange={(o) => !o && close()}
        title='Tolak pengajuan'
        description={
          <p>
            {booking.requester.name} · {slot}. Alasan dikirim ke peminjam.
          </p>
        }
        confirmText='Tolak'
        required
        isPending={reject.isPending}
        onSubmit={(reason) =>
          reject.mutate({ id: booking.id, reason }, { onSuccess: done })
        }
      />

      <ReasonDialog
        open={dialog === 'cancel'}
        onOpenChange={(o) => !o && close()}
        title={actions.cancelOwn ? 'Batalkan pengajuan' : 'Batalkan peminjaman'}
        description={
          actions.cancelOwn ? (
            <p>{slot}. Slot akan dibuka kembali untuk pengguna lain.</p>
          ) : (
            <p>
              Peminjaman {booking.requester.name} pada {slot} sudah disetujui.
              Peminjam akan diberi tahu beserta alasannya.
            </p>
          )
        }
        confirmText='Batalkan'
        required={!actions.cancelOwn}
        isPending={cancel.isPending}
        onSubmit={(reason) =>
          cancel.mutate(
            { id: booking.id, reason: reason || undefined },
            { onSuccess: done }
          )
        }
      />
    </div>
  )
}
