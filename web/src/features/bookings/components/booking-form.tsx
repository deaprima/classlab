import { useMemo, useState } from 'react'
import { type Control, useForm, useWatch } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link } from '@tanstack/react-router'
import { slotTimes } from '@/config/booking-rules'
import { AlertCircle, CheckCircle2, Loader2, Send } from 'lucide-react'
import { applyFieldErrors, getApiError, getErrorMessage } from '@/lib/api-error'
import { type Booking } from '@/lib/api-types'
import { toApiDateTime } from '@/lib/datetime'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { type ConflictItem, ConflictList } from '@/components/conflict-list'
import { AvailabilityBadge } from '@/components/status-badge'
import { useAvailability } from '@/features/availability/api'
import { useRooms } from '@/features/rooms/api'
import { ROOM_TYPE_LABELS } from '@/features/rooms/labels'
import { useCreateBooking, useIdempotencyKey } from '../api'
import {
  BOOKING_FIELD_MAP,
  type BookingFormValues,
  NOTES_MAX,
  PURPOSE_MAX,
  createBookingSchema,
  toCreateBookingRequest,
} from '../booking-schema'

type SubmitProblem =
  | { kind: 'conflict'; message: string; items: ConflictItem[] }
  | { kind: 'other'; message: string }

const EMPTY: BookingFormValues = {
  room_id: '',
  date: '',
  start_time: '',
  end_time: '',
  purpose: '',
  participants: '',
  notes: '',
}

