import { useMemo } from 'react'
import { Link } from '@tanstack/react-router'
import {
  AlertCircle,
  CalendarDays,
  CalendarPlus,
  ChevronRight,
  Inbox,
  Wrench,
} from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage } from '@/lib/api-error'
import {
  formatDate,
  formatDateShort,
  formatTimeRange,
  toWibDate,
} from '@/lib/datetime'
import { useNow } from '@/hooks/use-now'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { BentoCard } from '@/components/bento-card'
import { DateBadge } from '@/components/date-badge'
import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/layout/page'
import { BookingStatusBadge } from '@/components/status-badge'
import { useMe } from '@/features/auth/api'
import { useBookings } from '@/features/bookings/api'
import { roomTypeConfig } from '@/features/room-types/config'
import { NextBookingCard } from './next-booking-card'
import {
  countActive,
  expiresInLabel,
  greetingName,
  pickFeatured,
  pickSubmissions,
  pickUpcoming,
} from './selectors'

const UPCOMING_LIMIT = 3
const SUBMISSIONS_LIMIT = 4

/** Beranda of MAHASISWA and ORGANISASI (docs/DESIGN.md: "Dashboard Mahasiswa", "Beranda Organisasi"). */
export function Dashboard() {
  const user = useAuthStore((state) => state.auth.user)
  const me = useMe()
  const now = useNow()
  // Bookings that have not ended yet; fixed per visit so the query key stays stable.
  const from = useMemo(() => new Date().toISOString(), [])
  const bookings = useBookings({ mine: true, from, per_page: 100 })

  if (!user) return null
  const isOrganization = user.role === 'ORGANISASI'
  const list = bookings.data?.data ?? []
  const featured = pickFeatured(list)
  const upcoming = pickUpcoming(list, featured, UPCOMING_LIMIT)
  const submissions = pickSubmissions(list, SUBMISSIONS_LIMIT)
  const active = countActive(list)

  const blockedUntil = me.data?.booking_blocked_until
  const blocked = !!blockedUntil && new Date(blockedUntil) > now

  return (
    <Page
      title={`Hai, ${greetingName(user)}`}
      barTitle='Dashboard'
      description={
        isOrganization
          ? 'Ringkasan booking dan jadwal organisasimu.'
          : bookings.isSuccess
            ? active > 0
              ? `Kamu punya ${active} booking aktif.`
              : 'Belum ada booking aktif.'
            : undefined
      }
    >
      {blocked && blockedUntil && (
        <Alert variant='warning' className='mb-5'>
          <AlertCircle />
          <AlertTitle>Pengajuan diblokir sementara</AlertTitle>
          <AlertDescription>
            Poin pelanggaranmu mencapai batas. Kamu bisa mengajukan ruang lagi
            setelah {formatDate(blockedUntil)}. Booking yang sudah disetujui
            tetap berlaku.
          </AlertDescription>
        </Alert>
      )}

      {bookings.isError && (
        <Alert variant='error' className='mb-5'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(bookings.error)}</AlertDescription>
        </Alert>
      )}

      <div className='grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_340px]'>
        <div className='flex min-w-0 flex-col gap-5'>
          {bookings.isPending ? (
            <Skeleton className='h-60 rounded-2xl' />
          ) : featured ? (
            <NextBookingCard booking={featured} now={now} />
          ) : (
            <BentoCard title='Booking Berikutnya'>
              <EmptyState
                icon={CalendarDays}
                title='Belum ada booking disetujui'
                description='Booking yang sudah disetujui akan muncul di sini, lengkap dengan tombol check-in.'
                action={
                  <Button asChild>
                    <Link to='/bookings/new'>Ajukan Ruang</Link>
                  </Button>
                }
              />
            </BentoCard>
          )}

          <BentoCard title='Booking Mendatang'>
            {bookings.isPending ? (
              <Skeleton className='h-32' />
            ) : upcoming.length === 0 ? (
              <p className='type-body text-fg-secondary'>
                Tidak ada booking lain yang akan datang.
              </p>
            ) : (
              <ul className='flex flex-col gap-3'>
                {upcoming.map((b) => (
                  <li
                    key={b.id}
                    className='flex items-center gap-4 rounded-xl bg-subtle p-3'
                  >
                    <DateBadge
                      date={toWibDate(b.start_at)}
                      className='bg-surface'
                    />
                    <div className='min-w-0 grow'>
                      <p className='truncate type-card-title text-fg'>
                        {b.room.code}
                      </p>
                      <p className='truncate type-body-sm text-fg-secondary'>
                        {formatTimeRange(b.start_at, b.end_at)} ·{' '}
                        {roomTypeConfig(b.room.type).label}
                      </p>
                    </div>
                    <BookingStatusBadge status={b.status} />
                  </li>
                ))}
              </ul>
            )}
          </BentoCard>
        </div>

        <div className='flex min-w-0 flex-col gap-5'>
          <BentoCard title='Aksi Cepat'>
            <div className='flex flex-col gap-3'>
              <Button variant='accent' asChild>
                <Link
                  to='/bookings/new'
                  search={isOrganization ? { type: 'AUDITORIUM' } : {}}
                >
                  <CalendarPlus />
                  {isOrganization ? 'Ajukan Auditorium' : 'Booking Ruang'}
                </Link>
              </Button>
              <Button variant='secondary' asChild>
                <Link to='/schedule'>
                  <CalendarDays />
                  Lihat Jadwal Ruang
                </Link>
              </Button>
              <Button variant='secondary' asChild>
                <Link to='/reports'>
                  <Wrench />
                  Lapor Kerusakan
                </Link>
              </Button>
            </div>
          </BentoCard>

          <BentoCard title='Pengajuan Saya'>
            {bookings.isPending ? (
              <Skeleton className='h-24' />
            ) : submissions.length === 0 ? (
              <EmptyState
                icon={Inbox}
                title='Belum ada pengajuan'
                description='Pengajuan yang menunggu keputusan tampil di sini.'
                className='py-4'
              />
            ) : (
              <ul className='flex flex-col gap-4'>
                {submissions.map((b) => {
                  const left = expiresInLabel(b, now)
                  return (
                    <li
                      key={b.id}
                      className='flex items-start justify-between gap-3'
                    >
                      <div className='min-w-0'>
                        <p className='truncate type-card-title text-fg'>
                          {b.room.code}
                        </p>
                        <p className='type-body-sm text-fg-secondary'>
                          {formatDateShort(b.start_at)} ·{' '}
                          {formatTimeRange(b.start_at, b.end_at)}
                          {left && ` · ${left}`}
                        </p>
                      </div>
                      <BookingStatusBadge status={b.status} />
                    </li>
                  )
                })}
              </ul>
            )}
            <Button variant='ghost' size='sm' className='self-start' asChild>
              <Link to='/bookings'>
                Lihat semua
                <ChevronRight />
              </Link>
            </Button>
          </BentoCard>
        </div>
      </div>
    </Page>
  )
}
