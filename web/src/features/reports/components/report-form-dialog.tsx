import { useState } from 'react'
import { type Control, useForm, useWatch } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { AlertCircle, ImagePlus, Loader2, X } from 'lucide-react'
import { toast } from 'sonner'
import { applyFieldErrors, getApiError, getErrorMessage } from '@/lib/api-error'
import { type DamageReport } from '@/lib/api-types'
import { useDataUrl } from '@/hooks/use-data-url'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { Textarea } from '@/components/ui/textarea'
import { useRooms } from '@/features/rooms/api'
import { useCreateReport } from '../api'
import {
  DESCRIPTION_MAX,
  PHOTO_TYPES,
  type ReportFormValues,
  reportFormSchema,
} from '../report-rules'

const EMPTY: ReportFormValues = {
  room_id: '',
  description: '',
  photo: undefined,
}

export function ReportFormDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: (report: DamageReport) => void
}) {
  // The content mounts only while open, so every opening starts with an empty form.
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && (
        <ReportFormContent
          onCancel={() => onOpenChange(false)}
          onCreated={onCreated}
        />
      )}
    </Dialog>
  )
}

function ReportFormContent({
  onCancel,
  onCreated,
}: {
  onCancel: () => void
  onCreated: (report: DamageReport) => void
}) {
  const form = useForm<ReportFormValues>({
    resolver: zodResolver(reportFormSchema),
    defaultValues: EMPTY,
  })
  const [problem, setProblem] = useState<string | null>(null)
  // Inactive rooms can still be reported (decisions.md, Phase 4); deleted ones are not listed.
  const rooms = useRooms({ per_page: 100 })

  const create = useCreateReport({
    onSuccess: (report) => {
      toast.success(
        `Laporan untuk ${report.room.code} terkirim ke TU/Kepala Lab.`
      )
      onCreated(report)
    },
    onError: (error) => {
      const details = getApiError(error)?.details ?? []
      const rest = applyFieldErrors(error, form.setError, {
        room_id: 'room_id',
        description: 'description',
        photo: 'photo',
      })
      // Field errors show under their fields; anything else goes in the alert.
      if (rest.length > 0 || details.length === 0) {
        setProblem(rest[0]?.message ?? getErrorMessage(error))
      }
    },
  })

  // Closing mid-upload would hide the result; keep the dialog until the request settles.
  const holdOpen = (e: Event) => {
    if (create.isPending) e.preventDefault()
  }

  return (
    <>
      <DialogContent
        className='sm:max-w-lg'
        onEscapeKeyDown={holdOpen}
        onInteractOutside={holdOpen}
      >
        <DialogHeader>
          <DialogTitle>Laporkan kerusakan</DialogTitle>
          <DialogDescription>
            Laporan dikirim ke TU/Kepala Lab. Anda akan diberi tahu setiap kali
            statusnya berubah.
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            id='report-form'
            noValidate
            className='grid gap-4'
            onSubmit={form.handleSubmit((v) => {
              setProblem(null)
              create.mutate({
                room_id: v.room_id,
                description: v.description.trim(),
                photo: v.photo,
              })
            })}
          >
            {problem && (
              <Alert variant='destructive'>
                <AlertCircle />
                <AlertDescription>{problem}</AlertDescription>
              </Alert>
            )}
            <FormField
              control={form.control}
              name='room_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Ruang</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
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
                          {room.code} · {room.name}
                          {!room.is_active && ' (nonaktif)'}
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
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Kerusakan</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={4}
                      maxLength={DESCRIPTION_MAX}
                      placeholder='Contoh: Proyektor tidak menyala, AC bocor di sisi kiri.'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <PhotoField control={form.control} />
          </form>
        </Form>
        <DialogFooter>
          <Button
            variant='outline'
            disabled={create.isPending}
            onClick={onCancel}
          >
            Batal
          </Button>
          <Button type='submit' form='report-form' disabled={create.isPending}>
            {create.isPending && <Loader2 className='animate-spin' />}
            Kirim Laporan
          </Button>
        </DialogFooter>
      </DialogContent>
    </>
  )
}

function PhotoField({ control }: { control: Control<ReportFormValues> }) {
  const photo = useWatch({ control, name: 'photo' })
  const preview = useDataUrl(photo)

  return (
    <FormField
      control={control}
      name='photo'
      render={({ field: { onChange, value, ref, name, onBlur } }) => (
        <FormItem>
          <FormLabel>Foto (opsional)</FormLabel>
          {value && preview ? (
            <div className='relative w-fit'>
              <img
                src={preview}
                alt='Pratinjau foto kerusakan'
                className='max-h-40 rounded-md border object-contain'
              />
              <Button
                type='button'
                variant='secondary'
                size='icon'
                className='absolute end-1 top-1 size-7'
                aria-label='Hapus foto'
                onClick={() => onChange(undefined)}
              >
                <X />
              </Button>
            </div>
          ) : (
            <FormControl>
              <Input
                ref={ref}
                name={name}
                onBlur={onBlur}
                type='file'
                accept={PHOTO_TYPES.join(',')}
                onChange={(e) => onChange(e.target.files?.[0] ?? undefined)}
              />
            </FormControl>
          )}
          <FormDescription className='flex items-center gap-1'>
            <ImagePlus className='size-3.5' aria-hidden />
            JPG, PNG, atau WebP, maksimal 5 MB. Lokasi GPS di foto dihapus
            server.
          </FormDescription>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}
