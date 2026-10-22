import { Link } from '@tanstack/react-router'
import { AlertCircle, ChevronRight, DoorOpen } from 'lucide-react'
import { formatTimeRange } from '@/lib/datetime'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { BentoCard } from '@/components/bento-card'
import { EmptyState } from '@/components/empty-state'
import { StatusPill } from '@/components/status-pill'
import { ROOM_TYPE_LABELS } from '@/features/rooms/labels'
import { type InUseItem } from './rules'

const LIMIT = 6

/** "Sedang Dipakai": which rooms are in use right now, by which lecture or booking, and until when. */
export function InUseCard({
  items,
  isPending,
  isError,
}: {
  items: InUseItem[] | undefined
  isPending: boolean
  isError: boolean
}) {
  const shown = items?.slice(0, LIMIT) ?? []
  return (
    <BentoCard title='Sedang Dipakai Sekarang'>
      {isPending ? (
        <Skeleton className='h-40' />
      ) : isError || !items ? (
        <p className='flex items-center gap-2 type-body text-fg-secondary'>
          <AlertCircle className='size-4' />
          Pemakaian ruang tidak dapat dimuat.
        </p>
      ) : items.length === 0 ? (
        <EmptyState
          icon={DoorOpen}
          title='Tidak ada ruang yang dipakai'
          description='Ruang yang sedang berkuliah atau dibooking tampil di sini.'
          className='py-4'
        />
      ) : (
        <ul className='flex flex-col gap-3'>
          {shown.map((item) => (
            <li
              key={item.room.id}
              className='flex flex-wrap items-center justify-between gap-3 rounded-xl bg-subtle p-3'
            >
              <div className='min-w-0'>
                <p className='truncate type-card-title text-fg'>
                  {item.room.code}{' '}
                  <span className='font-normal text-fg-secondary'>
                    {item.title}
                  </span>
                </p>
                <p className='truncate type-body-sm text-fg-secondary'>
                  {ROOM_TYPE_LABELS[item.room.type]} ·{' '}
                  {formatTimeRange(item.start_at, item.end_at)}
                </p>
              </div>
              <StatusPill
                status='Digunakan'
                label={item.kind === 'SCHEDULE' ? 'Jadwal' : 'Booking'}
              />
            </li>
          ))}
        </ul>
      )}
      {items && items.length > LIMIT && (
        <p className='type-body-sm text-fg-secondary'>
          +{items.length - LIMIT} ruang lain sedang dipakai.
        </p>
      )}
      <Button variant='ghost' className='w-fit' asChild>
        <Link to='/today'>
          Lihat jadwal lengkap
          <ChevronRight />
        </Link>
      </Button>
    </BentoCard>
  )
}
