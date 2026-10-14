import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { formProblemFrom } from '@/lib/api-error'
import { type Room } from '@/lib/api-types'
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
import { ROOM_TYPE_LABELS } from '@/features/rooms/labels'
import { useCreateRoom, useUpdateRoom } from './api'
import {
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
} as const

/** Create (room undefined) or edit a room. Content mounts only while open, so it starts fresh. */
export function RoomFormDialog({
  open,
  room,
  onOpenChange,
}: {
  open: boolean
  room?: Room
  onOpenChange: (open: boolean) => void
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && (
        <RoomFormContent room={room} onDone={() => onOpenChange(false)} />
      )}
    </Dialog>
  )
}

function RoomFormContent({
  room,
  onDone,
}: {
  room?: Room
  onDone: () => void
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
    if (room) {
      update.mutate({
        id: room.id,
        body: { ...common, is_active: v.is_active },
      })
    } else if (orgUnitId) {
      // MVP has one unit; new rooms belong to the admin's own (decisions.md, Phase 5).
      create.mutate({ ...common, org_unit_id: orgUnitId, code: v.code.trim() })
    }
  })

  return (
    <DialogContent className='sm:max-w-lg'>
      <DialogHeader>
        <DialogTitle>{room ? `Ubah ${room.code}` : 'Tambah ruang'}</DialogTitle>
        <DialogDescription>
          {room
            ? 'Kode ruang tidak dapat diubah.'
            : 'Kode disimpan dalam huruf besar dan harus unik.'}
        </DialogDescription>
      </DialogHeader>
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
                      <SelectItem value='KELAS'>
                        {ROOM_TYPE_LABELS.KELAS}
                      </SelectItem>
                      <SelectItem value='LAB'>
                        {ROOM_TYPE_LABELS.LAB}
                      </SelectItem>
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
                <FormItem className='flex items-center justify-between rounded-md border p-3'>
                  <div className='space-y-0.5'>
                    <FormLabel>Aktif</FormLabel>
                    <FormDescription>
                      Ruang nonaktif tidak bisa dibooking dan tidak tampil di
                      ketersediaan.
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
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
      <DialogFooter>
        <Button variant='outline' disabled={pending} onClick={onDone}>
          Batal
        </Button>
        <Button type='submit' form='room-form' disabled={pending}>
          {pending && <Loader2 className='animate-spin' />}
          Simpan
        </Button>
      </DialogFooter>
    </DialogContent>
  )
}
