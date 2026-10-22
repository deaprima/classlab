import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { Camera, CircleCheck, Loader2, TriangleAlert } from 'lucide-react'
import { getApiError, getErrorMessage } from '@/lib/api-error'
import { type CheckOutResult, type CheckoutCondition } from '@/lib/api-types'
import { formatDateShort, formatTimeRange } from '@/lib/datetime'
import { useDataUrl } from '@/hooks/use-data-url'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { BentoCard } from '@/components/bento-card'
import { CameraCapture } from '@/components/camera-capture'
import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/layout/page'
import { useBooking } from '@/features/bookings/api'
import { useCheckOut } from './api'
import {
  CHECKOUT_NOTE_MAX,
  CONDITION_LABELS,
  type CheckoutErrors,
  validateCheckout,
} from './rules'

/** "Check-out – Kamera" (DESIGN.md): the condition of the room plus a photo taken on the spot (BR-12). */
export function CheckOut({ bookingId }: { bookingId: string }) {
  const booking = useBooking(bookingId)
  const [done, setDone] = useState<CheckOutResult | null>(null)

  return (
    <Page title='Check-out'>
      <div className='mx-auto w-full max-w-xl'>
        {done ? (
          <Done result={done} />
        ) : booking.isLoading ? (
          <Skeleton className='h-64 w-full rounded-2xl' />
        ) : !booking.data ? (
          <EmptyState
            icon={TriangleAlert}
            title='Booking tidak ditemukan'
            description='Booking ini tidak ada atau bukan milikmu.'
            action={<BackHome />}
          />
        ) : booking.data.status !== 'BERLANGSUNG' ? (
          <EmptyState
            icon={TriangleAlert}
            title='Belum bisa check-out'
            description='Check-out hanya untuk booking yang sedang berlangsung (sudah check-in).'
            action={<BackHome />}
          />
        ) : (
          <CheckOutForm
            bookingId={bookingId}
            roomCode={booking.data.room.code}
            slot={`${formatDateShort(booking.data.start_at)} · ${formatTimeRange(booking.data.start_at, booking.data.end_at)}`}
            onDone={setDone}
          />
        )}
      </div>
    </Page>
  )
}

function BackHome() {
  return (
    <Button asChild className='h-11'>
      <Link to='/'>Ke Beranda</Link>
    </Button>
  )
}

function CheckOutForm({
  bookingId,
  roomCode,
  slot,
  onDone,
}: {
  bookingId: string
  roomCode: string
  slot: string
  onDone: (result: CheckOutResult) => void
}) {
  const [condition, setCondition] = useState<CheckoutCondition | null>(null)
  const [note, setNote] = useState('')
  const [photo, setPhoto] = useState<Blob | null>(null)
  const [errors, setErrors] = useState<CheckoutErrors>({})
  const [retaking, setRetaking] = useState(false)
  const [problem, setProblem] = useState<string | null>(null)
  const photoUrl = useDataUrl(photo ?? undefined)
  const checkOut = useCheckOut()

  function submit() {
    const found = validateCheckout({ condition, note, photo })
    setErrors(found)
    setProblem(null)
    if (!condition || !photo || Object.keys(found).length > 0) return
    checkOut.mutate(
      { bookingId, condition, note: note.trim() || undefined, photo },
      {
        onSuccess: onDone,
        onError: (error) => {
          const detail = getApiError(error)?.details.find((d) => d.message)
          setProblem(detail?.message ?? getErrorMessage(error))
        },
      }
    )
  }

  return (
    <form
      className='flex flex-col gap-4'
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        submit()
      }}
    >
      <BentoCard title={roomCode}>
        <p className='type-body text-fg-secondary'>{slot}</p>
      </BentoCard>

      <BentoCard title='Kondisi ruang'>
        <RadioGroup
          value={condition ?? ''}
          onValueChange={(v) => setCondition(v as CheckoutCondition)}
          aria-label='Kondisi ruang'
          className='gap-1'
        >
          {(Object.keys(CONDITION_LABELS) as CheckoutCondition[]).map((c) => (
            <div key={c} className='flex min-h-11 items-center gap-3'>
              <RadioGroupItem value={c} id={`condition-${c}`} />
              <Label htmlFor={`condition-${c}`} className='flex-1 font-normal'>
                {CONDITION_LABELS[c]}
              </Label>
            </div>
          ))}
        </RadioGroup>
        <FieldError message={errors.condition} />

        {condition === 'ADA_MASALAH' && (
          <div className='flex flex-col gap-2'>
            <Label htmlFor='checkout-note'>Apa yang rusak?</Label>
            <Textarea
              id='checkout-note'
              value={note}
              maxLength={CHECKOUT_NOTE_MAX}
              rows={4}
              placeholder='Mis. kursi patah di baris kedua, proyektor tidak menyala'
              aria-invalid={!!errors.note}
              onChange={(event) => setNote(event.target.value)}
              className='text-base'
            />
            <FieldError message={errors.note} />
            <p className='type-body-sm text-fg-secondary'>
              Laporan kerusakan dibuat otomatis untuk TU atau Pengurus Lab.
            </p>
          </div>
        )}
      </BentoCard>

      <BentoCard title='Foto kondisi ruang'>
        {photo && photoUrl && (
          <img
            src={photoUrl}
            alt='Foto kondisi ruang yang akan dikirim'
            className='max-h-80 w-full rounded-xl border object-cover'
          />
        )}
        {(retaking || !photo) && (
          <CameraCapture
            onCapture={(blob) => {
              setPhoto(blob)
              setRetaking(false)
              setErrors((e) => ({ ...e, photo: undefined }))
            }}
            onCancel={() => setRetaking(false)}
          />
        )}
        {photo && !retaking && (
          <Button
            type='button'
            variant='secondary'
            className='h-11'
            onClick={() => setRetaking(true)}
          >
            <Camera />
            Ganti foto
          </Button>
        )}
        <FieldError message={errors.photo} />
      </BentoCard>

      {problem && (
        <Alert variant='error' role='alert'>
          <TriangleAlert />
          <AlertTitle>Check-out belum berhasil</AlertTitle>
          <AlertDescription>{problem}</AlertDescription>
        </Alert>
      )}

      <Button type='submit' className='h-11' disabled={checkOut.isPending}>
        {checkOut.isPending && <Loader2 className='animate-spin' />}
        Check-out
      </Button>
    </form>
  )
}

function FieldError({ message }: { message?: string }) {
  if (!message) return null
  return (
    <p role='alert' className='type-body-sm text-fg-error'>
      {message}
    </p>
  )
}

function Done({ result }: { result: CheckOutResult }) {
  return (
    <div className='flex flex-col gap-4'>
      <BentoCard title='Check-out berhasil'>
        <div className='flex items-start gap-3'>
          <CircleCheck aria-hidden className='mt-1 size-5 shrink-0 text-fg' />
          <div>
            <p className='type-body text-fg'>
              {result.booking.room.code} sudah kamu tinggalkan. Terima kasih!
            </p>
            {result.report_id && (
              <p className='mt-2 type-body-sm text-fg-secondary'>
                Laporan kerusakan sudah dikirim ke pengelola.
              </p>
            )}
          </div>
        </div>
      </BentoCard>
      <div className='flex flex-col gap-2 sm:flex-row'>
        {result.report_id && (
          <Button asChild variant='secondary' className='h-11 flex-1'>
            <Link to='/reports' search={{ report: result.report_id }}>
              Lihat laporan
            </Link>
          </Button>
        )}
        <Button asChild className='h-11 flex-1'>
          <Link to='/'>Ke Beranda</Link>
        </Button>
      </div>
    </div>
  )
}
