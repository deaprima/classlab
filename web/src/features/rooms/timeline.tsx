import { useMemo, useState } from 'react'
import { Link, getRouteApi } from '@tanstack/react-router'
import { bookingRules } from '@/config/booking-rules'
import {
  AlertCircle,
  ArrowLeft,
  CalendarPlus,
  ChevronLeft,
  ChevronRight,
  Users,
} from 'lucide-react'
import { getErrorMessage, isApiErrorCode } from '@/lib/api-error'
import { type Room, type TimelineItem } from '@/lib/api-types'
import {
  addDays,
  formatDate,
  formatDateShort,
  formatTimeRange,
  minutesOfDay,
  startOfWeek,
  timeFromMinutes,
  toApiDateTime,
  toWibDate,
  todayWib,
} from '@/lib/datetime'
import { ACCESS, useHasRole } from '@/lib/rbac'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Page } from '@/components/layout/page'
import { useRoom, useRoomSchedule } from './api'
import { ROOM_TYPE_LABELS } from './labels'
import { type GridSpec, placeItems, rowCount } from './timeline-layout'

const route = getRouteApi('/_authenticated/rooms/$id')

const KIND_LABELS: Record<TimelineItem['kind'], string> = {
  SCHEDULE: 'Jadwal kuliah',
  BOOKING: 'Peminjaman disetujui',
}

const kindClass: Record<TimelineItem['kind'], string> = {
  SCHEDULE:
    'border-sky-300 bg-sky-100 text-sky-900 dark:border-sky-800 dark:bg-sky-950 dark:text-sky-200',
  BOOKING:
    'border-emerald-300 bg-emerald-100 text-emerald-900 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-200',
}

export function RoomTimeline() {
  const { id } = route.useParams()
  const search = route.useSearch()
  const navigate = route.useNavigate()

  const today = todayWib()
  const date = search.date ?? today
  const view = search.view ?? 'week'
  const days = useMemo(() => {
    if (view === 'day') return [date]
    const monday = startOfWeek(date)
    return Array.from({ length: 7 }, (_, i) => addDays(monday, i))
  }, [date, view])

  const from = toApiDateTime(days[0], '00:00')
  const to = toApiDateTime(addDays(days[days.length - 1], 1), '00:00')
  const room = useRoom(id)
  const schedule = useRoomSchedule(id, from, to)

  const go = (patch: { date?: string; view?: 'day' | 'week' }) =>
    navigate({ search: (prev) => ({ ...prev, ...patch }), replace: true })
  const step = view === 'day' ? 1 : 7

  if (room.isError) {
    return (
      <Page title='Jadwal Ruang'>
        <Alert variant='destructive'>
          <AlertCircle />
          <AlertDescription>
            {isApiErrorCode(room.error, 'NOT_FOUND')
              ? 'Ruang tidak ditemukan atau sudah dihapus.'
              : getErrorMessage(room.error)}
          </AlertDescription>
        </Alert>
        <Button variant='outline' className='mt-4' asChild>
          <Link to='/'>
            <ArrowLeft />
            Kembali ke ketersediaan
          </Link>
        </Button>
      </Page>
    )
  }

  return (
    <Page
      title={
        room.data ? `${room.data.code} · ${room.data.name}` : 'Jadwal Ruang'
      }
      description={room.data && <RoomSummary room={room.data} />}
      actions={<BookButton room={room.data} date={date} />}
    >
      <div className='mb-4 flex flex-wrap items-end gap-3'>
        <div className='flex items-center gap-1'>
          <Button
            variant='outline'
            size='icon'
            aria-label={
              view === 'day' ? 'Hari sebelumnya' : 'Minggu sebelumnya'
            }
            onClick={() => go({ date: addDays(date, -step) })}
          >
            <ChevronLeft />
          </Button>
          <Button variant='outline' onClick={() => go({ date: today })}>
            Hari ini
          </Button>
          <Button
            variant='outline'
            size='icon'
            aria-label={
              view === 'day' ? 'Hari berikutnya' : 'Minggu berikutnya'
            }
            onClick={() => go({ date: addDays(date, step) })}
          >
            <ChevronRight />
          </Button>
        </div>
        <div className='grid gap-1'>
          <Label htmlFor='tl-date' className='sr-only'>
            Tanggal
          </Label>
          <Input
            id='tl-date'
            type='date'
            className='w-auto'
            value={date}
            onChange={(e) => e.target.value && go({ date: e.target.value })}
          />
        </div>
        <Tabs
          value={view}
          onValueChange={(v) => go({ view: v === 'day' ? 'day' : 'week' })}
        >
          <TabsList>
            <TabsTrigger value='day'>Harian</TabsTrigger>
            <TabsTrigger value='week'>Mingguan</TabsTrigger>
          </TabsList>
        </Tabs>
        <Legend />
      </div>

      {schedule.isError && (
        <Alert variant='destructive' className='mb-4'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(schedule.error)}</AlertDescription>
        </Alert>
      )}

      {schedule.isPending ? (
        <Skeleton className='h-[32rem] w-full rounded-xl' />
      ) : (
        <TimelineGrid
          days={days}
          items={schedule.data ?? []}
          room={room.data}
          today={today}
        />
      )}

      <AgendaList days={days} items={schedule.data ?? []} />
    </Page>
  )
}