export function BookingForm({
  initial,
  onCreated,
}: {
  initial?: Partial<BookingFormValues>
  onCreated: (booking: Booking) => void
}) {
  const schema = useMemo(() => createBookingSchema(), [])
  const form = useForm<BookingFormValues>({
    resolver: zodResolver(schema),
    defaultValues: withDefaults(initial),
  })
  const [problem, setProblem] = useState<SubmitProblem | null>(null)
  const keyFor = useIdempotencyKey()
  const rooms = useRooms({ is_active: true, per_page: 100 })
  const times = slotTimes()

  const create = useCreateBooking({
    onSuccess: onCreated,
    onError: (error) => {
      const apiError = getApiError(error)
      if (apiError?.code === 'BOOKING_CONFLICT') {
        setProblem({
          kind: 'conflict',
          message: apiError.message,
          items: apiError.details,
        })
        return
      }
      const rest = applyFieldErrors(error, form.setError, BOOKING_FIELD_MAP)
      if (!apiError || apiError.details.length === 0 || rest.length > 0) {
        setProblem({
          kind: 'other',
          message:
            rest.find((d) => d.message)?.message ?? getErrorMessage(error),
        })
      }
    },
  })

  function onSubmit(values: BookingFormValues) {
    setProblem(null)
    const body = toCreateBookingRequest(values)
    create.mutate({ body, idempotencyKey: keyFor(body) })
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className='grid gap-6'
        noValidate
      >
        {problem && <ProblemAlert problem={problem} />}

        <FormField
          control={form.control}
          name='room_id'
          render={({ field }) => (
            <FormItem>
              <FormLabel>Ruang</FormLabel>
              <Select
                value={field.value}
                onValueChange={field.onChange}
                disabled={rooms.isPending}
              >
                <FormControl>
                  <SelectTrigger className='w-full'>
                    <SelectValue
                      placeholder={
                        rooms.isPending ? 'Memuat ruang…' : 'Pilih ruang'
                      }
                    />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  {rooms.data?.data.map((room) => (
                    <SelectItem key={room.id} value={room.id}>
                      {room.code} · {room.name} ({ROOM_TYPE_LABELS[room.type]},{' '}
                      {room.capacity} orang)
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {rooms.isError && (
                <p className='text-sm text-destructive'>
                  Daftar ruang gagal dimuat. {getErrorMessage(rooms.error)}
                </p>
              )}
              <FormMessage />
            </FormItem>
          )}
        />

        <div className='grid items-start gap-4 sm:grid-cols-3'>
          <FormField
            control={form.control}
            name='date'
            render={({ field }) => (
              <FormItem>
                <FormLabel>Tanggal</FormLabel>
                <FormControl>
                  <Input type='date' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <TimeField
            control={form.control}
            name='start_time'
            label='Jam mulai'
            options={times.slice(0, -1)}
          />
          <TimeField
            control={form.control}
            name='end_time'
            label='Jam selesai'
            options={times.slice(1)}
          />
        </div>

        <SlotCheck control={form.control} />

        <FormField
          control={form.control}
          name='purpose'
          render={({ field }) => (
            <FormItem>
              <FormLabel>Keperluan</FormLabel>
              <FormControl>
                <Input
                  placeholder='Contoh: Latihan kelompok Manajemen Proyek'
                  maxLength={PURPOSE_MAX}
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <div className='grid items-start gap-4 sm:grid-cols-3'>
          <FormField
            control={form.control}
            name='participants'
            render={({ field }) => (
              <FormItem>
                <FormLabel>Jumlah peserta (opsional)</FormLabel>
                <FormControl>
                  <Input type='number' min={1} inputMode='numeric' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='notes'
            render={({ field }) => (
              <FormItem className='sm:col-span-2'>
                <FormLabel>Catatan (opsional)</FormLabel>
                <FormControl>
                  <Textarea
                    rows={2}
                    placeholder='Contoh: Butuh proyektor'
                    maxLength={NOTES_MAX}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  Pengajuan menahan slot sampai diputuskan TU/Kepala Lab.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        <div className='flex flex-wrap justify-end gap-2'>
          <Button variant='outline' type='button' asChild>
            <Link to='/'>Batal</Link>
          </Button>
          <Button type='submit' disabled={create.isPending}>
            {create.isPending ? <Loader2 className='animate-spin' /> : <Send />}
            Kirim Pengajuan
          </Button>
        </div>
      </form>
    </Form>
  )
}

function TimeField({
  control,
  name,
  label,
  options,
}: {
  control: Control<BookingFormValues>
  name: 'start_time' | 'end_time'
  label: string
  options: string[]
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <Select value={field.value} onValueChange={field.onChange}>
            <FormControl>
              <SelectTrigger className='w-full'>
                <SelectValue placeholder='--:--' />
              </SelectTrigger>
            </FormControl>
            <SelectContent>
              {options.map((t) => (
                <SelectItem key={t} value={t}>
                  {t}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

/** Live answer to "is this room free then?", before the user submits. Advisory only. */
function SlotCheck({ control }: { control: Control<BookingFormValues> }) {
  const [roomId, date, start, end] = useWatch({
    control,
    name: ['room_id', 'date', 'start_time', 'end_time'],
  })
  const ready = !!roomId && !!date && !!start && !!end && end > start
  const availability = useAvailability(
    ready
      ? {
          start_at: toApiDateTime(date, start),
          end_at: toApiDateTime(date, end),
        }
      : null
  )
  if (!ready) return null

  const entry = availability.data?.find((r) => r.room.id === roomId)
  return (
    <div aria-live='polite' className='min-h-6'>
      {availability.isFetching && !entry && (
        <p className='text-sm text-muted-foreground'>Memeriksa ketersediaan…</p>
      )}
      {entry?.status === 'TERSEDIA' && (
        <p className='flex items-center gap-2 text-sm text-emerald-700 dark:text-emerald-400'>
          <CheckCircle2 className='size-4' />
          Ruang tersedia pada jam ini.
        </p>
      )}
      {entry && entry.status !== 'TERSEDIA' && (
        <Alert>
          <AlertCircle />
          <AlertTitle className='flex items-center gap-2'>
            Slot ini tidak kosong <AvailabilityBadge status={entry.status} />
          </AlertTitle>
          <AlertDescription>
            <ConflictList items={entry.conflicts} />
            <p>Pilih jam atau ruang lain agar pengajuan tidak ditolak.</p>
          </AlertDescription>
        </Alert>
      )}
    </div>
  )
}

function ProblemAlert({ problem }: { problem: SubmitProblem }) {
  return (
    <Alert variant='destructive' role='alert'>
      <AlertCircle />
      <AlertTitle>
        {problem.kind === 'conflict' ? 'Jadwal bentrok' : 'Pengajuan gagal'}
      </AlertTitle>
      <AlertDescription>
        <p>{problem.message}</p>
        {problem.kind === 'conflict' && (
          <>
            <ConflictList items={problem.items} withDate />
            <p>Pilih jam atau ruang lain lalu kirim ulang.</p>
          </>
        )}
      </AlertDescription>
    </Alert>
  )
}

/** Prefill from the URL; keys the URL did not carry keep their empty default. */
function withDefaults(
  initial: Partial<BookingFormValues> | undefined
): BookingFormValues {
  const out = { ...EMPTY }
  for (const [key, value] of Object.entries(initial ?? {})) {
    if (typeof value === 'string') out[key as keyof BookingFormValues] = value
  }
  return out
}
