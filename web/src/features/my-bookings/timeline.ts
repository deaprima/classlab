import { type Booking, type BookingEvent } from '@/lib/api-types'
import { formatDateTime } from '@/lib/datetime'

type StepState = 'done' | 'current' | 'upcoming'

type TimelineStep = {
  key: string
  title: string
  description?: string
  /** Right-aligned note, usually a time. */
  meta?: string
  state: StepState
  tone?: 'error'
}

const FIELD_LABELS: Record<string, string> = {
  start_at: 'waktu',
  end_at: 'waktu',
  purpose: 'kegiatan',
  participants: 'peserta',
  notes: 'catatan',
  details: 'isian',
  letter_attachment_id: 'surat',
  letter_number: 'nomor surat',
}

/** "waktu, peserta" for the fields an EDITED event changed, each name once. */
function changedFields(event: BookingEvent): string {
  const names = Object.keys(event.changes ?? {}).map(
    (key) => FIELD_LABELS[key] ?? key
  )
  return [...new Set(names)].join(', ')
}

/**
 * The steps of a booking as the owner sees them (Figma "Detail Pengajuan"): sent, edits, review,
 * then check-in and use. What is still ahead is shown greyed out. Built from the booking's own
 * fields and its append-only events (BR-06).
 */
export function buildTimeline(
  booking: Booking,
  events: readonly BookingEvent[],
  approver: string
): TimelineStep[] {
  const steps: TimelineStep[] = [
    {
      key: 'sent',
      title: 'Pengajuan dikirim',
      meta: formatDateTime(booking.created_at),
      state: 'done',
    },
  ]

  for (const e of events.filter((ev) => ev.kind === 'EDITED')) {
    const what = changedFields(e)
    steps.push({
      key: `edit-${e.id}`,
      title: 'Pengajuan diubah',
      description: what ? `Diubah: ${what}` : undefined,
      meta: formatDateTime(e.created_at),
      state: 'done',
    })
  }

  const approved: TimelineStep = {
    key: 'approved',
    title:
      booking.approval_mode === 'AUTO'
        ? 'Disetujui otomatis'
        : `Disetujui ${approver}`,
    meta: booking.decided_at ? formatDateTime(booking.decided_at) : undefined,
    state: 'done',
  }
  const checkedIn: TimelineStep = {
    key: 'checked-in',
    title: 'Check-in',
    meta: booking.checked_in_at
      ? formatDateTime(booking.checked_in_at)
      : undefined,
    state: 'done',
  }
  const cancelled = [...events]
    .reverse()
    .find((e) => e.kind === 'STATUS' && e.to_status === 'DIBATALKAN')

  switch (booking.status) {
    case 'MENUNGGU':
      steps.push(
        {
          key: 'review',
          title: `Ditinjau ${approver}`,
          description: 'Menunggu keputusan',
          state: 'current',
        },
        { key: 'approved', title: 'Disetujui', state: 'upcoming' },
        { key: 'use', title: 'Check-in & pemakaian', state: 'upcoming' }
      )
      break
    case 'DITOLAK':
      steps.push(
        { key: 'review', title: `Ditinjau ${approver}`, state: 'done' },
        {
          key: 'rejected',
          title: 'Ditolak',
          description: booking.decision_reason || undefined,
          meta: booking.decided_at
            ? formatDateTime(booking.decided_at)
            : undefined,
          state: 'done',
          tone: 'error',
        }
      )
      break
    case 'DISETUJUI':
      steps.push(approved, {
        key: 'use',
        title: 'Check-in & pemakaian',
        description: 'Menunggu jam mulai',
        state: 'current',
      })
      break
    case 'BERLANGSUNG':
      steps.push(approved, checkedIn, {
        key: 'use',
        title: 'Pemakaian',
        description: 'Check-out dengan foto setelah selesai',
        state: 'current',
      })
      break
    case 'SELESAI':
      steps.push(approved, checkedIn, {
        key: 'done',
        title: 'Selesai',
        description:
          booking.checkout_condition === 'ADA_MASALAH'
            ? 'Ada kerusakan yang dilaporkan'
            : undefined,
        meta: booking.checked_out_at
          ? formatDateTime(booking.checked_out_at)
          : undefined,
        state: 'done',
      })
      break
    case 'TIDAK_HADIR':
      steps.push(approved, {
        key: 'no-show',
        title: 'Tidak hadir',
        description: 'Check-in tidak dilakukan dalam jendela waktu',
        state: 'done',
        tone: 'error',
      })
      break
    case 'TANPA_CHECKOUT':
      steps.push(approved, checkedIn, {
        key: 'no-checkout',
        title: 'Tanpa check-out',
        description: 'Check-out tidak dilakukan setelah selesai',
        state: 'done',
        tone: 'error',
      })
      break
    case 'DIBATALKAN':
      steps.push({
        key: 'cancelled',
        title: 'Dibatalkan',
        description: cancelled?.reason || undefined,
        meta: cancelled ? formatDateTime(cancelled.created_at) : undefined,
        state: 'done',
      })
      break
    case 'KEDALUWARSA':
      steps.push({
        key: 'expired',
        title: 'Kedaluwarsa',
        description: 'Tidak diputuskan dalam batas waktu',
        state: 'done',
      })
      break
  }
  return steps
}
