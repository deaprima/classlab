import { useMemo, useState } from 'react'
import { Link, getRouteApi } from '@tanstack/react-router'
import { slotTimes } from '@/config/booking-rules'
import { AlertCircle, CalendarDays, CalendarPlus, Users } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import {
  type AvailabilityRoom,
  type AvailabilityStatus,
  type RoomType,
} from '@/lib/api-types'
import { formatDate, toApiDateTime } from '@/lib/datetime'
import { ACCESS, useHasRole } from '@/lib/rbac'
import { AVAILABILITY_LABELS } from '@/lib/status-labels'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
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
import { Switch } from '@/components/ui/switch'
import { ConflictList } from '@/components/conflict-list'
import { Page } from '@/components/layout/page'
import { AvailabilityBadge } from '@/components/status-badge'
import { ROOM_TYPE_LABELS, ROOM_TYPES } from '@/features/rooms/labels'
import { useAvailability } from './api'
import { defaultRange, isValidRange } from './range'

const route = getRouteApi('/_authenticated/schedule')

const ALL = 'ALL'

export function Availability() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const canBook = useHasRole(ACCESS.createBooking)
  const [onlyFree, setOnlyFree] = useState(false)

  // Missing search params fall back to "now"; they stay out of the URL until the user changes them.
  const fallback = useMemo(() => defaultRange(), [])
  const date = search.date ?? fallback.date
  const start = search.start ?? fallback.start
  const end = search.end ?? fallback.end
  const rangeOk = isValidRange(start, end)

  const query = rangeOk
    ? {
        start_at: toApiDateTime(date, start),
        end_at: toApiDateTime(date, end),
        type: search.type,
        capacity_min: search.capacity_min,
      }
    : null
  const availability = useAvailability(query)

  const update = (patch: Partial<typeof search>) =>
    navigate({
      search: (prev) => ({ date, start, end, ...prev, ...patch }),
      replace: true,
    })

  const rooms = availability.data ?? []
  const counts = countByStatus(rooms)
  const shown = onlyFree ? rooms.filter((r) => r.status === 'TERSEDIA') : rooms
  const times = slotTimes()

  return (
    <Page
      title='Ketersediaan Ruang'
      description='Cek ruang kelas dan laboratorium yang kosong pada jam tertentu.'
      actions={
        canBook && (
          <Button asChild>
            <Link to='/bookings/new' search={{ date, start, end }}>
              <CalendarPlus />
              Ajukan Peminjaman
            </Link>
          </Button>
        )
      }
    >
      <Card className='mb-6'>
        <CardContent className='grid gap-4 sm:grid-cols-2 lg:grid-cols-5'>
          <div className='grid gap-2'>
            <Label htmlFor='av-date'>Tanggal</Label>
            <Input
              id='av-date'
              type='date'
              value={date}
              onChange={(e) =>
                e.target.value && update({ date: e.target.value })
              }
            />
          </div>
          <TimeSelect
            id='av-start'
            label='Mulai'
            value={start}
            options={times.slice(0, -1)}
            onChange={(v) => update({ start: v })}
          />
          <TimeSelect
            id='av-end'
            label='Selesai'
            value={end}
            options={times.slice(1)}
            onChange={(v) => update({ end: v })}
          />
          <div className='grid gap-2'>
            <Label htmlFor='av-type'>Tipe ruang</Label>
            <Select
              value={search.type ?? ALL}
              onValueChange={(v) =>
                update({ type: v === ALL ? undefined : (v as RoomType) })
              }
            >
              <SelectTrigger id='av-type' className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL}>Semua tipe</SelectItem>
                {ROOM_TYPES.map((t) => (
                  <SelectItem key={t} value={t}>
                    {ROOM_TYPE_LABELS[t]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <CapacityInput
            value={search.capacity_min}
            onCommit={(v) => update({ capacity_min: v })}
          />
        </CardContent>
      </Card>

      {!rangeOk && (
        <Alert variant='destructive' className='mb-6'>
          <AlertCircle />
          <AlertDescription>
            Jam selesai harus setelah jam mulai.
          </AlertDescription>
        </Alert>
      )}

      {rangeOk && (
        <div className='mb-4 flex flex-wrap items-center justify-between gap-3'>
          <p className='text-sm text-muted-foreground' aria-live='polite'>
            {formatDate(toApiDateTime(date, start))}, {start}–{end} WIB
            {availability.isSuccess &&
              ` · ${counts.TERSEDIA} ${AVAILABILITY_LABELS.TERSEDIA.toLowerCase()}, ${counts.DIAJUKAN} ${AVAILABILITY_LABELS.DIAJUKAN.toLowerCase()}, ${counts.DIGUNAKAN} ${AVAILABILITY_LABELS.DIGUNAKAN.toLowerCase()}`}
          </p>
          <div className='flex items-center gap-2'>
            <Switch
              id='av-only-free'
              checked={onlyFree}
              onCheckedChange={setOnlyFree}
            />
            <Label htmlFor='av-only-free'>Hanya yang tersedia</Label>
          </div>
        </div>
      )}

      {availability.isError && (
        <Alert variant='destructive'>
          <AlertCircle />
          <AlertDescription>
            {getErrorMessage(availability.error)}
          </AlertDescription>
        </Alert>
      )}

      {rangeOk && availability.isPending && <CardsSkeleton />}

      {availability.isSuccess && shown.length === 0 && (
        <p className='py-12 text-center text-muted-foreground'>
          {rooms.length === 0
            ? 'Tidak ada ruang yang cocok dengan filter.'
            : 'Tidak ada ruang yang tersedia pada rentang ini.'}
        </p>
      )}

      <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
        {shown.map((item) => (
          <RoomCard
            key={item.room.id}
            item={item}
            canBook={canBook}
            range={{ date, start, end }}
          />
        ))}
      </div>
    </Page>
  )
}

function RoomCard({
  item,
  canBook,
  range,
}: {
  item: AvailabilityRoom
  canBook: boolean
  range: { date: string; start: string; end: string }
}) {
  const { room, status, conflicts } = item
  return (
    <Card className='gap-4'>
      <CardHeader>
        <div className='flex items-start justify-between gap-2'>
          <div className='space-y-1'>
            <CardTitle>
              {room.code} · {room.name}
            </CardTitle>
            <CardDescription className='flex items-center gap-2'>
              {ROOM_TYPE_LABELS[room.type]}
              <span aria-hidden>·</span>
              <Users className='size-3.5' aria-hidden />
              {room.capacity} orang
            </CardDescription>
          </div>
          <AvailabilityBadge status={status} />
        </div>
      </CardHeader>
      <CardContent className='flex-1 space-y-3'>
        {room.facilities.length > 0 && (
          <p className='text-sm text-muted-foreground'>
            {room.facilities.join(', ')}
          </p>
        )}
        <ConflictList items={conflicts} />
      </CardContent>
      <CardFooter className='gap-2'>
        <Button variant='outline' size='sm' asChild>
          <Link
            to='/rooms/$id'
            params={{ id: room.id }}
            search={{ date: range.date }}
          >
            <CalendarDays />
            Lihat jadwal
          </Link>
        </Button>
        {canBook && status === 'TERSEDIA' && (
          <Button size='sm' asChild>
            <Link to='/bookings/new' search={{ room_id: room.id, ...range }}>
              Ajukan
            </Link>
          </Button>
        )}
      </CardFooter>
    </Card>
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
    <div className='grid gap-2'>
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

/** Applies on blur or Enter, so typing "40" does not fire a request for "4" first. */
function CapacityInput({
  value,
  onCommit,
}: {
  value: number | undefined
  onCommit: (value: number | undefined) => void
}) {
  const [draft, setDraft] = useState(value?.toString() ?? '')
  const commit = () => {
    const n = Number(draft)
    onCommit(
      draft.trim() !== '' && Number.isInteger(n) && n > 0 ? n : undefined
    )
  }
  return (
    <div className='grid gap-2'>
      <Label htmlFor='av-capacity'>Kapasitas minimal</Label>
      <Input
        id='av-capacity'
        type='number'
        min={1}
        inputMode='numeric'
        placeholder='Semua'
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => e.key === 'Enter' && commit()}
      />
    </div>
  )
}

function CardsSkeleton() {
  return (
    <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
      {Array.from({ length: 6 }, (_, i) => (
        <Skeleton key={i} className='h-44 rounded-xl' />
      ))}
    </div>
  )
}

function countByStatus(rooms: AvailabilityRoom[]) {
  const counts: Record<AvailabilityStatus, number> = {
    TERSEDIA: 0,
    DIAJUKAN: 0,
    DIGUNAKAN: 0,
  }
  for (const r of rooms) counts[r.status]++
  return counts
}
