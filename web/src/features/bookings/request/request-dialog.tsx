import { toast } from 'sonner'
import {
  type Booking,
  type BookingGroupResult,
  type Room,
} from '@/lib/api-types'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { roomTypeConfig } from '@/features/room-types/config'
import { createdMessage } from './created-message'
import { RequestForm } from './request-form'
import { type OccurrenceValues } from './request-schema'

type RequestDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  room: Room
  /** The slots picked on the schedule. */
  occurrences: readonly OccurrenceValues[]
  onCreated: (result: Booking | BookingGroupResult) => void
}

/** "Ajukan Ruang" as a popup over the schedule: a full-screen sheet on mobile. */
export function RequestDialog({
  open,
  onOpenChange,
  room,
  occurrences,
  onCreated,
}: RequestDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className='flex max-h-[90svh] flex-col gap-0 overflow-hidden p-0 max-sm:top-0 max-sm:left-0 max-sm:h-svh max-sm:max-h-svh max-sm:max-w-none max-sm:translate-x-0 max-sm:translate-y-0 max-sm:rounded-none sm:max-w-2xl'
        onInteractOutside={(e) => e.preventDefault()}
        // Not the date field: on a phone the keyboard would open before anything is read.
        onOpenAutoFocus={(e) => e.preventDefault()}
      >
        <DialogHeader className='px-6 pt-6 pb-4 text-start'>
          <DialogTitle>Ajukan Ruang</DialogTitle>
          <DialogDescription>
            {room.code} · {roomTypeConfig(room.type).label} · {room.capacity}{' '}
            orang
          </DialogDescription>
        </DialogHeader>
        {/* Mounted only while open, so every opening starts from the slots picked now. */}
        {open && (
          <RequestForm
            room={room}
            initialOccurrences={occurrences}
            onCancel={() => onOpenChange(false)}
            onCreated={(result) => {
              toast.success(createdMessage(result))
              onOpenChange(false)
              onCreated(result)
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}
