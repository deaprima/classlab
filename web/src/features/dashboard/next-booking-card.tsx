import { useState } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { attendanceRules } from '@/config/attendance-rules'
import { Check, LogOut } from 'lucide-react'
import { type Booking } from '@/lib/api-types'
import { formatDateShort, formatTimeRange } from '@/lib/datetime'
import { Button } from '@/components/ui/button'
import { BentoCard } from '@/components/bento-card'
import { QrScanner } from '@/components/qr-scanner'
import { BookingStatusBadge } from '@/components/status-badge'
import { parseCheckInQr } from '@/features/attendance/rules'
import { roomTypeConfig } from '@/features/room-types/config'
import { attendanceState, startsInLabel } from './selectors'

type NextBookingCardProps = {
  booking: Booking
  now: Date
}

/** "Booking Berikutnya": the room in use (to check out of) or the next approved one (to check in to). */
export function NextBookingCard({ booking, now }: NextBookingCardProps) {
  const [scanning, setScanning] = useState(false)
  const navigate = useNavigate()
  const state = attendanceState(booking, now)
  const label = startsInLabel(booking, now)
  const { label: typeLabel } = roomTypeConfig(booking.room.type)

  return (
    <BentoCard title='Booking Berikutnya'>
      <div className='flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between'>
        <div className='min-w-0'>
          {label && <p className='type-overline text-fg-accent'>{label}</p>}
          <p className='type-stat-xl text-fg'>{booking.room.code}</p>
          <p className='type-body text-fg-secondary'>
            {typeLabel} · {booking.room.capacity} orang
          </p>
        </div>
        <div className='flex flex-col gap-1 sm:items-end'>
          <BookingStatusBadge status={booking.status} />
          <p className='type-stat-l text-fg'>
            {formatTimeRange(booking.start_at, booking.end_at)}
          </p>
          <p className='type-body text-fg-secondary'>
            {formatDateShort(booking.start_at)}
          </p>
        </div>
      </div>

      <div className='flex flex-wrap items-center gap-3 border-t pt-4'>
        <Button disabled={!state.canCheckIn} onClick={() => setScanning(true)}>
          <Check />
          Check-in
        </Button>
        {state.canCheckOut ? (
          <Button variant='secondary' asChild>
            <Link to='/check-out/$id' params={{ id: booking.id }}>
              <LogOut />
              Check-out
            </Link>
          </Button>
        ) : (
          <Button variant='secondary' disabled>
            <LogOut />
            Check-out
          </Button>
        )}
        <p className='type-body-sm text-fg-secondary sm:ms-auto'>
          Scan QR di pintu ruang · dibuka {attendanceRules.checkInEarlyMinutes}{' '}
          menit sebelum
        </p>
      </div>

      {scanning && (
        <QrScanner
          roomCode={booking.room.code}
          onClose={() => setScanning(false)}
          onDetect={(text) => {
            const qr = parseCheckInQr(text)
            if (!qr)
              return 'Itu bukan QR check-in ClassLab. Arahkan ke QR yang menempel di pintu ruang.'
            setScanning(false)
            void navigate({
              to: '/check-in',
              search: { room: qr.room, t: qr.token },
            })
          }}
        />
      )}
    </BentoCard>
  )
}
