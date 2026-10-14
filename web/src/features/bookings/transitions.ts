import { type Booking, type UserRole } from '@/lib/api-types'

/** Whether the booking's time is over; approving it then makes no sense (the API answers 409). */
export function hasEnded(booking: Pick<Booking, 'end_at'>, now: number) {
  return Date.parse(booking.end_at) <= now
}

/**
 * The transitions of brief 4.1 the user may take on this booking. The server enforces the same
 * table; this only decides which buttons to show.
 */
export function availableActions(
  booking: Pick<Booking, 'status' | 'requester_id' | 'end_at'>,
  user: { id: string; role: UserRole } | null,
  now: number
) {
  const isApprover = user?.role === 'TU_KALAB'
  const isOwner = !!user && booking.requester_id === user.id
  const pending = booking.status === 'MENUNGGU'
  return {
    approve: isApprover && pending && !hasEnded(booking, now),
    // Expired pending requests can still be rejected, so the queue can be cleared.
    reject: isApprover && pending,
    cancelOwn: isOwner && pending,
    cancelApproved:
      isApprover && booking.status === 'DISETUJUI' && !hasEnded(booking, now),
  }
}
