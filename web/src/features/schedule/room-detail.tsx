import { useMemo, useState } from 'react'
import { Link, getRouteApi } from '@tanstack/react-router'
import { slotTimes } from '@/config/booking-rules'
import { AlertCircle, ArrowLeft, ImageIcon } from 'lucide-react'
import { getErrorMessage, isApiErrorCode } from '@/lib/api-error'
import { type RoomTypePolicy } from '@/lib/api-types'
import {
  addDays,
  formatDate,
  minutesOfDay,
  toApiDateTime,
  todayWib,
} from '@/lib/datetime'
import { ACCESS, useHasRole } from '@/lib/rbac'
import { useNow } from '@/hooks/use-now'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Page } from '@/components/layout/page'
import { StatusPill } from '@/components/status-pill'
import { effectiveHours } from '@/features/bookings/request/policy'
import { RequestDialog } from '@/features/bookings/request/request-dialog'
import { useRoomTypes } from '@/features/room-types/api'
import { roomTypeConfig } from '@/features/room-types/config'
import { useRoom, useRoomSchedule } from '@/features/rooms/api'
import { DayChips, SlotChips } from './day-slots'
import { buildDaySlots, pickSlot } from './slots'

const route = getRouteApi('/_authenticated/rooms/$id')

const DAYS_SHOWN = 8

