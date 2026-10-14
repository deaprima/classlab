import { Link, getRouteApi } from '@tanstack/react-router'
import { AlertCircle, CalendarPlus, X } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type BookingListQuery, type BookingStatus } from '@/lib/api-types'
import {
  addDays,
  formatDateShort,
  formatTimeRange,
  toApiDateTime,
} from '@/lib/datetime'
import { ACCESS, useHasRole } from '@/lib/rbac'
import { BOOKING_STATUS_LABELS } from '@/lib/status-labels'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Page } from '@/components/layout/page'
import { Pager } from '@/components/pager'
import { BookingStatusBadge } from '@/components/status-badge'
import { useRooms } from '@/features/rooms/api'
import { useBookings } from './api'
import { BookingDetailSheet } from './components/booking-detail-sheet'

const route = getRouteApi('/_authenticated/bookings/')
const ALL = 'ALL'
const PER_PAGE = 20

export function Bookings() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const isStudent = useHasRole(ACCESS.createBooking)
  const rooms = useRooms({ per_page: 100 })

  const query: BookingListQuery = {
    page: search.page ?? 1,
    per_page: PER_PAGE,
    status: search.status,
    room_id: search.room_id,
    // A day filter selects bookings that overlap that WIB day.
    ...(search.date && {
      from: toApiDateTime(search.date, '00:00'),
      to: toApiDateTime(addDays(search.date, 1), '00:00'),
    }),
  }
  const bookings = useBookings(query)

  // Any filter change returns to page 1; opening a detail keeps the list where it is.
  const filter = (patch: Partial<typeof search>) =>
    navigate({ search: (prev) => ({ ...prev, ...patch, page: undefined }) })
  const openDetail = (id: string | undefined) =>
    navigate({ search: (prev) => ({ ...prev, booking: id }) })
  const hasFilters = !!(search.status || search.room_id || search.date)
  const rows = bookings.data?.data ?? []

  return (
    <Page
      title='Riwayat Peminjaman'
      description={
        isStudent
          ? 'Pengajuan dan riwayat peminjaman Anda.'
          : 'Semua pengajuan dan riwayat penggunaan ruang.'
      }
      actions={
        isStudent && (
          <Button asChild>
            <Link to='/bookings/new'>
              <CalendarPlus />
              Ajukan Peminjaman
            </Link>
          </Button>
        )
      }
    >
      <Card className='mb-6'>
        <CardContent className='grid items-end gap-4 sm:grid-cols-2 lg:grid-cols-4'>
          <div className='grid gap-2'>
            <Label htmlFor='bk-status'>Status</Label>
            <Select
              value={search.status ?? ALL}
              onValueChange={(v) =>
                filter({ status: v === ALL ? undefined : (v as BookingStatus) })
              }
            >
              <SelectTrigger id='bk-status' className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL}>Semua status</SelectItem>
                {Object.entries(BOOKING_STATUS_LABELS).map(([value, label]) => (
                  <SelectItem key={value} value={value}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className='grid gap-2'>
            <Label htmlFor='bk-room'>Ruang</Label>
            <Select
              value={search.room_id ?? ALL}
              onValueChange={(v) =>
                filter({ room_id: v === ALL ? undefined : v })
              }
            >
              <SelectTrigger id='bk-room' className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL}>Semua ruang</SelectItem>
                {rooms.data?.data.map((room) => (
                  <SelectItem key={room.id} value={room.id}>
                    {room.code} · {room.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className='grid gap-2'>
            <Label htmlFor='bk-date'>Tanggal</Label>
            <Input
              id='bk-date'
              type='date'
              value={search.date ?? ''}
              onChange={(e) => filter({ date: e.target.value || undefined })}
            />
          </div>
          {hasFilters && (
            <Button
              variant='ghost'
              className='justify-self-start'
              onClick={() =>
                filter({
                  status: undefined,
                  room_id: undefined,
                  date: undefined,
                })
              }
            >
              <X />
              Hapus filter
            </Button>
          )}
        </CardContent>
      </Card>

      {bookings.isError && (
        <Alert variant='destructive' className='mb-4'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(bookings.error)}</AlertDescription>
        </Alert>
      )}

      <div className='overflow-x-auto rounded-xl border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Waktu</TableHead>
              <TableHead>Ruang</TableHead>
              <TableHead>Keperluan</TableHead>
              {!isStudent && <TableHead>Peminjam</TableHead>}
              <TableHead>Status</TableHead>
              <TableHead>
                <span className='sr-only'>Aksi</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {bookings.isPending &&
              Array.from({ length: 5 }, (_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={isStudent ? 5 : 6}>
                    <Skeleton className='h-6' />
                  </TableCell>
                </TableRow>
              ))}
            {bookings.isSuccess && rows.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={isStudent ? 5 : 6}
                  className='py-10 text-center text-muted-foreground'
                >
                  {hasFilters
                    ? 'Tidak ada peminjaman yang cocok dengan filter.'
                    : 'Belum ada peminjaman.'}
                </TableCell>
              </TableRow>
            )}
            {rows.map((b) => (
              <TableRow
                key={b.id}
                className='cursor-pointer'
                onClick={() => openDetail(b.id)}
              >
                <TableCell className='whitespace-nowrap'>
                  <div>{formatDateShort(b.start_at)}</div>
                  <div className='text-muted-foreground tabular-nums'>
                    {formatTimeRange(b.start_at, b.end_at)}
                  </div>
                </TableCell>
                <TableCell className='whitespace-nowrap'>
                  {b.room.code}
                  <div className='text-muted-foreground'>{b.room.name}</div>
                </TableCell>
                <TableCell className='max-w-64 truncate'>{b.purpose}</TableCell>
                {!isStudent && (
                  <TableCell className='whitespace-nowrap'>
                    {b.requester.name}
                    <div className='text-muted-foreground'>
                      {b.requester.identifier}
                    </div>
                  </TableCell>
                )}
                <TableCell>
                  <BookingStatusBadge status={b.status} />
                </TableCell>
                <TableCell className='text-end'>
                  <Button
                    variant='ghost'
                    size='sm'
                    onClick={(e) => {
                      e.stopPropagation()
                      openDetail(b.id)
                    }}
                  >
                    Detail
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <Pager
        meta={bookings.data?.meta}
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
