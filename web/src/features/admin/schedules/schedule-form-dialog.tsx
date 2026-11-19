import { useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { formProblemFrom } from '@/lib/api-error'
import { type Room, type TimelineItem } from '@/lib/api-types'
import { addDays, toApiDateTime, toWibDate, toWibTime } from '@/lib/datetime'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
import { type FormProblem, FormProblemAlert } from '@/components/form-problem'
import { useCreateSchedule, useUpdateSchedule } from './api'
import {
  MAX_SESSIONS,
  SCHEDULE_FIELD_MAP,
  type ScheduleFormValues,
  scheduleFormSchema,
  sessionCount,
} from './schedule-schema'

export type ScheduleDialogState =
  | { mode: 'create'; roomId: string; date: string }
  | { mode: 'edit'; roomId: string; item: TimelineItem }
  | null

export function ScheduleFormDialog({
  state,
  rooms,
  onClose,
}: {
  state: ScheduleDialogState
  rooms: Room[]
  onClose: () => void
}) {
  return (
    <Dialog open={state !== null} onOpenChange={(o) => !o && onClose()}>
      {state && (
        <ScheduleFormContent state={state} rooms={rooms} onDone={onClose} />
      )}
    </Dialog>
  )
}

function ScheduleFormContent({
  state,
  rooms,
  onDone,
}: {
  state: NonNullable<ScheduleDialogState>
  rooms: Room[]
  onDone: () => void
}) {
  const editing = state.mode === 'edit' ? state.item : undefined
  const date = editing
    ? toWibDate(editing.start_at)
    : state.mode === 'create'
      ? state.date
      : ''
  const [problem, setProblem] = useState<FormProblem | null>(null)
  const form = useForm<ScheduleFormValues>({
    resolver: zodResolver(scheduleFormSchema),
    defaultValues: {
      room_id: state.roomId,
      title: editing?.title ?? '',
      date,
      start_time: editing ? toWibTime(editing.start_at) : '07:30',
      end_time: editing ? toWibTime(editing.end_at) : '10:00',
      source: 'AKADEMIK',
      repeat: false,
      // One semester of weekly classes by default.
      until: addDays(date, 15 * 7),
    },
  })
  const onError = (e: unknown) =>
    setProblem(formProblemFrom(e, form.setError, SCHEDULE_FIELD_MAP))
  const create = useCreateSchedule({ onSuccess: onDone, onError })
  const update = useUpdateSchedule({ onSuccess: onDone, onError })
  const pending = create.isPending || update.isPending
  const [repeat, startDate, until] = useWatch({
    control: form.control,
    name: ['repeat', 'date', 'until'],
  })
  const sessions = repeat ? sessionCount(startDate, until) : 1

  const submit = form.handleSubmit((v) => {
    setProblem(null)
    const times = {
      start_at: toApiDateTime(v.date, v.start_time),
      end_at: toApiDateTime(v.date, v.end_time),
    }
    if (editing) {
      update.mutate({
        id: editing.id,
        body: { title: v.title.trim(), ...times },
      })
      return
    }
    create.mutate({
      room_id: v.room_id,
      title: v.title.trim(),
      source: v.source,
      ...times,
      ...(v.repeat && { repeat: { freq: 'WEEKLY', until: v.until } }),
    })
  })

  return (
    <DialogContent className='max-h-[90dvh] overflow-y-auto sm:max-w-lg'>
      <DialogHeader>
        <DialogTitle>{editing ? 'Ubah jadwal' : 'Tambah jadwal'}</DialogTitle>
        <DialogDescription>
          {editing
            ? 'Mengubah satu sesi ini saja. Ruang tidak dapat dipindah.'
            : 'Jadwal kuliah atau kegiatan tetap. Bentrok dengan jadwal atau peminjaman lain akan ditolak.'}
        </DialogDescription>
      </DialogHeader>
      <Form {...form}>
        <form
          id='schedule-form'
          noValidate
          className='grid gap-4'
          onSubmit={submit}
        >
          <FormProblemAlert problem={problem} />
          <FormField
            control={form.control}
            name='room_id'
            render={({ field }) => (
              <FormItem>
                <FormLabel>Ruang</FormLabel>
                <Select
                  value={field.value}
                  onValueChange={field.onChange}
                  disabled={!!editing}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue placeholder='Pilih ruang' />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {rooms.map((room) => (
                      <SelectItem key={room.id} value={room.id}>
                        {room.code} · {room.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='title'
            render={({ field }) => (
              <FormItem>
                <FormLabel>Judul</FormLabel>
                <FormControl>
                  <Input
                    placeholder='Basis Data (Praktikum) - Kelas A'
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
            <FormField
              control={form.control}
              name='start_time'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Mulai</FormLabel>
                  <FormControl>
                    <Input type='time' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='end_time'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Selesai</FormLabel>
                  <FormControl>
                    <Input type='time' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
          {!editing && (
            <>
              <FormField
                control={form.control}
                name='source'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Sumber</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger className='w-full'>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='AKADEMIK'>
                          Jadwal akademik
                        </SelectItem>
                        <SelectItem value='MANUAL'>Kegiatan lain</SelectItem>
                      </SelectContent>
                    </Select>
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='repeat'
                render={({ field }) => (
                  <FormItem className='flex items-center gap-2'>
                    <FormControl>
                      <Checkbox
                        checked={field.value}
                        onCheckedChange={(c) => field.onChange(c === true)}
                      />
                    </FormControl>
                    <FormLabel className='!mt-0'>
                      Ulangi setiap minggu
                    </FormLabel>
                  </FormItem>
                )}
              />
              {repeat && (
                <FormField
                  control={form.control}
                  name='until'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Sampai tanggal</FormLabel>
                      <FormControl>
                        <Input type='date' {...field} />
                      </FormControl>
                      <FormDescription>
                        {sessions > 0
                          ? `${sessions} sesi akan dibuat (maksimal ${MAX_SESSIONS}). Bila satu sesi bentrok, tidak ada yang dibuat.`
                          : `Maksimal ${MAX_SESSIONS} sesi.`}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
            </>
          )}
        </form>
      </Form>
      <DialogFooter>
        <Button variant='outline' disabled={pending} onClick={onDone}>
          Batal
        </Button>
        <Button type='submit' form='schedule-form' disabled={pending}>
          {pending && <Loader2 className='animate-spin' />}
          {editing
            ? 'Simpan'
            : sessions > 1
              ? `Buat ${sessions} sesi`
              : 'Buat jadwal'}
        </Button>
      </DialogFooter>
    </DialogContent>
  )
}
