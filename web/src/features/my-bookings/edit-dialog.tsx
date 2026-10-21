import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { bookingChangeRules } from '@/config/attendance-rules'
import { slotTimes } from '@/config/booking-rules'
import { AlertCircle, Loader2, TriangleAlert } from 'lucide-react'
import { toast } from 'sonner'
import { formProblemFrom } from '@/lib/api-error'
import { type Booking, type RoomTypePolicy } from '@/lib/api-types'
import { toWibDate } from '@/lib/datetime'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
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
import { ConflictList, type ConflictItem } from '@/components/conflict-list'
import { useEditBooking } from '@/features/bookings/api'
import {
  effectiveHours,
  participantsRequired,
} from '@/features/bookings/request/policy'
import {
  lastBookableDate,
  NOTES_MAX,
  PURPOSE_MAX,
  SLOT_MINUTES,
} from '@/features/bookings/request/request-schema'
import { useRoomTypes } from '@/features/room-types/api'
import {
  createEditSchema,
  EDIT_FIELD_MAP,
  editDefaults,
  toEditRequest,
  type EditFormValues,
} from './edit-schema'
import { mayNeedReapproval } from './rules'

type EditDialogProps = {
  /** The booking to change; null keeps the dialog closed. */
  booking: Booking | null
  onOpenChange: (open: boolean) => void
}

type Problem = { message: string; conflicts?: ConflictItem[] }

/** "Ubah pengajuan" (Figma): the date, the times and the details; the room cannot change (BR-26). */
export function EditDialog({ booking, onOpenChange }: EditDialogProps) {
  return (
    <Dialog open={!!booking} onOpenChange={onOpenChange}>
      <DialogContent
        className='sm:max-w-lg'
        // Not the date field: on a phone the keyboard would open before anything is read.
        onOpenAutoFocus={(e) => e.preventDefault()}
      >
        {booking && (
          <EditForm
            key={booking.id}
            booking={booking}
            onClose={() => onOpenChange(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

function EditForm({
  booking,
  onClose,
}: {
  booking: Booking
  onClose: () => void
}) {
  const policies = useRoomTypes()
  const policy: RoomTypePolicy | undefined = policies.data?.find(
    (p) => p.type === booking.room.type
  )
  const [problem, setProblem] = useState<Problem | null>(null)

  // The schema needs the policy, which may load after the form exists: the resolver reads it lazily.
  const policyRef = useRef(policy)
  useEffect(() => {
    policyRef.current = policy
  })
  const form = useForm<EditFormValues>({
    resolver: (values, context, options) =>
      zodResolver(
        createEditSchema({ room: booking.room, policy: policyRef.current })
      )(values, context, options),
    defaultValues: editDefaults(booking),
  })

  const edit = useEditBooking({
    onSuccess: (updated) => {
      toast.success(
        updated.status === 'MENUNGGU' && booking.status === 'DISETUJUI'
          ? 'Pengajuan diubah dan menunggu persetujuan ulang.'
          : 'Pengajuan diubah.'
      )
      onClose()
    },
    onError: (error) => {
      const found = formProblemFrom(error, form.setError, EDIT_FIELD_MAP)
      if (found) {
        setProblem({
          message: found.message,
          conflicts: found.conflicts as ConflictItem[] | undefined,
        })
      }
    },
  })

  function onSubmit(values: EditFormValues) {
    setProblem(null)
    const body = toEditRequest(booking, values)
    if (!body) {
      toast.info('Tidak ada perubahan.')
      onClose()
      return
    }
    edit.mutate({ id: booking.id, body })
  }

  const hours = policy
    ? effectiveHours(booking.room, policy)
    : {
        open: booking.room.open_time ?? '07:00',
        close: booking.room.close_time ?? '21:00',
      }
  const times = slotTimes(hours.open, hours.close, SLOT_MINUTES)
  const now = new Date()

  return (
    <>
      <DialogHeader>
        <DialogTitle>Ubah pengajuan</DialogTitle>
        <DialogDescription>
          Bisa diubah sampai {bookingChangeRules.editCutoffMinutes} menit
          sebelum mulai. Ruang tidak bisa diganti.
        </DialogDescription>
      </DialogHeader>

      {mayNeedReapproval(booking) && (
        <Alert variant='warning'>
          <TriangleAlert />
          <AlertTitle>Perlu persetujuan ulang</AlertTitle>
          <AlertDescription>
            Perubahan waktu atau peserta bisa mengembalikan status ke Menunggu
            dan melepas slot lama.
          </AlertDescription>
        </Alert>
      )}
      {problem && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertTitle>{problem.message}</AlertTitle>
          {problem.conflicts && problem.conflicts.length > 0 && (
            <AlertDescription>
              <ConflictList items={problem.conflicts} withDate />
            </AlertDescription>
          )}
        </Alert>
      )}

      <Form {...form}>
        <form
          id='edit-booking-form'
          onSubmit={form.handleSubmit(onSubmit)}
          className='grid gap-4'
          noValidate
        >
          <div className='grid grid-cols-2 items-start gap-3 sm:grid-cols-[minmax(0,1fr)_7.5rem_7.5rem]'>
            <FormField
              control={form.control}
              name='date'
              render={({ field }) => (
                <FormItem className='col-span-2 sm:col-span-1'>
                  <FormLabel>Tanggal</FormLabel>
                  <FormControl>
                    <Input
                      type='date'
                      min={toWibDate(now)}
                      max={policy ? lastBookableDate(policy, now) : undefined}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <TimeField
              name='start_time'
              label='Mulai'
              options={times.slice(0, -1)}
              form={form}
            />
            <TimeField
              name='end_time'
              label='Selesai'
              options={times.slice(1)}
              form={form}
            />
          </div>
          <div className='grid items-start gap-3 sm:grid-cols-[minmax(0,1fr)_10rem]'>
            <FormField
              control={form.control}
              name='purpose'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Nama kegiatan</FormLabel>
                  <FormControl>
                    <Input maxLength={PURPOSE_MAX} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='participants'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    Jumlah peserta
                    {participantsRequired(booking.room.type) ? ' (wajib)' : ''}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      inputMode='numeric'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
          <FormField
            control={form.control}
            name='notes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>Catatan (opsional)</FormLabel>
                <FormControl>
                  <Textarea rows={2} maxLength={NOTES_MAX} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>

      <DialogFooter>
        <Button type='button' variant='secondary' onClick={onClose}>
          Batal
        </Button>
        <Button
          type='submit'
          form='edit-booking-form'
          disabled={edit.isPending}
        >
          {edit.isPending && <Loader2 className='animate-spin' />}
          Simpan
        </Button>
      </DialogFooter>
    </>
  )
}

function TimeField({
  name,
  label,
  options,
  form,
}: {
  name: 'start_time' | 'end_time'
  label: string
  options: string[]
  form: ReturnType<typeof useForm<EditFormValues>>
}) {
  return (
    <FormField
      control={form.control}
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
