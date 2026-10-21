import { getRouteApi } from '@tanstack/react-router'
import { toast } from 'sonner'
import { type Booking, type BookingGroupResult } from '@/lib/api-types'
import { formatSlot } from '@/lib/datetime'
import { Page } from '@/components/layout/page'
import { roomTypeConfig } from '@/features/room-types/config'
import { RequestForm } from './request/request-form'

const route = getRouteApi('/_authenticated/bookings/new')

function isGroup(
  result: Booking | BookingGroupResult
): result is BookingGroupResult {
  return 'group_id' in result && 'bookings' in result
}

/** Ajukan Ruang (and "Ajukan Auditorium" when the type is fixed by the link). */
export function NewBooking() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const locked = search.type
  const title = locked
    ? `Ajukan ${roomTypeConfig(locked).label}`
    : 'Ajukan Ruang'

  return (
    <Page title={title}>
      <RequestForm
        lockedType={locked}
        initial={{
          room_id: search.room_id,
          date: search.date,
          start: search.start,
          end: search.end,
        }}
        onCreated={(result) => {
          const waiting = result.status === 'MENUNGGU'
          if (isGroup(result)) {
            toast.success(
              `Pengajuan ${result.bookings.length} tanggal terkirim. ${waiting ? 'Menunggu persetujuan.' : 'Disetujui.'}`
            )
          } else {
            toast.success(
              `Pengajuan ${result.room.code} (${formatSlot(result.start_at, result.end_at)}) ${waiting ? 'terkirim dan menunggu persetujuan.' : 'disetujui.'}`
            )
          }
          navigate({ to: '/bookings' })
        }}
      />
    </Page>
  )
}
