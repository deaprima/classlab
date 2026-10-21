import { useState } from 'react'
import { AlertCircle, ArrowRight, ImageOff, Loader2 } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage, isApiErrorCode } from '@/lib/api-error'
import { type ReportDetail, type ReportEvent } from '@/lib/api-types'
import { formatDateTime } from '@/lib/datetime'
import { ACCESS, useHasRole } from '@/lib/rbac'
import { REPORT_STATUS_LABELS } from '@/lib/status-labels'
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
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { ReportStatusBadge } from '@/components/status-badge'
import { useReport, useReportPhoto, useUpdateReportStatus } from '../api'
import { NOTE_MAX, nextReportStatus } from '../report-rules'

/** Report detail with photo and append-only status trail. Opened from the list by `?report=<id>`. */
export function ReportDetailSheet({
  reportId,
  onClose,
}: {
  reportId: string | undefined
  onClose: () => void
}) {
  const report = useReport(reportId)

  return (
    <Sheet open={!!reportId} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className='w-full gap-0 overflow-y-auto sm:max-w-lg'>
        <SheetHeader>
          <SheetTitle>Detail laporan</SheetTitle>
          <SheetDescription>
            {report.data
              ? `${report.data.room.code} · ${report.data.room.name}`
              : 'Memuat…'}
          </SheetDescription>
        </SheetHeader>
        <div className='px-4 pb-4'>
          {report.isError && (
            <Alert variant='destructive'>
              <AlertCircle />
              <AlertDescription>
                {isApiErrorCode(report.error, 'NOT_FOUND')
                  ? 'Laporan tidak ditemukan.'
                  : getErrorMessage(report.error)}
              </AlertDescription>
            </Alert>
          )}
          {report.isPending && reportId && <Skeleton className='h-64' />}
          {report.data && <ReportInfo report={report.data} />}
        </div>
        {report.data && (
          <SheetFooter className='border-t'>
            <StatusAction report={report.data} />
          </SheetFooter>
        )}
      </SheetContent>
    </Sheet>
  )
}

function ReportInfo({ report }: { report: ReportDetail }) {
  return (
    <div className='space-y-6'>
      <dl className='grid grid-cols-[7rem_1fr] gap-x-4 gap-y-2 text-sm'>
        <dt className='text-muted-foreground'>Status</dt>
        <dd>
          <ReportStatusBadge status={report.status} />
        </dd>
        <dt className='text-muted-foreground'>Ruang</dt>
        <dd>
          {report.room.code} · {report.room.name}
        </dd>
        <dt className='text-muted-foreground'>Pelapor</dt>
        <dd>
          {report.reporter.name} ({report.reporter.identifier})
        </dd>
        <dt className='text-muted-foreground'>Dilaporkan</dt>
        <dd>{formatDateTime(report.created_at)}</dd>
        <dt className='text-muted-foreground'>Kerusakan</dt>
        <dd className='break-words whitespace-pre-line'>
          {report.description}
        </dd>
      </dl>
      {report.attachment_id && <ReportPhoto report={report} />}
      <EventTrail report={report} />
    </div>
  )
}

function ReportPhoto({ report }: { report: ReportDetail }) {
  const photo = useReportPhoto(report)
  if (photo.error) {
    return (
      <p className='flex items-center gap-2 text-sm text-destructive'>
        <ImageOff className='size-4' />
        Foto gagal dimuat. {getErrorMessage(photo.error)}
      </p>
    )
  }
  if (!photo.url) return <Skeleton className='h-48' />
  return (
    <img
      src={photo.url}
      alt={`Foto kerusakan di ${report.room.code}`}
      className='max-h-96 w-full rounded-md border object-contain'
    />
  )
}

function EventTrail({ report }: { report: ReportDetail }) {
  const me = useAuthStore((s) => s.auth.user?.id)
  const actor = (e: ReportEvent) => {
    if (e.actor_id === me) return 'Anda'
    if (e.actor_id === report.reporter_id) return report.reporter.name
    return 'TU/Kepala Lab'
  }
  return (
    <section>
      <h3 className='mb-3 text-sm font-semibold'>Riwayat status</h3>
      <ol className='space-y-4 border-s ps-4'>
        {report.events.map((e) => (
          <li key={e.id} className='text-sm'>
            <p className='font-medium'>
              {e.from_status
                ? `${REPORT_STATUS_LABELS[e.from_status]} → ${REPORT_STATUS_LABELS[e.to_status]}`
                : `Dilaporkan (${REPORT_STATUS_LABELS[e.to_status]})`}
            </p>
            <p className='text-muted-foreground'>
              {formatDateTime(e.created_at)} · {actor(e)}
            </p>
            {e.note && <p className='mt-1 break-words'>“{e.note}”</p>}
          </li>
        ))}
      </ol>
    </section>
  )
}

/** BR-08: TU/Kalab moves the report one step forward, with an optional note for the reporter. */
function StatusAction({ report }: { report: ReportDetail }) {
  const canUpdate = useHasRole(ACCESS.updateReportStatus)
  const update = useUpdateReportStatus()
  const [open, setOpen] = useState(false)
  const [note, setNote] = useState('')
  const next = nextReportStatus(report.status)
  if (!canUpdate || !next) return null

  const close = () => {
    setOpen(false)
    setNote('')
  }

  return (
    <>
      <Button onClick={() => setOpen(true)}>
        Tandai {REPORT_STATUS_LABELS[next]}
        <ArrowRight />
      </Button>
      <Dialog
        open={open}
        onOpenChange={(o) => !update.isPending && !o && close()}
      >
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>
              Ubah status ke {REPORT_STATUS_LABELS[next]}?
            </DialogTitle>
            <DialogDescription>
              {report.room.code}: pelapor akan diberi tahu. Status tidak bisa
              dikembalikan.
            </DialogDescription>
          </DialogHeader>
          <div className='grid gap-2'>
            <Label htmlFor='report-note'>
              Catatan untuk pelapor (opsional)
            </Label>
            <Textarea
              id='report-note'
              rows={3}
              maxLength={NOTE_MAX}
              value={note}
              onChange={(e) => setNote(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              disabled={update.isPending}
              onClick={close}
            >
              Batal
            </Button>
            <Button
              disabled={update.isPending}
              onClick={() =>
                update.mutate(
                  {
                    id: report.id,
                    status: next,
                    note: note.trim() || undefined,
                  },
                  { onSuccess: close }
                )
              }
            >
              {update.isPending && <Loader2 className='animate-spin' />}
              Tandai {REPORT_STATUS_LABELS[next]}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