function RoomSummary({ room }: { room: Room }) {
  return (
    <span className='flex flex-wrap items-center gap-x-2 gap-y-1'>
      {ROOM_TYPE_LABELS[room.type]}
      <span aria-hidden>·</span>
      <span className='inline-flex items-center gap-1'>
        <Users className='size-3.5' aria-hidden />
        {room.capacity} orang
      </span>
      {room.facilities.length > 0 && (
        <>
          <span aria-hidden>·</span>
          {room.facilities.join(', ')}
        </>
      )}
      {!room.is_active && <Badge variant='destructive'>Nonaktif</Badge>}
    </span>
  )
}

function BookButton({ room, date }: { room: Room | undefined; date: string }) {
  const canBook = useHasRole(ACCESS.createBooking)
  if (!canBook || !room?.is_active) return null
  return (
    <Button asChild>
      <Link to='/bookings/new' search={{ room_id: room.id, date }}>
        <CalendarPlus />
        Ajukan di ruang ini
      </Link>
    </Button>
  )
}

function Legend() {
  return (
    <div className='ms-auto flex flex-wrap items-center gap-3 text-xs text-muted-foreground'>
      {(['SCHEDULE', 'BOOKING'] as const).map((kind) => (
        <span key={kind} className='inline-flex items-center gap-1.5'>
          <span
            className={cn('size-3 rounded-sm border', kindClass[kind])}
            aria-hidden
          />
          {KIND_LABELS[kind]}
        </span>
      ))}
      <span>Pengajuan yang masih menunggu tidak ditampilkan.</span>
    </div>
  )
}

