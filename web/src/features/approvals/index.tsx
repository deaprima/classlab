import { useState } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { AlertCircle, Clock, Users } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type Booking } from '@/lib/api-types'
import { formatDateTime, formatSlot } from '@/lib/datetime'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Page } from '@/components/layout/page'
import { Pager } from '@/components/pager'
import { useBookings } from '@/features/bookings/api'
import { BookingActions } from '@/features/bookings/components/booking-actions'
import { BookingDetailSheet } from '@/features/bookings/components/booking-detail-sheet'
import { hasEnded } from '@/features/bookings/transitions'

const route = getRouteApi('/_authenticated/approvals/')
const PER_PAGE = 20

export function Approvals() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const queue = useBookings({
    status: 'MENUNGGU',
    page: search.page ?? 1,
    per_page: PER_PAGE,
  })
  const [now] = useState(() => Date.now())
  const openDetail = (id: string | undefined) =>
    navigate({ search: (prev) => ({ ...prev, booking: id }) })
  const items = queue.data?.data ?? []

  return (
    <Page
      title='Persetujuan Peminjaman'
      description={
        queue.data
          ? `${queue.data.meta.total ?? 0} pengajuan menunggu keputusan.`
          : 'Pengajuan yang menunggu keputusan TU/Kepala Lab.'
      }
    >
      {queue.isError && (
        <Alert variant='destructive' className='mb-4'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(queue.error)}</AlertDescription>
        </Alert>
      )}

      {queue.isPending && (
        <div className='grid gap-4 lg:grid-cols-2'>
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className='h-48 rounded-xl' />
          ))}
        </div>
      )}

      {queue.isSuccess && items.length === 0 && (
        <p className='py-12 text-center text-muted-foreground'>
          Tidak ada pengajuan yang menunggu. Semua sudah diputuskan.
        </p>
      )}

      <div className='grid gap-4 lg:grid-cols-2'>
        {items.map((b) => (
          <RequestCard
            key={b.id}
            booking={b}
            expired={hasEnded(b, now)}
            onDetail={() => openDetail(b.id)}
          />
        ))}
      </div>

      <Pager
        meta={queue.data?.meta}
        onPageChange={(page) =>
          navigate({ search: (prev) => ({ ...prev, page }) })
        }
      />

      <BookingDetailSheet
        bookingId={search.booking}
        onClose={() => openDetail(undefined)}
      />
    </Page>
  )
}

function RequestCard({
  booking,
  expired,
  onDetail,
}: {
  booking: Booking
  expired: boolean
  onDetail: () => void
}) {
  return (
    <Card className='gap-4'>
      <CardHeader>
        <div className='flex items-start justify-between gap-2'>
          <div className='space-y-1'>
            <CardTitle>
              {booking.room.code} ·{' '}
              {formatSlot(booking.start_at, booking.end_at)}
            </CardTitle>
            <CardDescription>
              {booking.requester.name} ({booking.requester.identifier})
            </CardDescription>
          </div>
          {expired && <Badge variant='secondary'>Waktu sudah lewat</Badge>}
        </div>
      </CardHeader>
      <CardContent className='flex-1 space-y-2 text-sm'>
        <p className='break-words'>{booking.purpose}</p>
        <p className='flex flex-wrap items-center gap-x-3 gap-y-1 text-muted-foreground'>
          <span>{booking.room.name}</span>
          {booking.participants !== undefined && (
            <span className='inline-flex items-center gap-1'>
              <Users className='size-3.5' aria-hidden />
              {booking.participants} / {booking.room.capacity} orang
            </span>
          )}
          <span className='inline-flex items-center gap-1'>
            <Clock className='size-3.5' aria-hidden />
            Diajukan {formatDateTime(booking.created_at)}
          </span>
        </p>
        {booking.notes && (
          <p className='break-words text-muted-foreground'>
            Catatan: {booking.notes}
          </p>
        )}
        {expired && (
          <p className='text-muted-foreground'>
            Pengajuan ini tidak bisa disetujui lagi; tolak untuk membersihkan
            antrean.
          </p>
        )}
      </CardContent>
      <CardFooter className='flex-wrap justify-between gap-2'>
        <BookingActions booking={booking} />
        <Button variant='ghost' size='sm' onClick={onDetail}>
          Detail
        </Button>
      </CardFooter>
    </Card>
  )
}
