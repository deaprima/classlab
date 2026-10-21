import { Link } from '@tanstack/react-router'
import { MoreHorizontal } from 'lucide-react'
import { type Booking, type BookingEvent } from '@/lib/api-types'
import { formatTimeRange } from '@/lib/datetime'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
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
}: {
  booking: Booking
  events: readonly BookingEvent[]
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
        />
      ))}
    </ol>
  )
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className='flex items-start justify-between gap-4'>
      <dt className='type-body text-fg-secondary'>{label}</dt>
      <dd className='text-end type-label text-fg'>{value}</dd>
    </div>
  )
}

/** The facts of a booking as label/value rows (Figma "Ringkasan"). */
export function BookingFacts({ booking }: { booking: Booking }) {
  return (
    <dl className='flex flex-col gap-3'>
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

/** The "…" menu of a list row: detail, and edit or cancel while they are still allowed. */
export function RowMenu({
  booking,
  now,
  withChanges,
  onEdit,
  onCancel,
}: {
  booking: Booking
  now: Date
  /** Pengajuan Saya offers edit and cancel; the history only offers the detail. */
  withChanges: boolean
  onEdit?: (booking: Booking) => void
  onCancel?: (booking: Booking) => void
}) {
  const edit = withChanges && canEdit(booking, now)
  const cancel = withChanges && canCancel(booking, now)
  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button
          variant='ghost'
          size='icon'
          aria-label={`Aksi ${booking.room.code}, ${dayShort(booking.start_at)}`}
        >
          <MoreHorizontal />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem asChild>
          <Link to='/bookings/$id' params={{ id: booking.id }}>
            Lihat detail
          </Link>
        </DropdownMenuItem>
        {edit && (
          <DropdownMenuItem onSelect={() => onEdit?.(booking)}>
            Ubah
          </DropdownMenuItem>
        )}
        {cancel && (
          <DropdownMenuItem onSelect={() => onCancel?.(booking)}>
            Batalkan
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
