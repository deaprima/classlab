import { Ban, Eye, Pencil } from 'lucide-react'
import { type Booking, type BookingEvent } from '@/lib/api-types'
import { formatTimeRange } from '@/lib/datetime'
import { Button } from '@/components/ui/button'
import { Step } from '@/components/step'
import { ROOM_TYPE_LABELS } from '@/features/rooms/labels'
import { dayShort } from './format'
import { canCancel, canEdit } from './rules'
import { buildTimeline } from './timeline'
import { useApprover } from './use-approver'

/** The steps of a booking as a vertical list with its connectors. */
export function StatusTimeline({
  booking,
  events,
  stackMeta,
}: {
  booking: Booking
  events: readonly BookingEvent[]
  stackMeta?: boolean
}) {
  const approver = useApprover(booking.room.type)
  const steps = buildTimeline(booking, events, approver)
  return (
    <ol aria-label='Status pengajuan'>
      {steps.map((step, i) => (
        <Step
          key={step.key}
          state={step.state}
          title={step.title}
          description={step.description}
          meta={step.meta}
          hasNext={i < steps.length - 1}
          stackMeta={stackMeta}
          tone={step.tone}
        />
      ))}
    </ol>
  )
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className='flex items-start justify-between gap-4'>
      <dt className='type-body-sm text-fg-secondary'>{label}</dt>
      <dd className='text-end type-label text-fg'>{value}</dd>
    </div>
  )
}

/** The facts of a booking as label/value rows (Figma "Ringkasan"). */
export function BookingFacts({ booking }: { booking: Booking }) {
  return (
    <dl className='flex flex-col gap-2.5'>
      <Row label='Ruang' value={booking.room.code} />
      <Row label='Tipe' value={ROOM_TYPE_LABELS[booking.room.type]} />
      <Row label='Tanggal' value={dayShort(booking.start_at)} />
      <Row
        label='Waktu'
        value={formatTimeRange(booking.start_at, booking.end_at)}
      />
      {booking.participants ? (
        <Row label='Peserta' value={`${booking.participants} orang`} />
      ) : null}
      <Row label='PJ' value={booking.requester.name} />
    </dl>
  )
}

/** Icon buttons of a list row: detail (where it is not beside the list), edit and cancel while allowed. */
export function RowActions({
  booking,
  now,
  onDetail,
  onEdit,
  onCancel,
}: {
  booking: Booking
  now: Date
  /** Left out where the detail is already beside the list. */
  onDetail?: (booking: Booking) => void
  onEdit?: (booking: Booking) => void
  onCancel?: (booking: Booking) => void
}) {
  const edit = canEdit(booking, now)
  const cancel = canCancel(booking, now)
  if (!onDetail && !edit && !cancel) return null
  const where = `${booking.room.code}, ${dayShort(booking.start_at)}`
  return (
    <div className='flex items-center justify-start gap-1'>
      {onDetail && (
        <Button
          variant='ghost'
          size='icon'
          title='Lihat detail'
          aria-label={`Lihat detail ${where}`}
          onClick={() => onDetail(booking)}
        >
          <Eye />
        </Button>
      )}
      {edit && (
        <Button
          variant='ghost'
          size='icon'
          title='Ubah'
          aria-label={`Ubah ${where}`}
          onClick={() => onEdit?.(booking)}
        >
          <Pencil />
        </Button>
      )}
      {cancel && (
        <Button
          variant='ghost'
          size='icon'
          title='Batalkan'
          aria-label={`Batalkan ${where}`}
          onClick={() => onCancel?.(booking)}
        >
          <Ban />
        </Button>
      )}
    </div>
  )
}
