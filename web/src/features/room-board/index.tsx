import { useMemo } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { AlertCircle, ChevronLeft, ChevronRight } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type Room, type TimelineItem } from '@/lib/api-types'
import {
  addDays,
  formatDate,
  formatTimeRange,
  minutesOfDay,
  timeFromMinutes,
  todayWib,
} from '@/lib/datetime'
import { cn } from '@/lib/utils'
import { useNow } from '@/hooks/use-now'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/layout/page'
import { SegmentedControl } from '@/components/segmented-control'
import {
  type TypeGroup,
  TYPE_GROUP_LABELS,
  groupOf,
} from '@/features/bookings/request/policy'
import { useRoomTypes } from '@/features/room-types/api'
import { useRooms } from '@/features/rooms/api'
import { useRoomsDay } from './api'
import {
  SLOT_MINUTES,
  boardHours,
  boardRows,
  currentRow,
  placeInColumn,
  roomHours,
} from './rules'

const route = getRouteApi('/_authenticated/today')
const ALL = 'ALL'
const ROW_HEIGHT = '1.625rem'
const NO_ITEMS: TimelineItem[] = []

/** "Papan Ruang": the whole campus on one day, rooms across and hours down, today unless another day is picked. */
export function RoomBoard() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const now = useNow(30_000)
  const today = todayWib(now)
  const date = search.date ?? today
  const group = search.group
  const policies = useRoomTypes()
  const rooms = useRooms({ per_page: 100, is_active: true })

  const shown = useMemo(
    () =>
      (rooms.data?.data ?? []).filter(
        (r) => !group || groupOf(r.type) === group
      ),
    [rooms.data, group]
  )
  const days = useRoomsDay(shown, date)
  const hours = useMemo(
    () => boardHours(shown, policies.data ?? []),
    [shown, policies.data]
  )
  const failed = days.find((d) => d.isError)

  const go = (patch: Partial<typeof search>) =>
    navigate({ search: (prev) => ({ ...prev, ...patch }), replace: true })

  return (
    <Page
      title='Papan Ruang'
      description='Pemakaian semua ruang dalam satu hari: kuliah, jadwal tetap, dan booking yang disetujui.'
    >
      <section
        aria-label='Papan ruang'
        className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
      >
        <div className='flex flex-wrap items-center gap-3'>
          <div className='flex items-center gap-1'>
            <Button
              variant='outline'
              size='icon'
              aria-label='Hari sebelumnya'
              onClick={() => go({ date: addDays(date, -1) })}
            >
              <ChevronLeft />
            </Button>
            <Button
              variant='outline'
              disabled={date === today}
              onClick={() => go({ date: undefined })}
            >
              Hari ini
            </Button>
            <Button
              variant='outline'
              size='icon'
              aria-label='Hari berikutnya'
              onClick={() => go({ date: addDays(date, 1) })}
            >
              <ChevronRight />
            </Button>
          </div>
          <Input
            type='date'
            aria-label='Tanggal'
            className='w-auto'
            value={date}
            onChange={(e) => e.target.value && go({ date: e.target.value })}
          />
          <p className='type-card-title text-fg'>
            {formatDate(`${date}T12:00:00+07:00`)}
          </p>
          <SegmentedControl<string>
            aria-label='Kelompok ruang'
            className='sm:ms-auto'
            value={group ?? ALL}
            onValueChange={(v) =>
              go({ group: v === ALL ? undefined : (v as TypeGroup) })
            }
            options={[
              { value: ALL, label: 'Semua' },
              ...(['KELAS', 'LAB', 'LAINNYA'] as const).map((g) => ({
                value: g,
                label: TYPE_GROUP_LABELS[g],
              })),
            ]}
          />
        </div>

        <ul className='flex flex-wrap items-center gap-x-4 gap-y-1 type-body-sm text-fg-secondary'>
          <li className='flex items-center gap-2'>
            <span className='size-3 rounded-sm bg-inverse' />
            Jadwal tetap
          </li>
          <li className='flex items-center gap-2'>
            <span className='size-3 rounded-sm border border-line-accent bg-accent-subtle' />
            Booking disetujui
          </li>
          <li className='flex items-center gap-2'>
            <span className='size-3 rounded-sm bg-muted' />
            Di luar jam operasional
          </li>
        </ul>

        {(rooms.isError || failed) && (
          <Alert variant='error'>
            <AlertCircle />
            <AlertDescription>
              {getErrorMessage(rooms.error ?? failed?.error)}
            </AlertDescription>
          </Alert>
        )}

        {rooms.isPending ? (
          <Skeleton className='h-96' />
        ) : shown.length === 0 ? (
          <EmptyState
            title='Tidak ada ruang'
            description='Belum ada ruang aktif di kelompok ini.'
          />
        ) : (
          <Board
            rooms={shown}
            items={days.map((d) => d.data ?? NO_ITEMS)}
            loading={days.some((d) => d.isPending)}
            policies={policies.data ?? []}
            date={date}
            hours={hours}
            now={now}
          />
        )}
      </section>
    </Page>
  )
}

