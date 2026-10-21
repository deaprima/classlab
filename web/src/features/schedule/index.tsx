import { useMemo } from 'react'
import { Link, getRouteApi } from '@tanstack/react-router'
import { slotTimes } from '@/config/booking-rules'
import {
  AlertCircle,
  CalendarPlus,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage } from '@/lib/api-error'
import {
  type AvailabilityRoom,
  type RoomTypePolicy,
  type TimelineItem,
} from '@/lib/api-types'
import {
  addDays,
  formatDate,
  startOfWeek,
  toApiDateTime,
  todayWib,
} from '@/lib/datetime'
import { ACCESS, useHasRole } from '@/lib/rbac'
import { AVAILABILITY_LABELS } from '@/lib/status-labels'
import { AVAILABILITY_PILL } from '@/lib/status-pill-map'
import { cn } from '@/lib/utils'
import { useIsMobile } from '@/hooks/use-mobile'
import { useNow } from '@/hooks/use-now'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
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
import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/layout/page'
import { SegmentedControl } from '@/components/segmented-control'
import { StatusPill } from '@/components/status-pill'
import { useAvailability } from '@/features/availability/api'
import { defaultRange, isValidRange } from '@/features/availability/range'
import {
  availableGroups,
  groupOf,
  requestablePolicies,
  TYPE_GROUP_LABELS,
  type TypeGroup,
} from '@/features/bookings/request/policy'
import { useRoomTypes } from '@/features/room-types/api'
import { roomTypeConfig } from '@/features/room-types/config'
import { useRoomSchedule } from '@/features/rooms/api'
import { DayChips, SlotChips } from './day-slots'
import { buildDaySlots } from './slots'
import { WeekGrid } from './week-grid'

const route = getRouteApi('/_authenticated/schedule')

