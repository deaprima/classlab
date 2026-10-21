import { ACCESS, useHasRole } from '@/lib/rbac'
import { Bookings } from '@/features/bookings'
import { MyBookings } from './my-bookings-page'

/** "/bookings": the requester's own "Pengajuan Saya"; staff keep the list of every booking. */
export function BookingsEntry() {
  const requester = useHasRole(ACCESS.createBooking)
  return requester ? <MyBookings /> : <Bookings />
}