function Board({
  rooms,
  items,
  loading,
  policies,
  date,
  hours,
  now,
}: {
  rooms: Room[]
  items: TimelineItem[][]
  loading: boolean
  policies: Parameters<typeof roomHours>[1]
  date: string
  hours: { open: string; close: string }
  now: Date
}) {
  const rows = boardRows(hours)
  const open = minutesOfDay(hours.open)
  const slotStart = (row: number) => timeFromMinutes(open + row * SLOT_MINUTES)
  const nowRow = currentRow(now, date, hours)

  return (
    <div
      className='max-h-[70dvh] overflow-auto rounded-xl border'
      aria-busy={loading}
    >
      <div
        role='grid'
        aria-label={`Papan ruang ${date}`}
        className='grid gap-x-1.5 gap-y-1 bg-card p-2 text-xs'
        style={{
          gridTemplateColumns: `3rem repeat(${rooms.length}, minmax(7rem, 1fr))`,
          gridTemplateRows: `auto repeat(${rows}, ${ROW_HEIGHT})`,
          minWidth: `${3 + rooms.length * 7.5}rem`,
        }}
      >
        <div
          className='sticky start-0 top-0 z-30 bg-card'
          style={{ gridColumn: 1, gridRow: 1 }}
        />
        {rooms.map((room, c) => (
          <div
            key={room.id}
            role='columnheader'
            title={`${room.name} · kapasitas ${room.capacity}`}
            className='sticky top-0 z-20 flex h-9 items-center justify-center truncate rounded-full bg-card px-2 type-label text-fg'
            style={{ gridColumn: c + 2, gridRow: 1 }}
          >
            <span className='truncate'>{room.code}</span>
          </div>
        ))}

        {Array.from({ length: rows }, (_, row) => (
          <div
            key={`label-${row}`}
            className={cn(
              'sticky start-0 z-20 flex items-center justify-end bg-card pe-1 type-label-sm tabular-nums',
              row === nowRow ? 'text-fg-accent' : 'text-fg-secondary'
            )}
            style={{ gridColumn: 1, gridRow: row + 2 }}
          >
            {slotStart(row)}
          </div>
        ))}

        {rooms.flatMap((room, c) => {
          const own = roomHours(room, policies)
          const ownOpen = minutesOfDay(own.open)
          const ownClose = minutesOfDay(own.close)
          return Array.from({ length: rows }, (_, row) => {
            const minute = open + row * SLOT_MINUTES
            const closed = minute < ownOpen || minute >= ownClose
            return (
              <div
                key={`${room.id}-${row}`}
                className={cn(
                  'rounded-md border',
                  closed
                    ? 'border-transparent bg-muted/60'
                    : row === nowRow
                      ? 'border-line-accent bg-surface'
                      : 'bg-surface'
                )}
                style={{ gridColumn: c + 2, gridRow: row + 2 }}
              />
            )
          })
        })}

        {rooms.flatMap((room, c) =>
          placeInColumn(items[c] ?? NO_ITEMS, date, hours).map(
            ({ item, rowStart, rowEnd }) => (
              <div
                key={item.id}
                className={cn(
                  'z-10 flex min-w-0 flex-col justify-center overflow-hidden rounded-md px-2 py-1',
                  item.kind === 'SCHEDULE'
                    ? 'bg-inverse text-fg-inverse'
                    : 'border border-line-accent bg-accent-subtle text-fg-accent'
                )}
                style={{
                  gridColumn: c + 2,
                  gridRow: `${rowStart + 2} / ${rowEnd + 2}`,
                }}
                title={`${room.code}: ${item.title} (${formatTimeRange(item.start_at, item.end_at)})`}
              >
                <p className='truncate type-label'>{item.title}</p>
                {rowEnd - rowStart > 1 && (
                  <p className='truncate type-label-sm tabular-nums opacity-70'>
                    {formatTimeRange(item.start_at, item.end_at)}
                  </p>
                )}
              </div>
            )
          )
        )}
      </div>
    </div>
  )
}