/** Jadwal Ruang (Figma): find a free room on the left, see its week on the right. */
export function Schedule() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const role = useAuthStore((state) => state.auth.user?.role)
  const canBook = useHasRole(ACCESS.createBooking)
  const policiesQuery = useRoomTypes()
  const isMobile = useIsMobile()
  const now = useNow()
  const today = todayWib(now)

  const fallback = useMemo(() => defaultRange(), [])
  const date = search.date ?? fallback.date
  const start = search.start ?? fallback.start
  const end = search.end ?? fallback.end
  const rangeOk = isValidRange(start, end)

  // Requesters see the types they can book; everyone else (TU, Kalab, OB, Admin) sees them all.
  const policies = useMemo(() => {
    const all = policiesQuery.data ?? []
    const own = role ? requestablePolicies(all, role) : []
    return own.length > 0 ? own : all
  }, [policiesQuery.data, role])
  const groups = availableGroups(policies)
  const group: TypeGroup | undefined =
    search.group && groups.includes(search.group) ? search.group : groups[0]

  const availability = useAvailability(
    rangeOk
      ? {
          start_at: toApiDateTime(date, start),
          end_at: toApiDateTime(date, end),
        }
      : null
  )
  const rooms = (availability.data ?? []).filter(
    (r) => !group || groupOf(r.room.type) === group
  )
  const free = rooms.filter((r) => r.status === 'TERSEDIA').length
  const selected: AvailabilityRoom | undefined =
    rooms.find((r) => r.room.id === search.room) ??
    rooms.find((r) => r.status === 'TERSEDIA') ??
    rooms[0]

  const update = (patch: Partial<typeof search>) =>
    navigate({
      search: (prev) => ({ date, start, end, ...prev, ...patch }),
      replace: true,
    })

  const times = slotTimes()

  return (
    <Page title='Jadwal Ruang'>
      <div className='grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,22rem)_minmax(0,1fr)]'>
        <section
          aria-label='Cari ruang'
          className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
        >
          <h2 className='type-section text-fg'>Ruang</h2>
          {groups.length > 1 && group && (
            <SegmentedControl
              aria-label='Jenis ruang'
              value={group}
              onValueChange={(g) => update({ group: g, room: undefined })}
              options={groups.map((g) => ({
                value: g,
                label: TYPE_GROUP_LABELS[g],
              }))}
            />
          )}

          <p className='type-overline text-fg-secondary'>Cari ruang kosong</p>
          <div className='grid gap-3'>
            <div className='grid gap-1.5'>
              <Label htmlFor='sc-date'>Tanggal</Label>
              <Input
                id='sc-date'
                type='date'
                value={date}
                min={today}
                onChange={(e) =>
                  e.target.value && update({ date: e.target.value })
                }
              />
            </div>
            <div className='grid grid-cols-2 gap-3'>
              <TimeSelect
                id='sc-start'
                label='Mulai'
                value={start}
                options={times.slice(0, -1)}
                onChange={(v) => update({ start: v })}
              />
              <TimeSelect
                id='sc-end'
                label='Selesai'
                value={end}
                options={times.slice(1)}
                onChange={(v) => update({ end: v })}
              />
            </div>
          </div>

          {!rangeOk && (
            <p className='type-body-sm text-fg-error'>
              Jam selesai harus setelah jam mulai.
            </p>
          )}
          {availability.isError && (
            <Alert variant='error'>
              <AlertCircle />
              <AlertDescription>
                {getErrorMessage(availability.error)}
              </AlertDescription>
            </Alert>
          )}

          <div className='border-t pt-4'>
            {availability.isPending && rangeOk ? (
              <Skeleton className='h-40' />
            ) : rooms.length === 0 ? (
              <p className='type-body text-fg-secondary'>
                Belum ada ruang untuk pilihan ini.
              </p>
            ) : (
              <>
                <p className='mb-3 type-card-title text-fg' aria-live='polite'>
                  {free} dari {rooms.length} ruang{' '}
                  {group ? TYPE_GROUP_LABELS[group].toLowerCase() : ''} kosong
                </p>
                <ul className='flex flex-col gap-1'>
                  {rooms.map((entry) => (
                    <RoomRow
                      key={entry.room.id}
                      entry={entry}
                      active={entry.room.id === selected?.room.id}
                      onPick={() => update({ room: entry.room.id })}
                    />
                  ))}
                </ul>
              </>
            )}
          </div>
        </section>

        <section
          aria-label='Jadwal ruang'
          className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
        >
          {selected ? (
            <RoomWeek
              entry={selected}
              policy={policies.find((p) => p.type === selected.room.type)}
              date={date}
              start={start}
              end={end}
              today={today}
              now={now}
              mobile={isMobile}
              canBook={canBook}
              onDate={(d) => update({ date: d })}
            />
          ) : (
            <EmptyState
              title='Pilih ruang'
              description='Pilih ruang di daftar untuk melihat jadwalnya.'
            />
          )}
        </section>
      </div>
    </Page>
  )
}

