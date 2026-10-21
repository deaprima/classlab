import { bookingChangeRules } from '@/config/attendance-rules'
import { type Booking, type BookingStatus } from '@/lib/api-types'
import { formatDuration } from '@/lib/datetime'

const MINUTE = 60_000
const HOUR = 60 * MINUTE

type Timed = Pick<Booking, 'status' | 'start_at'>

const startMs = (b: Pick<Booking, 'start_at'>) => new Date(b.start_at).getTime()

/** Only waiting or approved bookings can still be changed (BR-26). */
const CHANGEABLE: readonly BookingStatus[] = ['MENUNGGU', 'DISETUJUI']

/** The owner may edit until `editCutoffMinutes` before the start. The server re-checks. */
export function canEdit(booking: Timed, now: Date): boolean {
  return (
    CHANGEABLE.includes(booking.status) &&
    now.getTime() <
      startMs(booking) - bookingChangeRules.editCutoffMinutes * MINUTE
  )
}

/** The owner may cancel a waiting or approved booking that has not started. */
export function canCancel(booking: Timed, now: Date): boolean {
  return CHANGEABLE.includes(booking.status) && now.getTime() < startMs(booking)
}

/** Cancelling an approved booking less than 2 hours before the start adds a violation point (BR-14). */
export function isLateCancel(booking: Timed, now: Date): boolean {
  return (
    booking.status === 'DISETUJUI' &&
    startMs(booking) - now.getTime() < bookingChangeRules.lateCancelHours * HOUR
  )
}

/** An edit of an approved booking can send it back to "Menunggu" (BR-26). */
export function mayNeedReapproval(booking: Pick<Booking, 'status'>): boolean {
  return booking.status === 'DISETUJUI'
}

type StatusNote = {
  tone: 'info' | 'error'
  title: string
  description: string
}

/** "23 jam 40 menit" for a span in milliseconds. */
function span(ms: number): string {
  return formatDuration(Math.max(1, Math.floor(ms / MINUTE)))
}

/** The note under the title of the detail page: what the status means right now. */
export function statusNote(
  booking: Pick<
    Booking,
    | 'status'
    | 'approval_mode'
    | 'expires_at'
    | 'decision_reason'
    | 'checked_out_at'
  >,
  now: Date,
  approver: string
): StatusNote {
  switch (booking.status) {
    case 'MENUNGGU': {
      const left = booking.expires_at
        ? new Date(booking.expires_at).getTime() - now.getTime()
        : null
      return {
        tone: 'info',
        title: `Menunggu validasi ${approver}`,
        description:
          left !== null && left > 0
            ? `Kedaluwarsa otomatis dalam ${span(left)} bila tidak diputuskan.`
            : 'Menunggu keputusan.',
      }
    }
    case 'DISETUJUI':
      return {
        tone: 'info',
        title:
          booking.approval_mode === 'AUTO'
            ? 'Disetujui otomatis'
            : `Disetujui ${approver}`,
        description:
          'Check-in dibuka 15 menit sebelum jam mulai lewat QR di pintu.',
      }
    case 'BERLANGSUNG':
      return {
        tone: 'info',
        title: 'Sedang berlangsung',
        description: 'Check-out dengan foto kondisi ruang setelah selesai.',
      }
    case 'SELESAI':
      return {
        tone: 'info',
        title: 'Selesai',
        description: 'Check-out sudah tercatat.',
      }
    case 'DITOLAK':
      return {
        tone: 'error',
        title: 'Ditolak',
        description: booking.decision_reason || 'Pengajuan ini ditolak.',
      }
    case 'DIBATALKAN':
      return {
        tone: 'info',
        title: 'Dibatalkan',
        description: 'Slot sudah dibuka kembali untuk pengguna lain.',
      }
    case 'KEDALUWARSA':
      return {
        tone: 'info',
        title: 'Kedaluwarsa',
        description: 'Pengajuan tidak diputuskan dalam batas waktu.',
      }
    case 'TIDAK_HADIR':
      return {
        tone: 'error',
        title: 'Tidak hadir',
        description:
          'Check-in tidak dilakukan dalam jendela waktu; tercatat sebagai pelanggaran.',
      }
    case 'TANPA_CHECKOUT':
      return {
        tone: 'error',
        title: 'Tanpa check-out',
        description: 'Check-out tidak dilakukan; tercatat sebagai pelanggaran.',
      }
  }
}
