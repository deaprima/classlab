import { getRouteApi } from '@tanstack/react-router'
import { toast } from 'sonner'
import { formatSlot } from '@/lib/datetime'
import { Card, CardContent } from '@/components/ui/card'
import { Page } from '@/components/layout/page'
import { BookingForm } from './components/booking-form'

const route = getRouteApi('/_authenticated/bookings/new')

export function NewBooking() {
  const search = route.useSearch()
  const navigate = route.useNavigate()

  return (
    <Page
      title='Ajukan Peminjaman'
      description='Isi ruang, waktu, dan keperluan. TU/Kepala Lab akan menyetujui atau menolak pengajuan Anda.'
    >
      <Card className='max-w-3xl'>
        <CardContent>
          <BookingForm
            initial={{
              room_id: search.room_id,
              date: search.date,
              start_time: search.start,
              end_time: search.end,
            }}
            onCreated={(booking) => {
              toast.success(
                `Pengajuan ${booking.room.code} (${formatSlot(booking.start_at, booking.end_at)}) terkirim dan menunggu persetujuan.`
              )
              navigate({ to: '/bookings' })
            }}
          />
        </CardContent>
      </Card>
    </Page>
  )
}
