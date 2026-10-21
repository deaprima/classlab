import { Link } from '@tanstack/react-router'
import { type TimelineItem } from '@/lib/api-types'
import {
  dateParts,
  formatTimeRange,
  minutesOfDay,
  timeFromMinutes,
} from '@/lib/datetime'
import { cn } from '@/lib/utils'
import {
  type GridSpec,
  placeItems,
  rowCount,
} from '@/features/rooms/timeline-layout'

type WeekGridProps = {
  days: string[]
  items: TimelineItem[]
  hours: { open: string; close: string }
  today: string
  roomId: string
  /** Only requesters can turn an empty slot into a request. */
  canBook: boolean
  now: Date
}

const SLOT_MINUTES = 30
const ROW_HEIGHT = '1.625rem'

/** The week of one room (Figma "Jadwal Ruang"): days across, 30-minute rows down. */
export function WeekGrid({
  days,
  items,
  hours,
  today,
  roomId,
  canBook,
  now,
}: WeekGridProps) {
  const spec: GridSpec = {
    days,
    openTime: hours.open,
    closeTime: hours.close,
    slotMinutes: SLOT_MINUTES,
  }
  const rows = rowCount(spec)
  const open = minutesOfDay(hours.open)
  const close = minutesOfDay(hours.close)
  const placed = placeItems(items, spec)
  const slotStart = (row: number) => timeFromMinutes(open + row * SLOT_MINUTES)

  return (
    <div className='overflow-x-auto'>
      <div
        role='grid'
        aria-label='Jadwal mingguan ruang'
        className='grid min-w-[44rem] gap-x-1.5 gap-y-1 text-xs'
        style={{
          gridTemplateColumns: `3rem repeat(${days.length}, minmax(5.5rem, 1fr))`,
          gridTemplateRows: `auto repeat(${rows}, ${ROW_HEIGHT})`,
        }}
      >
        <div style={{ gridColumn: 1, gridRow: 1 }} />
        {days.map((day, d) => {
          const { weekday, day: n } = dateParts(day)
          const isToday = day === today
          return (
            <div
              key={day}
              role='columnheader'
              className={cn(
                'mb-1 flex h-9 items-center justify-center rounded-full type-label',
                isToday
                  ? 'bg-primary text-primary-foreground'
                  : day < today
                    ? 'text-fg-tertiary'
                    : 'text-fg-secondary'
              )}
              style={{ gridColumn: d + 2, gridRow: 1 }}
            >
              {weekday} {n}
            </div>
          )
        })}

        {Array.from({ length: rows }, (_, row) => (
          <div
            key={`label-${row}`}
            className='flex items-center justify-end pe-1 type-label-sm text-fg-secondary tabular-nums'
            style={{ gridColumn: 1, gridRow: row + 2 }}
          >
            {slotStart(row)}
          </div>
        ))}

        {days.flatMap((day, d) =>
          Array.from({ length: rows }, (_, row) => {
            const start = slotStart(row)
            const past = Date.parse(`${day}T${start}:00+07:00`) < now.getTime()
            const style = { gridColumn: d + 2, gridRow: row + 2 }
            const cell = cn(
              'rounded-md border',
              past ? 'border-transparent bg-muted/60' : 'bg-surface'
            )
            if (!canBook || past) {
              return (
                <div key={`${day}-${row}`} className={cell} style={style} />
              )
            }
            const end = timeFromMinutes(
              Math.min(minutesOfDay(start) + 60, close)
            )
            return (
              <Link
                key={`${day}-${row}`}
                to='/bookings/new'
                search={{ room_id: roomId, date: day, start, end }}
                aria-label={`Ajukan ${day} ${start}–${end}`}
                title={`Ajukan ${start} – ${end}`}
                className={cn(
                  cell,
                  'hover:border-line-accent hover:bg-accent-subtle'
                )}
                style={style}
              />
            )
          })
        )}

        {placed.map(({ item, day, rowStart, rowEnd }) => (
          <div
            key={item.id}
            className='z-10 flex min-w-0 flex-col justify-center overflow-hidden rounded-md bg-inverse px-2 py-1 text-fg-inverse'
            style={{
              gridColumn: day + 2,
              gridRow: `${rowStart + 2} / ${rowEnd + 2}`,
            }}
            title={`${item.title} (${formatTimeRange(item.start_at, item.end_at)})`}
          >
            <p className='truncate type-label'>{item.title}</p>
            {rowEnd - rowStart > 2 && (
              <p className='truncate type-label-sm tabular-nums opacity-70'>
                {formatTimeRange(item.start_at, item.end_at)}
              </p>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}
