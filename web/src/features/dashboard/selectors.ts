import { attendanceRules } from '@/config/attendance-rules'
import { type Booking } from '@/lib/api-types'

const MINUTE = 60_000

const startMs = (b: Booking) => new Date(b.start_at).getTime()

/** Bookings that still matter on the dashboard: approved or running. */
const ACTIVE: readonly Booking['status'][] = ['DISETUJUI', 'BERLANGSUNG']

/** Counts a multi-date request once, like the server's quota (BR-15). */
export function countActive(bookings: readonly Booking[]): number {
  const keys = new Set(
    bookings
      .filter((b) => b.status === 'MENUNGGU' || ACTIVE.includes(b.status))
      .map((b) => b.group_id ?? b.id)
  )
  return keys.size
}

/** The card at the top: the room in use right now (to check out of), else the next approved one. */
export function pickFeatured(
  bookings: readonly Booking[]
): Booking | undefined {
  const byStart = [...bookings].sort((a, b) => startMs(a) - startMs(b))
  return (
    byStart.find((b) => b.status === 'BERLANGSUNG') ??
    byStart.find((b) => b.status === 'DISETUJUI')
  )
}

/** Approved or running bookings after the featured one, soonest first. */
export function pickUpcoming(
  bookings: readonly Booking[],
  featured: Booking | undefined,
  limit: number
): Booking[] {
  return bookings
    .filter((b) => ACTIVE.includes(b.status) && b.id !== featured?.id)
    .sort((a, b) => startMs(a) - startMs(b))
    .slice(0, limit)
}

/** Requests waiting for a decision (soonest to expire first), then recent rejections. */
export function pickSubmissions(
  bookings: readonly Booking[],
  limit: number
): Booking[] {
  const waiting = bookings
    .filter((b) => b.status === 'MENUNGGU')
    .sort(
      (a, b) =>
        new Date(a.expires_at ?? a.start_at).getTime() -
        new Date(b.expires_at ?? b.start_at).getTime()
    )
  const rejected = bookings
    .filter((b) => b.status === 'DITOLAK')
    .sort((a, b) => startMs(a) - startMs(b))
  return [...waiting, ...rejected].slice(0, limit)
}

type AttendanceStep = 'check-in' | 'check-out' | 'none'

type AttendanceState = {
  step: AttendanceStep
  /** Check-in can be done right now (inside the window). */
  canCheckIn: boolean
  /** Check-out can be done right now. */
  canCheckOut: boolean
}

/** What the featured booking allows at `now`; the server re-checks the window on the real call. */
export function attendanceState(booking: Booking, now: Date): AttendanceState {
  if (booking.status === 'BERLANGSUNG') {
    return { step: 'check-out', canCheckIn: false, canCheckOut: true }
  }
  if (booking.status !== 'DISETUJUI') {
    return { step: 'none', canCheckIn: false, canCheckOut: false }
  }
  const opens = startMs(booking) - attendanceRules.checkInEarlyMinutes * MINUTE
  const closes = startMs(booking) + attendanceRules.checkInLateMinutes * MINUTE
  const t = now.getTime()
  return {
    step: 'check-in',
    canCheckIn: t >= opens && t <= closes,
    canCheckOut: false,
  }
}

/** "Mulai dalam 15 menit", "Sedang berlangsung"; null when there is nothing worth saying. */
export function startsInLabel(booking: Booking, now: Date): string | null {
  if (booking.status === 'BERLANGSUNG') return 'Sedang berlangsung'
  if (booking.status !== 'DISETUJUI') return null
  const minutes = Math.ceil((startMs(booking) - now.getTime()) / MINUTE)
  if (minutes <= 0) return 'Sudah dimulai'
  if (minutes < 60) return `Mulai dalam ${minutes} menit`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `Mulai dalam ${hours} jam`
  return `Mulai dalam ${Math.floor(hours / 24)} hari`
}

/** "sisa 23 j" until a waiting request expires (BR-22); null when it is not waiting. */
export function expiresInLabel(booking: Booking, now: Date): string | null {
  if (booking.status !== 'MENUNGGU' || !booking.expires_at) return null
  const minutes = Math.floor(
    (new Date(booking.expires_at).getTime() - now.getTime()) / MINUTE
  )
  if (minutes <= 0) return 'kedaluwarsa'
  if (minutes < 60) return `sisa ${minutes} mnt`
  return `sisa ${Math.floor(minutes / 60)} j`
}

/** Mahasiswa are greeted by first name; an organisation by its full name. */
export function greetingName(user: { name: string; role: string }): string {
  if (user.role === 'ORGANISASI') return user.name
  return user.name.trim().split(/\s+/)[0] ?? user.name
}
