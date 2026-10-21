import { useState } from 'react'
import { useFormContext } from 'react-hook-form'
import { FileText, Loader2 } from 'lucide-react'
import { applyFieldErrors, getErrorMessage } from '@/lib/api-error'
import { type Attachment } from '@/lib/api-types'
import { formatDateShort } from '@/lib/datetime'
import { formatFileSize } from '@/lib/file-size'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Dropzone } from '@/components/dropzone'
import { useLetters, useUploadLetter } from './letters'
import { LETTER_NUMBER_MAX, type RequestFormValues } from './request-schema'

const LETTER_ACCEPT = 'application/pdf,image/jpeg,image/png,image/webp'
/** MAX_LETTER_MB of the API; the server stays the authority. */
const LETTER_MAX_BYTES = 5 * 1024 * 1024

type LetterFieldProps = {
  required: boolean
}

/** The letter of an organisation or auditorium request: upload one, or reuse an earlier letter (BR-19). */
export function LetterField({ required }: LetterFieldProps) {
  const form = useFormContext<RequestFormValues>()
  const [picked, setPicked] = useState<Attachment | null>(null)
  const [oldOpen, setOldOpen] = useState(false)
  const upload = useUploadLetter()

  const attach = (letter: Attachment | null) => {
    setPicked(letter)
    form.setValue('letter_attachment_id', letter?.id ?? '', {
      shouldValidate: form.formState.isSubmitted,
    })
  }

  const onFiles = ([file]: File[]) => {
    if (file.size > LETTER_MAX_BYTES) {
      form.setError('letter_attachment_id', {
        message: 'Berkas terlalu besar. Maksimal 5 MB.',
      })
      return
    }
    form.clearErrors('letter_attachment_id')
    upload.mutate(file, {
      onSuccess: attach,
      onError: (error) => {
        const rest = applyFieldErrors(error, form.setError, {
          file: 'letter_attachment_id',
        })
        if (
          rest.length > 0 ||
          !form.getFieldState('letter_attachment_id').error
        ) {
          form.setError('letter_attachment_id', {
            message: getErrorMessage(error),
          })
        }
      },
    })
  }

  const title = required
    ? 'Surat tugas / kegiatan (wajib)'
    : 'Lampiran surat izin (opsional)'

  return (
    <div className='grid gap-3'>
      <p className='type-overline text-fg-secondary'>{title}</p>

      <FormField
        control={form.control}
        name='letter_attachment_id'
        render={() => (
          <FormItem>
            {picked ? (
              <div className='grid items-start gap-3 sm:grid-cols-[1fr_minmax(0,14rem)]'>
                <div className='flex flex-wrap items-center gap-3 rounded-xl border bg-card p-4'>
                  <FileText
                    aria-hidden
                    className='size-5 shrink-0 text-fg-secondary'
                  />
                  <div className='min-w-0 grow'>
                    <p className='truncate type-card-title text-fg'>
                      {picked.original_name ?? 'Surat'}
                    </p>
                    <p className='type-body-sm text-fg-secondary'>
                      {picked.content_type === 'application/pdf'
                        ? 'PDF'
                        : 'Foto'}
                      {' · '}
                      {formatFileSize(picked.size_bytes)}
                    </p>
                  </div>
                  <div className='flex gap-1'>
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      onClick={() => attach(null)}
                    >
                      Ganti
                    </Button>
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      onClick={() => setOldOpen(true)}
                    >
                      Pilih surat lama
                    </Button>
                  </div>
                </div>
                <FormField
                  control={form.control}
                  name='letter_number'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Nomor surat (opsional)</FormLabel>
                      <FormControl>
                        <Input
                          maxLength={LETTER_NUMBER_MAX}
                          placeholder='012/HMIF/X/2026'
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            ) : (
              <div className='grid gap-2'>
                <Dropzone
                  accept={LETTER_ACCEPT}
                  disabled={upload.isPending}
                  title={
                    upload.isPending
                      ? 'Mengunggah…'
                      : 'Tarik surat ke sini atau pilih berkas'
                  }
                  hint='PDF atau foto, maks 5 MB'
                  onFiles={onFiles}
                />
                {upload.isPending && (
                  <p className='flex items-center gap-2 type-body-sm text-fg-secondary'>
                    <Loader2 className='size-4 animate-spin' aria-hidden />
                    Mengunggah surat…
                  </p>
                )}
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  className='justify-self-start'
                  onClick={() => setOldOpen(true)}
                >
                  Pilih surat lama
                </Button>
              </div>
            )}
            <FormMessage />
          </FormItem>
        )}
      />

      <OldLettersDialog
        open={oldOpen}
        onOpenChange={setOldOpen}
        onPick={(letter) => {
          attach(letter)
          setOldOpen(false)
        }}
      />
    </div>
  )
}

function OldLettersDialog({
  open,
  onOpenChange,
  onPick,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onPick: (letter: Attachment) => void
}) {
  const letters = useLetters(open)
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Pilih surat lama</DialogTitle>
          <DialogDescription>
            Surat yang pernah kamu unggah bisa dipakai lagi.
          </DialogDescription>
        </DialogHeader>
        {letters.isPending && <Skeleton className='h-24' />}
        {letters.isError && (
          <p className='type-body text-fg-error'>
            {getErrorMessage(letters.error)}
          </p>
        )}
        {letters.data?.length === 0 && (
          <p className='type-body text-fg-secondary'>
            Belum ada surat yang diunggah.
          </p>
        )}
        <ul className='grid max-h-72 gap-2 overflow-y-auto'>
          {letters.data?.map((letter) => (
            <li key={letter.id}>
              <button
                type='button'
                onClick={() => onPick(letter)}
                className='flex w-full items-center gap-3 rounded-xl border p-3 text-start hover:border-line-hover'
              >
                <FileText
                  aria-hidden
                  className='size-5 shrink-0 text-fg-secondary'
                />
                <span className='min-w-0 grow'>
                  <span className='block truncate type-card-title text-fg'>
                    {letter.original_name ?? 'Surat'}
                  </span>
                  <span className='type-body-sm text-fg-secondary'>
                    {formatDateShort(letter.created_at)} ·{' '}
                    {formatFileSize(letter.size_bytes)}
                  </span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      </DialogContent>
    </Dialog>
  )
}
