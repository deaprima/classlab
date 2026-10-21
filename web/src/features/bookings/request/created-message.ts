import { type Booking, type BookingGroupResult } from '@/lib/api-types'
import { formatSlot } from '@/lib/datetime'

function isGroup(
  result: Booking | BookingGroupResult
): result is BookingGroupResult {
  return 'group_id' in result && 'bookings' in result
}

/** The toast after a request went through: says whether it is approved or waiting for a decision. */
export function createdMessage(result: Booking | BookingGroupResult): string {
  const waiting = result.status === 'MENUNGGU'
  if (isGroup(result)) {
    return `Pengajuan ${result.bookings.length} tanggal terkirim. ${waiting ? 'Menunggu persetujuan.' : 'Disetujui.'}`
  }
  return `Pengajuan ${result.room.code} (${formatSlot(result.start_at, result.end_at)}) ${waiting ? 'terkirim dan menunggu persetujuan.' : 'disetujui.'}`
}
