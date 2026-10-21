import { useState } from 'react'
import { bookingChangeRules } from '@/config/attendance-rules'
import { Loader2, TriangleAlert } from 'lucide-react'
import { type Booking } from '@/lib/api-types'
import { formatDateShort } from '@/lib/datetime'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { useCancelBooking, type CancelScope } from '@/features/bookings/api'
import { isLateCancel } from './rules'

type CancelDialogProps = {
  /** The booking to cancel; null keeps the dialog closed. */
  booking: Booking | null
  onOpenChange: (open: boolean) => void
}

/** "Batalkan pengajuan?" (Figma): this date only, or the whole multi-date request (BR-27). */
export function CancelDialog({ booking, onOpenChange }: CancelDialogProps) {
  return (
    <Dialog open={!!booking} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        {booking && (
          <CancelForm
            key={booking.id}
            booking={booking}
            onClose={() => onOpenChange(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

function CancelForm({
  booking,
  onClose,
}: {
  booking: Booking
  onClose: () => void
}) {
  const [scope, setScope] = useState<CancelScope>('OCCURRENCE')
  const cancel = useCancelBooking()
  const inGroup = !!booking.group_id && booking.occurrence_count > 1
  // The point is charged for the date being cancelled; a whole group may include later dates too.
  const late = isLateCancel(booking, new Date())

  return (
    <>
      <DialogHeader>
        <DialogTitle>Batalkan pengajuan?</DialogTitle>
        <DialogDescription>
          {inGroup
            ? 'Pilih cakupan pembatalan.'
            : `${booking.room.code} · ${formatDateShort(booking.start_at)}. Slot dibuka kembali untuk pengguna lain.`}
        </DialogDescription>
      </DialogHeader>

      {inGroup && (
        <RadioGroup
          value={scope}
          onValueChange={(v) => setScope(v as CancelScope)}
          aria-label='Cakupan pembatalan'
          className='gap-3'
        >
          <div className='flex items-center gap-3'>
            <RadioGroupItem value='OCCURRENCE' id='cancel-one' />
            <Label htmlFor='cancel-one' className='font-normal'>
              Kejadian ini saja (
              {formatDateShort(booking.start_at).replace(/ \d{4}$/, '')})
            </Label>
          </div>
          <div className='flex items-center gap-3'>
            <RadioGroupItem value='GROUP' id='cancel-group' />
            <Label htmlFor='cancel-group' className='font-normal'>
              Seluruh grup ({booking.occurrence_count} tanggal)
            </Label>
          </div>
        </RadioGroup>
      )}

      {late && (
        <Alert variant='warning'>
          <TriangleAlert />
          <AlertTitle>Batal terlambat kena poin</AlertTitle>
          <AlertDescription>
            Pembatalan kurang dari {bookingChangeRules.lateCancelHours} jam
            sebelum mulai menambah 1 poin pelanggaran.
          </AlertDescription>
        </Alert>
      )}

      <DialogFooter>
        <Button type='button' variant='secondary' onClick={onClose}>
          Kembali
        </Button>
        <Button
          type='button'
          disabled={cancel.isPending}
          onClick={() =>
            cancel.mutate(
              { id: booking.id, ...(inGroup && { scope }) },
              { onSuccess: onClose }
            )
          }
        >
          {cancel.isPending && <Loader2 className='animate-spin' />}
          Batalkan
        </Button>
      </DialogFooter>
    </>
  )
}