function TimeSelect({
  id,
  label,
  value,
  options,
  onChange,
}: {
  id: string
  label: string
  value: string
  options: string[]
  onChange: (value: string) => void
}) {
  return (
    <div className='grid gap-1.5'>
      <Label htmlFor={id}>{label}</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger id={id} className='w-full'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((t) => (
            <SelectItem key={t} value={t}>
              {t}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

function RoomRow({
  entry,
  active,
  onPick,
}: {
  entry: AvailabilityRoom
  active: boolean
  onPick: () => void
}) {
  const taken = entry.status !== 'TERSEDIA'
  return (
    <li>
      <button
        type='button'
        onClick={onPick}
        aria-pressed={active}
        className={cn(
          'flex w-full items-center justify-between gap-3 rounded-xl p-3 text-start transition-colors hover:bg-subtle',
          active && 'bg-subtle'
        )}
      >
        <span className='min-w-0'>
          <span
            className={cn(
              'block truncate type-card-title',
              taken ? 'text-fg-tertiary' : 'text-fg'
            )}
          >
            {entry.room.code}
          </span>
          <span className='type-body-sm text-fg-secondary'>
            {roomTypeConfig(entry.room.type).label} · {entry.room.capacity}{' '}
            orang
          </span>
        </span>
        <StatusPill
          status={AVAILABILITY_PILL[entry.status]}
          label={AVAILABILITY_LABELS[entry.status]}
        />
      </button>
    </li>
  )
}

function RoomWeek({
  entry,
  policy,
  date,
  start,
  end,
  today,
  now,
  mobile,
  canBook,
  onDate,
}: {
  entry: AvailabilityRoom
  policy: RoomTypePolicy | undefined
  date: string
  start: string
  end: string
  today: string
  now: Date
  mobile: boolean
  canBook: boolean
  onDate: (date: string) => void
}) {
  const { room, operating_hours: hours } = entry
  const monday = startOfWeek(date)
  const days = useMemo(
    () => Array.from({ length: 7 }, (_, i) => addDays(monday, i)),
    [monday]
  )
  const schedule = useRoomSchedule(
    room.id,
    toApiDateTime(days[0], '00:00'),
    toApiDateTime(addDays(days[6], 1), '00:00')
  )
  const items = schedule.data ?? []
  const effective = { open: hours.open_time, close: hours.close_time }

  return (
    <>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='min-w-0'>
          <h2 className='type-section text-fg'>{room.code}</h2>
          <p className='type-body text-fg-secondary'>
            {roomTypeConfig(room.type).label} · Kapasitas {room.capacity}
            {room.facilities.length > 0 && ` · ${room.facilities.join(', ')}`}
          </p>
        </div>
        <div className='flex flex-col items-end gap-1'>
          {policy && (
            <p className='type-body-sm text-fg-tertiary'>
              Booking dibuka H-{policy.max_advance_days}
            </p>
          )}
          <Button variant='ghost' size='sm' asChild>
            <Link to='/rooms/$id' params={{ id: room.id }}>
              Detail ruang
            </Link>
          </Button>
        </div>
      </div>

      <div className='flex flex-wrap items-center gap-2'>
        <Button
          variant='secondary'
          size='icon'
          aria-label='Minggu sebelumnya'
          onClick={() => onDate(addDays(date, -7))}
        >
          <ChevronLeft />
        </Button>
        <Button variant='secondary' onClick={() => onDate(today)}>
          Hari ini
        </Button>
        <Button
          variant='secondary'
          size='icon'
          aria-label='Minggu berikutnya'
          onClick={() => onDate(addDays(date, 7))}
        >
          <ChevronRight />
        </Button>
        <p className='type-label text-fg-secondary'>
          {formatDate(toApiDateTime(days[0], '12:00')).replace(/^\S+, /, '')}
          {' – '}
          {formatDate(toApiDateTime(days[6], '12:00')).replace(/^\S+, /, '')}
        </p>
      </div>

      {schedule.isError && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(schedule.error)}</AlertDescription>
        </Alert>
      )}

      {schedule.isPending ? (
        <Skeleton className='h-96' />
      ) : mobile ? (
        <MobileDay
          days={days}
          date={date}
          today={today}
          hours={effective}
          items={items}
          now={now}
          onDate={onDate}
        />
      ) : (
        <WeekGrid
          days={days}
          items={items}
          hours={effective}
          today={today}
          roomId={room.id}
          canBook={canBook && room.is_active}
          now={now}
        />
      )}

      <div className='flex flex-wrap items-center justify-between gap-3'>
        <Legend />
        {canBook && room.is_active && (
          <Button variant='accent' asChild>
            <Link
              to='/bookings/new'
              search={{ room_id: room.id, date, start, end }}
            >
              <CalendarPlus />
              Ajukan Ruang
            </Link>
          </Button>
        )}
      </div>
    </>
  )
}

function MobileDay({
  days,
  date,
  today,
  hours,
  items,
  now,
  onDate,
}: {
  days: string[]
  date: string
  today: string
  hours: { open: string; close: string }
  items: TimelineItem[]
  now: Date
  onDate: (date: string) => void
}) {
  const slots = buildDaySlots({ date, hours, items, now })
  return (
    <div className='flex flex-col gap-4'>
      <DayChips
        days={days}
        value={date}
        onChange={onDate}
        disabledBefore={today}
      />
      <SlotChips slots={slots} range={null} />
    </div>
  )
}

function Legend() {
  return (
    <div className='flex flex-wrap items-center gap-2'>
      <StatusPill status='Tersedia' />
      <StatusPill status='Digunakan' />
      <StatusPill status='Terbatas' label='Sudah lewat' />
    </div>
  )
}
