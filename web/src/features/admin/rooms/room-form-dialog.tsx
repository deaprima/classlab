import { type ReactNode, useState } from 'react'
import { type UseFormReturn, useForm, useWatch } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { formProblemFrom } from '@/lib/api-error'
import { type Room, type RoomType } from '@/lib/api-types'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { type FormProblem, FormProblemAlert } from '@/components/form-problem'
import { ROOM_TYPE_LABELS, ROOM_TYPES } from '@/features/rooms/labels'
import { useCreateRoom, useUpdateRoom } from './api'
import {
  HOUR_OPTIONS,
  type RoomFormValues,
  parseFacilities,
  roomFormSchema,
} from './room-schema'

const FIELDS = {
  code: 'code',
  name: 'name',
  type: 'type',
  capacity: 'capacity',
  facilities: 'facilities',
  is_active: 'is_active',
  open_time: 'open_time',
  close_time: 'close_time',
} as const

const DEFAULT_HOURS = 'DEFAULT'

/** Create (room undefined) or edit a room. Content mounts only while open, so it starts fresh. */
export function RoomFormDialog({
  open,
  room,
  onOpenChange,
  defaultHours,
}: {
  open: boolean
  room?: Room
  onOpenChange: (open: boolean) => void
  /** Hours of the room type, shown as the hint under "ikut tipe". */
  defaultHours?: (type: RoomType) => string | undefined
}) {
  const done = () => onOpenChange(false)
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && (
        <DialogContent className='max-h-[90dvh] overflow-y-auto sm:max-w-lg'>
          <DialogHeader>
            <DialogTitle>
              {room ? `Ubah ${room.code}` : 'Tambah ruang'}
            </DialogTitle>
            <DialogDescription>
              {room
                ? 'Kode ruang tidak dapat diubah.'
                : 'Ruang baru langsung muncul di Jadwal Ruang. QR dibuat otomatis.'}
            </DialogDescription>
          </DialogHeader>
          <RoomForm
            room={room}
            onDone={done}
            onCancel={done}
            defaultHours={defaultHours}
          />
        </DialogContent>
      )}
    </Dialog>
  )
}

/** The room form with its own Batal and Simpan buttons; used in the dialog and in the side panel. */
export function RoomForm({
  room,
  onDone,
  onCancel,
  defaultHours,
  extra,
}: {
  room?: Room
  onDone: () => void
  onCancel: () => void
  defaultHours?: (type: RoomType) => string | undefined
  /** Shown above the buttons (the QR card of the side panel). */
  extra?: ReactNode
}) {
  const orgUnitId = useAuthStore((s) => s.auth.user?.org_unit_id)
  const [problem, setProblem] = useState<FormProblem | null>(null)
  const form = useForm<RoomFormValues>({
    resolver: zodResolver(roomFormSchema),
    defaultValues: {
      code: room?.code ?? '',
      name: room?.name ?? '',
      type: room?.type ?? 'KELAS',
      capacity: room ? String(room.capacity) : '',
      facilities: room?.facilities.join('\n') ?? '',
      is_active: room?.is_active ?? true,
      open_time: room?.open_time ?? '',
      close_time: room?.close_time ?? '',
    },
  })
  const onError = (e: unknown) =>
    setProblem(formProblemFrom(e, form.setError, FIELDS))
  const create = useCreateRoom({ onSuccess: onDone, onError })
  const update = useUpdateRoom({ onSuccess: onDone, onError })
  const pending = create.isPending || update.isPending

  const submit = form.handleSubmit((v) => {
    setProblem(null)
    const common = {
      name: v.name.trim(),
      type: v.type,
      capacity: Number(v.capacity),
      facilities: parseFacilities(v.facilities),
    }
    const hours =
      v.open_time && v.close_time
        ? { open_time: v.open_time, close_time: v.close_time }
        : null
    if (room) {
      update.mutate({
        id: room.id,
        body: {
          ...common,
          is_active: v.is_active,
          ...(hours ?? (room.open_time ? { clear_operating_hours: true } : {})),
        },
      })
    } else if (orgUnitId) {
      // MVP has one unit; new rooms belong to the admin's own (decisions.md, Phase 5).
      create.mutate({
        ...common,
        ...hours,
        org_unit_id: orgUnitId,
        code: v.code.trim(),
      })
    }
  })

  const type = useWatch({ control: form.control, name: 'type' })
  const fallback = defaultHours?.(type)

  return (
    <>
      <Form {...form}>
        <form
          id='room-form'
          noValidate
          className='grid gap-4'
          onSubmit={submit}
        >
          <FormProblemAlert problem={problem} />
          <div className='grid gap-4 sm:grid-cols-[8rem_1fr]'>
            <FormField
              control={form.control}
              name='code'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Kode</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='LAB-4'
                      disabled={!!room}
                      className='uppercase'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Nama</FormLabel>
                  <FormControl>
                    <Input placeholder='Lab Kecerdasan Buatan' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='type'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Tipe</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {ROOM_TYPES.map((t) => (
                        <SelectItem key={t} value={t}>
                          {ROOM_TYPE_LABELS[t]}
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
              name='capacity'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Kapasitas</FormLabel>
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
          <div className='grid gap-4 sm:grid-cols-2'>
            <HoursField form={form} name='open_time' label='Buka (khusus)' />
            <HoursField form={form} name='close_time' label='Tutup (khusus)' />
          </div>
          <p className='-mt-2 type-body-sm text-fg-secondary'>
            Kosong = ikut jam default tipe ruang
            {fallback ? ` (${fallback})` : ''}.
          </p>
          <FormField
            control={form.control}
            name='facilities'
            render={({ field }) => (
              <FormItem>
                <FormLabel>Fasilitas</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder={'Proyektor\nAC\n30 PC'}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  Satu per baris atau pisahkan dengan koma.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          {room && (
            <FormField
              control={form.control}
              name='is_active'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4 rounded-md border p-3'>
                  <div className='min-w-0 flex-1 space-y-0.5'>
                    <FormLabel>Aktif</FormLabel>
                    <FormDescription>
                      Ruang nonaktif tidak bisa dibooking dan tidak tampil di
                      ketersediaan.
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      className='shrink-0'
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          )}
        </form>
      </Form>
      {extra}
      <div className='flex flex-col-reverse gap-2 sm:flex-row sm:justify-end'>
        <Button variant='outline' disabled={pending} onClick={onCancel}>
          Batal
        </Button>
        <Button type='submit' form='room-form' disabled={pending}>
          {pending && <Loader2 className='animate-spin' />}
          Simpan
        </Button>
      </div>
    </>
  )
}

function HoursField({
  form,
  name,
  label,
}: {
  form: UseFormReturn<RoomFormValues>
  name: 'open_time' | 'close_time'
  label: string
}) {
  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <Select
            value={field.value || DEFAULT_HOURS}
            onValueChange={(v) => field.onChange(v === DEFAULT_HOURS ? '' : v)}
          >
            <FormControl>
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
            </FormControl>
            <SelectContent>
              <SelectItem value={DEFAULT_HOURS}>Ikut tipe</SelectItem>
              {HOUR_OPTIONS.map((h) => (
                <SelectItem key={h} value={h}>
                  {h}
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
