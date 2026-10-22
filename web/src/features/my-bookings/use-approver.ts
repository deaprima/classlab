import { type Booking } from '@/lib/api-types'
import { approverLabel } from '@/features/bookings/request/policy'
import { useRoomTypes } from '@/features/room-types/api'

/** "TU", "Pengurus Lab", "TU / Admin": who decides on this room type (a generic word until it loads). */
export function useApprover(roomType: Booking['room']['type']): string {
  const policies = useRoomTypes()
  const policy = policies.data?.find((p) => p.type === roomType)
  return policy ? approverLabel(policy) : 'pengelola'
}
