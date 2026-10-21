import { useState } from 'react'
import { attendanceRules } from '@/config/attendance-rules'
import { AlertCircle, Download, Loader2, TriangleAlert } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type Booking } from '@/lib/api-types'
import { formatDateTime, formatTimeRange } from '@/lib/datetime'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { BookingStatusBadge } from '@/components/status-badge'
import { StatusPill } from '@/components/status-pill'
import {
  useApproveBooking,
  useBooking,
  useBookings,
  useRejectBooking,
} from '@/features/bookings/api'
import { ReasonDialog } from '@/features/bookings/components/reason-dialog'
import { hasEnded } from '@/features/bookings/transitions'
import { dayShort } from '@/features/my-bookings/format'
import { ROOM_TYPE_LABELS } from '@/features/rooms/labels'
import { useDownloadLetter } from './api'
import { MANUAL_REASON_LABELS, expiryLabel } from './rules'

/** The "Alasan Manual" pill: why the system did not approve this request by itself. */
export function ReasonPill({ reason }: { reason: Booking['manual_reason'] }) {
  if (!reason) return null
  return (
    <StatusPill
      status={reason === 'ROOM_POLICY' ? 'Terbatas' : 'Diajukan'}
      label={MANUAL_REASON_LABELS[reason]}
    />
  )
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className='flex items-baseline justify-between gap-4'>
      <dt className='type-body text-fg-secondary'>{label}</dt>
      <dd className='min-w-0 text-end type-card-title text-fg'>{value}</dd>
    </div>
  )
}

type ApprovalDetailProps = {
  bookingId: string
  now: Date
  /** Called after a decision went through, so a popup can close. */
  onDecided?: () => void
}

/** "Detail Pengajuan" (Figma): what the approver needs to decide, and the two buttons that decide. */
export function ApprovalDetail({
  bookingId,
  now,
  onDecided,
}: ApprovalDetailProps) {
  const booking = useBooking(bookingId)
  const b = booking.data
  const groupId = b && b.occurrence_count > 1 ? b.group_id : null
  const group = useBookings(
    { group_id: groupId ?? undefined, per_page: 50 },
    !!groupId
  )

  if (booking.isError) {
    return (
      <Alert variant='error'>
        <AlertCircle />
        <AlertDescription>{getErrorMessage(booking.error)}</AlertDescription>
      </Alert>
    )
  }
  if (!b) return <Skeleton className='h-80' />

  const dates = groupId ? (group.data?.data ?? []) : [b]
  return <Body booking={b} dates={dates} now={now} onDecided={onDecided} />
}