function TimelineGrid({
  days,
  items,
  room,
  today,
}: {
  days: string[]
  items: TimelineItem[]
  room: Room | undefined
  today: string
}) {
  const canBook = useHasRole(ACCESS.createBooking) && !!room?.is_active
  const spec: GridSpec = {
    days,
    openTime: bookingRules.openTime,
    closeTime: bookingRules.closeTime,
    slotMinutes: bookingRules.slotMinutes,
  }
  const rows = rowCount(spec)
  const open = minutesOfDay(spec.openTime)
  const close = minutesOfDay(spec.closeTime)
  const placed = placeItems(items, spec)
  // Fixed at mount: past slots are greyed out relative to when the page was opened.
  const [now] = useState(() => Date.now())

  const slotStart = (row: number) =>
    timeFromMinutes(open + row * spec.slotMinutes)

  return (
    <div className='overflow-x-auto rounded-xl border'>
      <div
        className='grid min-w-fit text-xs'
        style={{
          gridTemplateColumns: `3.5rem repeat(${days.length}, minmax(7.5rem, 1fr))`,
          gridTemplateRows: `auto repeat(${rows}, 1.5rem)`,
        }}
      >
        {/* Header row */}
        <div
          className='sticky left-0 z-20 border-b bg-background'
          style={{ gridColumn: 1, gridRow: 1 }}
        />
        {days.map((day, d) => (
          <div
            key={day}
            className={cn(
              'border-s border-b px-2 py-2 text-center font-medium',
              day === today && 'bg-primary/5 text-primary'
            )}
            style={{ gridColumn: d + 2, gridRow: 1 }}
          >
            {formatDateShort(toApiDateTime(day, '12:00')).replace(
              / \d{4}$/,
              ''
            )}
          </div>
        ))}

        {/* Hour labels */}
        {Array.from({ length: rows }, (_, row) =>
          row % 2 === 0 ? (
            <div
              key={`label-${row}`}
              className='sticky left-0 z-20 -mt-2 bg-background pe-2 text-end text-muted-foreground tabular-nums'
              style={{ gridColumn: 1, gridRow: `${row + 2} / span 2` }}
            >
              {slotStart(row)}
            </div>
          ) : null
        )}

        {/* Background cells; for students an empty future cell is a shortcut to the booking form. */}
        {days.flatMap((day, d) =>
          Array.from({ length: rows }, (_, row) => {
            const start = slotStart(row)
            const past = Date.parse(toApiDateTime(day, start)) < now
            const cellClass = cn(
              'border-s',
              row % 2 === 1 ? 'border-b' : 'border-b border-b-border/40',
              past && 'bg-muted/40'
            )
            const style = { gridColumn: d + 2, gridRow: row + 2 }
            if (!canBook || past) {
              return (
                <div
                  key={`${day}-${row}`}
                  className={cellClass}
                  style={style}
                />
              )
            }
            const end = timeFromMinutes(
              Math.min(minutesOfDay(start) + 60, close)
            )
            return (
              <Link
                key={`${day}-${row}`}
                to='/bookings/new'
                search={{ room_id: room?.id, date: day, start, end }}
                tabIndex={-1}
                aria-hidden
                title={`Ajukan ${start}–${end}`}
                className={cn(cellClass, 'hover:bg-primary/10')}
                style={style}
              />
            )
          })
        )}

        {/* Items */}
        {placed.map(({ item, day, rowStart, rowEnd }) => (
          <div
            key={item.id}
            className={cn(
              'z-10 m-0.5 overflow-hidden rounded-md border px-1.5 py-1 leading-tight',
              kindClass[item.kind]
            )}
            style={{
              gridColumn: day + 2,
              gridRow: `${rowStart + 2} / ${rowEnd + 2}`,
            }}
            title={`${item.title} (${formatTimeRange(item.start_at, item.end_at)})`}
          >
            <p className='truncate font-medium'>{item.title}</p>
            <p className='truncate tabular-nums opacity-80'>
              {formatTimeRange(item.start_at, item.end_at)}
            </p>
          </div>
        ))}
      </div>
    </div>
  )
}

/** The same items as text, for screen readers and narrow screens. */
function AgendaList({
  days,
  items,
}: {
  days: string[]
  items: TimelineItem[]
}) {
  const byDay = days
    .map((day) => ({
      day,
      items: items.filter((i) => toWibDate(i.start_at) === day),
    }))
    .filter((g) => g.items.length > 0)

  return (
    <Card className='mt-6'>
      <CardContent>
        <h2 className='mb-3 font-semibold'>Daftar kegiatan</h2>
        {byDay.length === 0 ? (
          <p className='text-sm text-muted-foreground'>
            Tidak ada jadwal kuliah atau peminjaman disetujui pada rentang ini.
          </p>
        ) : (
          <div className='space-y-4'>
            {byDay.map(({ day, items }) => (
              <section key={day}>
                <h3 className='mb-1 text-sm font-medium'>
                  {formatDate(toApiDateTime(day, '12:00'))}
                </h3>
                <ul className='space-y-1 text-sm'>
                  {items.map((item) => (
                    <li key={item.id}>
                      <span className='font-medium tabular-nums'>
                        {formatTimeRange(item.start_at, item.end_at)}
                      </span>{' '}
                      <span className='text-muted-foreground'>
                        · {KIND_LABELS[item.kind]}: {item.title}
                      </span>
                    </li>
                  ))}
                </ul>
              </section>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