/** Detail Ruang (Figma): the room's facts, its free slots for the coming days, and a request panel. */
export function RoomDetail() {
  const { id } = route.useParams()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const room = useRoom(id)
  const policies = useRoomTypes()
  const now = useNow()
  const today = todayWib(now)
  const canBook = useHasRole(ACCESS.createBooking)

  const policy = policies.data?.find((p) => p.type === room.data?.type)
  const days = useMemo(
    () => Array.from({ length: DAYS_SHOWN }, (_, i) => addDays(today, i)),
    [today]
  )
  const date = search.date && days.includes(search.date) ? search.date : today
  const schedule = useRoomSchedule(
    id,
    toApiDateTime(days[0], '00:00'),
    toApiDateTime(addDays(days[DAYS_SHOWN - 1], 1), '00:00')
  )
  const [range, setRange] = useState<{ start: string; end: string } | null>(
    null
  )
  const [requestOpen, setRequestOpen] = useState(false)

  if (room.isError) {
    return (
      <Page title='Detail Ruang'>
        <Alert variant='error'>
          <AlertCircle />
          <AlertDescription>
            {isApiErrorCode(room.error, 'NOT_FOUND')
              ? 'Ruang tidak ditemukan atau sudah dihapus.'
              : getErrorMessage(room.error)}
          </AlertDescription>
        </Alert>
        <Button variant='secondary' className='mt-4' asChild>
          <Link to='/schedule'>
            <ArrowLeft />
            Kembali ke jadwal ruang
          </Link>
        </Button>
      </Page>
    )
  }
  if (room.isPending || policies.isPending) {
    return (
      <Page title='Detail Ruang'>
        <Skeleton className='h-96 rounded-2xl' />
      </Page>
    )
  }

  const r = room.data
  const hours = policy
    ? effectiveHours(r, policy)
    : { open: r.open_time ?? '07:00', close: r.close_time ?? '21:00' }
  const items = schedule.data ?? []
  const slots = buildDaySlots({ date, hours, items, now })
  const occupiedNow = items.some(
    (i) =>
      Date.parse(i.start_at) <= now.getTime() &&
      Date.parse(i.end_at) > now.getTime()
  )

  const setDate = (d: string) => {
    setRange(null)
    navigate({ search: { date: d }, replace: true })
  }

  return (
    <Page title='Detail Ruang'>
      <div className='grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_340px]'>
        <div className='flex min-w-0 flex-col gap-5'>
          <section
            aria-label='Info ruang'
            className='flex flex-col gap-4 rounded-2xl border bg-card p-5'
          >
            <h2 className='type-section text-fg'>Info Ruang</h2>
            <div className='grid gap-5 sm:grid-cols-[15rem_1fr]'>
              <div className='flex h-36 flex-col items-center justify-center gap-2 rounded-xl bg-subtle text-fg-secondary'>
                <ImageIcon aria-hidden className='size-5' />
                <span className='type-body-sm'>Foto ruangan</span>
              </div>
              <div className='flex min-w-0 flex-col gap-3'>
                <div className='flex items-start justify-between gap-3'>
                  <div className='min-w-0'>
                    <h3 className='truncate type-page-title text-fg'>
                      {r.code}
                    </h3>
                    <p className='type-body text-fg-secondary'>
                      {roomTypeConfig(r.type).label}
                      {r.name !== r.code && ` · ${r.name}`}
                    </p>
                  </div>
                  {r.is_active ? (
                    <StatusPill
                      status={occupiedNow ? 'Digunakan' : 'Tersedia'}
                    />
                  ) : (
                    <StatusPill status='Terbatas' label='Nonaktif' />
                  )}
                </div>
                <dl className='flex flex-wrap gap-x-8 gap-y-2'>
                  <Fact label='Kapasitas' value={`${r.capacity} orang`} />
                  <Fact
                    label='Jam operasional'
                    value={`${hours.open} – ${hours.close}`}
                  />
                </dl>
                {r.facilities.length > 0 && (
                  <ul className='flex flex-wrap gap-2'>
                    {r.facilities.map((f) => (
                      <li
                        key={f}
                        className='rounded-full border px-3 py-1 type-label text-fg'
                      >
                        {f}
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
          </section>

          <section
            aria-label='Ketersediaan'
            className='flex flex-col gap-4 rounded-2xl border bg-card p-5'
          >
            <h2 className='type-section text-fg'>Ketersediaan</h2>
            <DayChips days={days} value={date} onChange={setDate} />
            {schedule.isError && (
              <Alert variant='error'>
                <AlertCircle />
                <AlertDescription>
                  {getErrorMessage(schedule.error)}
                </AlertDescription>
              </Alert>
            )}
            {schedule.isPending ? (
              <Skeleton className='h-24' />
            ) : (
              <SlotChips
                slots={slots}
                range={range}
                onPick={
                  canBook && r.is_active
                    ? (time) => setRange((cur) => pickSlot(slots, cur, time))
                    : undefined
                }
              />
            )}
            <div className='flex flex-wrap gap-2'>
              <StatusPill status='Tersedia' />
              <StatusPill status='Digunakan' />
              <StatusPill status='Terbatas' label='Sudah lewat' />
            </div>
          </section>
        </div>

        {canBook && r.is_active && (
          <RequestPanel
            policy={policy}
            date={date}
            days={days}
            hours={hours}
            range={range}
            onDate={setDate}
            onRange={setRange}
            onRequest={() => setRequestOpen(true)}
          />
        )}
      </div>

      {canBook && r.is_active && (
        <RequestDialog
          open={requestOpen}
          onOpenChange={setRequestOpen}
          room={r}
          occurrences={[
            {
              date,
              start_time: range?.start ?? '',
              end_time: range?.end ?? '',
            },
          ]}
          onCreated={() => setRange(null)}
        />
      )}
    </Page>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className='type-body-sm text-fg-secondary'>{label}</dt>
      <dd className='type-card-title text-fg'>{value}</dd>
    </div>
  )
}

function RequestPanel({
  policy,
  date,
  days,
  hours,
  range,
  onDate,
  onRange,
  onRequest,
}: {
  policy: RoomTypePolicy | undefined
  date: string
  days: string[]
  hours: { open: string; close: string }
  range: { start: string; end: string } | null
  onDate: (date: string) => void
  onRange: (range: { start: string; end: string } | null) => void
  onRequest: () => void
}) {
  const times = slotTimes(hours.open, hours.close)
  const start = range?.start ?? ''
  const end = range?.end ?? ''
  const startOptions = times.slice(0, -1)
  const endOptions = times
    .slice(1)
    .filter((t) => !start || minutesOfDay(t) > minutesOfDay(start))

  return (
    <aside
      aria-label='Ajukan ruang ini'
      className='flex h-fit flex-col gap-4 rounded-2xl border bg-card p-5'
    >
      <h2 className='type-section text-fg'>Ajukan Ruang Ini</h2>
      <div className='grid gap-1.5'>
        <Label htmlFor='rd-date'>Tanggal</Label>
        <Select value={date} onValueChange={onDate}>
          <SelectTrigger id='rd-date' className='w-full'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {days.map((d) => (
              <SelectItem key={d} value={d}>
                {formatDate(toApiDateTime(d, '12:00'))}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className='grid gap-1.5'>
        <Label htmlFor='rd-start'>Mulai</Label>
        <Select
          value={start}
          onValueChange={(v) =>
            onRange({
              start: v,
              end: end && minutesOfDay(end) > minutesOfDay(v) ? end : '',
            })
          }
        >
          <SelectTrigger id='rd-start' className='w-full'>
            <SelectValue placeholder='--:--' />
          </SelectTrigger>
          <SelectContent>
            {startOptions.map((t) => (
              <SelectItem key={t} value={t}>
                {t}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className='grid gap-1.5'>
        <Label htmlFor='rd-end'>Selesai</Label>
        <Select
          value={end}
          disabled={!start}
          onValueChange={(v) => onRange({ start, end: v })}
        >
          <SelectTrigger id='rd-end' className='w-full'>
            <SelectValue placeholder='--:--' />
          </SelectTrigger>
          <SelectContent>
            {endOptions.map((t) => (
              <SelectItem key={t} value={t}>
                {t}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {policy && (
        <p className='type-body-sm text-fg-secondary'>
          Pengajuan dibuka maksimal {policy.max_advance_days} hari ke depan.
        </p>
      )}
      <Button variant='accent' onClick={onRequest}>
        Ajukan Ruang
      </Button>
    </aside>
  )
}