function Body({
  booking: b,
  dates,
  now,
  onDecided,
}: {
  booking: Booking
  dates: Booking[]
  now: Date
  onDecided?: () => void
}) {
  const [dialog, setDialog] = useState<'approve' | 'reject' | null>(null)
  const approve = useApproveBooking()
  const reject = useRejectBooking()
  const download = useDownloadLetter()
  const pending = b.status === 'MENUNGGU'
  const ended = hasEnded(b, now.getTime())
  const left = b.expires_at ? expiryLabel(b.expires_at, now) : null
  const summary = b.requester_summary
  const slot = `${b.room.code}, ${dayShort(b.start_at)} ${formatTimeRange(b.start_at, b.end_at)}`
  const done = () => {
    setDialog(null)
    onDecided?.()
  }

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <p className='truncate type-section text-fg'>{b.room.code}</p>
          <p className='type-body text-fg-secondary'>
            {ROOM_TYPE_LABELS[b.room.type]} · {b.purpose}
          </p>
        </div>
        {pending ? (
          <ReasonPill reason={b.manual_reason} />
        ) : (
          <BookingStatusBadge status={b.status} />
        )}
      </div>

      {pending && ended && (
        <Alert variant='error'>
          <TriangleAlert />
          <AlertTitle>Waktu sudah lewat</AlertTitle>
          <AlertDescription>
            Tidak bisa disetujui lagi; tolak untuk membersihkan antrean.
          </AlertDescription>
        </Alert>
      )}
      {pending && !ended && b.expires_at && (
        <Alert variant='warning'>
          <TriangleAlert />
          <AlertTitle>
            {left ? `Kedaluwarsa dalam ${left}` : 'Pengajuan kedaluwarsa'}
          </AlertTitle>
          <AlertDescription>
            Diputuskan sebelum {formatDateTime(b.expires_at)}.
          </AlertDescription>
        </Alert>
      )}
      {b.decision_reason && (
        <p className='type-body text-fg-secondary'>
          Alasan: {b.decision_reason}
        </p>
      )}

      <dl className='flex flex-col gap-3'>
        <Row
          label='Peminjam'
          value={`${b.requester.name} · ${b.requester.identifier}`}
        />
        <Row
          label='Tanggal'
          value={
            dates.length > 1
              ? `${dates.length} tanggal (grup)`
              : dayShort(b.start_at)
          }
        />
        {dates.length > 1 &&
          dates.map((d) => (
            <Row
              key={d.id}
              label={dayShort(d.start_at)}
              value={formatTimeRange(d.start_at, d.end_at)}
            />
          ))}
        {dates.length <= 1 && (
          <Row label='Waktu' value={formatTimeRange(b.start_at, b.end_at)} />
        )}
        {b.participants ? (
          <Row label='Peserta' value={`${b.participants} orang`} />
        ) : null}
        {summary && (
          <Row
            label='Poin pelanggaran'
            value={`${summary.active_points} / ${attendanceRules.blockThresholdPoints}`}
          />
        )}
      </dl>

      {summary && summary.courses.length > 0 && (
        <div className='flex flex-col gap-1'>
          <p className='type-overline text-fg-secondary'>
            Mata kuliah semester ini
          </p>
          <ul className='type-body-sm text-fg-secondary'>
            {summary.courses.map((c, i) => (
              <li key={i}>
                {c.course_code ? `${c.course_code} · ` : ''}
                {c.course_name ?? '-'}
                {c.class_name ? ` (${c.class_name})` : ''}
              </li>
            ))}
          </ul>
        </div>
      )}

      {b.notes && (
        <p className='type-body-sm break-words text-fg-secondary'>
          Catatan: {b.notes}
        </p>
      )}

      {b.letter && (
        <div className='flex items-center gap-3 rounded-2xl border py-3 ps-4 pe-3'>
          <div className='min-w-0 flex-1'>
            <p className='truncate type-card-title text-fg'>
              {b.letter.letter_number
                ? `Surat ${b.letter.letter_number}`
                : 'Surat tugas / kegiatan'}
            </p>
            <p className='type-body-sm text-fg-secondary'>Lampiran pemohon</p>
          </div>
          <Button
            variant='secondary'
            size='sm'
            disabled={download.isPending}
            onClick={() =>
              download.mutate({
                id: b.letter!.attachment_id,
                name: `surat-${b.letter!.letter_number ?? b.id}.pdf`,
              })
            }
          >
            {download.isPending ? (
              <Loader2 className='animate-spin' />
            ) : (
              <Download />
            )}
            Unduh
          </Button>
        </div>
      )}

      {pending && (
        <div className='flex gap-2'>
          <Button
            variant='secondary'
            className='h-10 flex-1'
            onClick={() => setDialog('reject')}
          >
            Tolak
          </Button>
          <Button
            className='h-10 flex-1'
            disabled={ended}
            onClick={() => setDialog('approve')}
          >
            Setujui
          </Button>
        </div>
      )}

      <ConfirmDialog
        open={dialog === 'approve'}
        onOpenChange={(o) => !o && setDialog(null)}
        title='Setujui pengajuan?'
        desc={
          <p>
            {b.requester.name} meminjam <strong>{slot}</strong>
            {dates.length > 1
              ? ` dan ${dates.length - 1} tanggal lainnya`
              : ''}{' '}
            untuk “{b.purpose}”. Peminjam diberi notifikasi.
          </p>
        }
        confirmText={
          approve.isPending ? (
            <>
              <Loader2 className='animate-spin' />
              Menyetujui…
            </>
          ) : (
            'Setujui'
          )
        }
        isLoading={approve.isPending}
        handleConfirm={() =>
          approve.mutate(b.id, {
            onSuccess: done,
            onError: () => setDialog(null),
          })
        }
      />
      <ReasonDialog
        open={dialog === 'reject'}
        onOpenChange={(o) => !o && setDialog(null)}
        title='Tolak pengajuan?'
        description={
          <p>Pemohon menerima notifikasi beserta alasan penolakan.</p>
        }
        label='Alasan penolakan'
        placeholder='Minimal 5 karakter. Contoh: Bentrok dengan kegiatan jurusan.'
        confirmText='Tolak'
        required
        isPending={reject.isPending}
        onSubmit={(reason) =>
          reject.mutate({ id: b.id, reason }, { onSuccess: done })
        }
      />
    </div>
  )
}
