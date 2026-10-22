import { useEffect, useRef } from 'react'
import { Link } from '@tanstack/react-router'
import {
  CircleCheck,
  Clock,
  Loader2,
  QrCode,
  TriangleAlert,
  UserX,
} from 'lucide-react'
import { type CheckInResult } from '@/lib/api-types'
import {
  formatDateShort,
  formatDateTime,
  formatTimeRange,
} from '@/lib/datetime'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { BentoCard } from '@/components/bento-card'
import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/layout/page'
import { useReportPhoto } from '@/features/reports/api'
import { useCheckIn } from './api'
import { CONDITION_LABELS, classifyCheckInError } from './rules'

type CheckInProps = {
  /** `room` and `t` from the QR link; either may be missing from a hand-typed URL. */
  room?: string
  token?: string
}

/**
 * Landing page of the QR on the door (`/check-in?room=&t=`). Opening it is the intent to check in,
 * so the request goes out once on arrival; the server decides who may and when (BR-11).
 */
export function CheckIn({ room, token }: CheckInProps) {
  const checkIn = useCheckIn()
  const sent = useRef(false)
  const { mutate } = checkIn

  useEffect(() => {
    // StrictMode runs effects twice in dev; a second request would meet an already running booking.
    if (sent.current || !room || !token) return
    sent.current = true
    mutate({ room_code: room, qr_token: token })
  }, [room, token, mutate])

  function retry() {
    if (room && token) mutate({ room_code: room, qr_token: token })
  }

  return (
    <Page title='Check-in'>
      <div className='mx-auto w-full max-w-xl'>
        <Outcome
          missingLink={!room || !token}
          pending={checkIn.isPending || (checkIn.isIdle && !!room && !!token)}
          error={checkIn.isError ? checkIn.error : null}
          result={checkIn.data}
          onRetry={retry}
        />
      </div>
    </Page>
  )
}

function Outcome({
  missingLink,
  pending,
  error,
  result,
  onRetry,
}: {
  missingLink: boolean
  pending: boolean
  error: unknown
  result: CheckInResult | undefined
  onRetry: () => void
}) {
  if (missingLink) return <InvalidQr />
  if (pending) {
    return (
      <div
        role='status'
        className='flex items-center justify-center gap-2 py-16 text-fg-secondary'
      >
        <Loader2 className='animate-spin' aria-hidden />
        Memeriksa booking kamu...
      </div>
    )
  }
  if (result) return <Success result={result} />

  const problem = classifyCheckInError(error)
  switch (problem.kind) {
    case 'qr-invalid':
      return <InvalidQr />
    case 'outside-window':
      return (
        <EmptyState
          icon={Clock}
          title='Belum waktunya check-in'
          description={`Check-in booking berikutnya di ruang ini dibuka ${formatDateTime(problem.opensAt)} dan ditutup ${formatDateTime(problem.closesAt)}. Pindai lagi QR-nya saat itu.`}
          action={<HomeButton />}
        />
      )
    case 'not-owner':
      return (
        <EmptyState
          icon={UserX}
          title='Tidak ada booking untukmu di sini'
          description='Akunmu tidak punya booking disetujui di ruang ini pada jam ini, atau kamu sudah check-in. Check-in hanya untuk pemilik booking.'
          action={<HomeButton />}
        />
      )
    case 'other':
      return (
        <div className='flex flex-col gap-4'>
          <Alert variant='error' role='alert'>
            <TriangleAlert />
            <AlertTitle>Check-in belum berhasil</AlertTitle>
            <AlertDescription>{problem.message}</AlertDescription>
          </Alert>
          <Button className='h-11' onClick={onRetry}>
            Coba lagi
          </Button>
        </div>
      )
  }
}

function HomeButton() {
  return (
    <Button asChild className='h-11'>
      <Link to='/'>Ke Beranda</Link>
    </Button>
  )
}

function InvalidQr() {
  return (
    <EmptyState
      icon={QrCode}
      title='QR tidak valid'
      description='QR ini tidak dikenali atau sudah diganti pengelola. Pindai QR yang terpasang di pintu ruang. Bila tetap gagal, hubungi TU atau Pengurus Lab.'
      action={<HomeButton />}
    />
  )
}

function Success({ result }: { result: CheckInResult }) {
  const { booking, previous_checkout: previous } = result
  return (
    <div className='flex flex-col gap-4'>
      <BentoCard title='Check-in berhasil'>
        <div className='flex items-start gap-3'>
          <CircleCheck aria-hidden className='mt-1 size-5 shrink-0 text-fg' />
          <div className='min-w-0'>
            <p className='type-stat-l text-fg'>{booking.room.code}</p>
            <p className='type-body text-fg-secondary'>
              {formatDateShort(booking.start_at)} ·{' '}
              {formatTimeRange(booking.start_at, booking.end_at)}
            </p>
            <p className='mt-2 type-body-sm text-fg-secondary'>
              Jangan lupa check-out dengan foto sebelum jam selesai.
            </p>
          </div>
        </div>
      </BentoCard>

      <BentoCard title='Kondisi ruang sebelum kamu'>
        {previous ? (
          <PreviousCheckoutView previous={previous} />
        ) : (
          <p className='type-body text-fg-secondary'>
            Belum ada catatan check-out sebelumnya untuk ruang ini.
          </p>
        )}
        <p className='type-body-sm text-fg-secondary'>
          Ruang tidak seperti yang dicatat? Laporkan sekarang supaya tidak
          dianggap kerusakanmu.
        </p>
        <Button variant='secondary' className='h-11' asChild>
          <Link to='/reports'>Laporkan masalah</Link>
        </Button>
      </BentoCard>

      <div className='flex flex-col gap-2 sm:flex-row'>
        <Button asChild className='h-11 flex-1'>
          <Link to='/check-out/$id' params={{ id: booking.id }}>
            Check-out
          </Link>
        </Button>
        <Button asChild variant='secondary' className='h-11 flex-1'>
          <Link to='/'>Ke Beranda</Link>
        </Button>
      </div>
    </div>
  )
}

function PreviousCheckoutView({
  previous,
}: {
  previous: NonNullable<CheckInResult['previous_checkout']>
}) {
  const photo = useReportPhoto({ attachment_id: previous.attachment_id })
  return (
    <div className='flex flex-col gap-3'>
      <p className='type-body text-fg'>
        {CONDITION_LABELS[previous.condition]}
        <span className='text-fg-secondary'>
          {' '}
          · ditinggalkan {formatDateTime(previous.checked_out_at)}
        </span>
      </p>
      {photo.url ? (
        <img
          src={photo.url}
          alt='Foto kondisi ruang saat ditinggalkan pengguna sebelumnya'
          className='max-h-80 w-full rounded-xl border object-cover'
        />
      ) : (
        !photo.isLoading && (
          <p className='type-body-sm text-fg-secondary'>
            Foto tidak tersedia (sudah dihapus sesuai masa simpan).
          </p>
        )
      )}
    </div>
  )
}
