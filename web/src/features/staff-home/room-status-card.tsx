import { AlertCircle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Skeleton } from '@/components/ui/skeleton'
import { BentoCard } from '@/components/bento-card'
import { type RoomStatusCounts } from './rules'

const RADIUS = 40
const CIRCUMFERENCE = 2 * Math.PI * RADIUS

const SEGMENTS = [
  { key: 'inUse', label: 'Dipakai', stroke: 'stroke-fg', dot: 'bg-fg' },
  {
    key: 'available',
    label: 'Tersedia',
    stroke: 'stroke-line-strong',
    dot: 'bg-line-strong',
  },
  {
    key: 'requested',
    label: 'Diajukan',
    stroke: 'stroke-indicator',
    dot: 'bg-indicator',
  },
] as const

/** "Status Ruang" (Figma): a ring of the rooms in use, available and requested right now, with the numbers beside it. */
export function RoomStatusCard({
  title,
  totalLabel,
  counts,
  action,
  isPending,
  isError,
}: {
  title: string
  /** Under the big number in the ring ("Total Ruang"). */
  totalLabel: string
  counts: RoomStatusCounts | undefined
  /** Link beside the title, to the schedule of the rooms. */
  action?: React.ReactNode
  isPending: boolean
  isError: boolean
}) {
  let offset = 0
  return (
    <BentoCard title={title} action={action}>
      {isPending ? (
        <Skeleton className='mx-auto size-48 rounded-full' />
      ) : isError || !counts ? (
        <p className='flex items-center gap-2 type-body text-fg-secondary'>
          <AlertCircle className='size-4' />
          Status ruang tidak dapat dimuat.
        </p>
      ) : (
        <>
          <div className='relative mx-auto size-48'>
            <svg
              viewBox='0 0 100 100'
              className='size-full -rotate-90'
              role='img'
              aria-label={`${counts.total} ruang: ${counts.inUse} dipakai, ${counts.available} tersedia, ${counts.requested} diajukan`}
            >
              <circle
                cx='50'
                cy='50'
                r={RADIUS}
                fill='none'
                strokeWidth='12'
                className='stroke-muted'
              />
              {counts.total > 0 &&
                SEGMENTS.map(({ key, stroke }) => {
                  const length = (counts[key] / counts.total) * CIRCUMFERENCE
                  const start = offset
                  offset += length
                  return length === 0 ? null : (
                    <circle
                      key={key}
                      cx='50'
                      cy='50'
                      r={RADIUS}
                      fill='none'
                      strokeWidth='12'
                      className={stroke}
                      strokeDasharray={`${length} ${CIRCUMFERENCE - length}`}
                      strokeDashoffset={-start}
                    />
                  )
                })}
            </svg>
            <div className='absolute inset-0 flex flex-col items-center justify-center'>
              <span className='type-stat-l text-fg'>{counts.total}</span>
              <span className='type-body-sm text-fg-secondary'>
                {totalLabel}
              </span>
            </div>
          </div>
          <ul className='flex flex-col gap-2'>
            {SEGMENTS.map(({ key, label, dot }) => (
              <li
                key={key}
                className='flex items-center gap-2 type-body text-fg-secondary'
              >
                <span className={cn('size-2 rounded-full', dot)} />
                {label}
                <span className='ms-auto text-fg tabular-nums'>
                  {counts[key]}
                </span>
              </li>
            ))}
          </ul>
        </>
      )}
    </BentoCard>
  )
}
