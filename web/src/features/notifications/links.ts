import { type AppNotification, type UserRole } from '@/lib/api-types'
import { ACCESS } from '@/lib/rbac'

type NotificationTarget =
  | { to: '/approvals'; search: { booking: string } }
  | { to: '/bookings'; search: { booking: string } }
  | { to: '/reports'; search: { report: string } }

/** Where a notification leads, for the reader's role. Null when it has no page to open. */
export function notificationTarget(
  n: Pick<AppNotification, 'type' | 'ref_type' | 'ref_id'>,
  role: UserRole | undefined
): NotificationTarget | null {
  if (!n.ref_id || !role) return null
  if (n.ref_type === 'BOOKING') {
    // A new request is something TU/Kalab acts on in the queue.
    if (
      n.type === 'BOOKING_CREATED' &&
      (ACCESS.approveBookings as readonly UserRole[]).includes(role)
    ) {
      return { to: '/approvals', search: { booking: n.ref_id } }
    }
    if ((ACCESS.viewBookings as readonly UserRole[]).includes(role)) {
      return { to: '/bookings', search: { booking: n.ref_id } }
    }
    return null
  }
  if (n.ref_type === 'REPORT') {
    return { to: '/reports', search: { report: n.ref_id } }
  }
  return null
}
