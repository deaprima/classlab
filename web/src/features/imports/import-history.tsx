import { useState } from 'react'
import { AlertCircle, History, Undo2 } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type ImportRecord } from '@/lib/api-types'
import { formatDateTime } from '@/lib/datetime'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { ListFooter } from '@/components/list-footer'
import {
  type DataColumn,
  ResponsiveDataList,
} from '@/components/responsive-data-list'
import { StatusPill } from '@/components/status-pill'
import { useImports, useRollbackImport } from './api'
import { summaryNumber } from './rules'

const PER_PAGE = 10

/** What an import did, from its summary (the keys depend on the kind). */
function resultText(record: ImportRecord): string {
  const s = record.summary
  if (record.kind === 'USERS') {
    const created = summaryNumber(s, 'created')
    const updated = summaryNumber(s, 'updated')
    return created === undefined && updated === undefined
      ? '–'
      : `${created ?? 0} baru · ${updated ?? 0} diperbarui`
  }
  const occurrences = summaryNumber(s, 'occurrences')
  const cancelled = summaryNumber(s, 'bookings_cancelled')
  return occurrences === undefined
    ? '–'
    : `${occurrences.toLocaleString('id-ID')} pertemuan${cancelled ? ` · ${cancelled} booking dibatalkan` : ''}`
}

/** "Riwayat Impor": every import, newest first; a schedule import can be rolled back once. */
export function ImportHistory({
  kind,
  title,
}: {
  kind: ImportRecord['kind']
  title: string
}) {
  const [page, setPage] = useState(1)
  const [rolling, setRolling] = useState<ImportRecord | null>(null)
  const imports = useImports({ kind, page, per_page: PER_PAGE })
  // A failed rollback (e.g. already rolled back) is shown by the default toast handler.
  const rollback = useRollbackImport({ onSuccess: () => setRolling(null) })
  const rows = imports.data?.data ?? []

  const columns: DataColumn<ImportRecord>[] = [
    {
      key: 'file',
      header: 'Berkas',
      primary: true,
      cell: (r) => (
        <span className='type-card-title text-fg'>{r.file_name ?? '–'}</span>
      ),
    },
    {
      key: 'semester',
      header: 'Semester',
      cell: (r) => (
        <span className='text-fg-secondary'>{r.semester ?? '–'}</span>
      ),
    },
    {
      key: 'result',
      header: 'Hasil',
      cell: (r) => <span className='text-fg-secondary'>{resultText(r)}</span>,
    },
    {
      key: 'when',
      header: 'Waktu',
      cell: (r) => (
        <span className='text-fg-secondary'>
          {formatDateTime(r.created_at)} · {r.uploaded_by.name}
        </span>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      cell: (r) =>
        r.status === 'COMMITTED' ? (
          <StatusPill status='Disetujui' label='Diterapkan' />
        ) : (
          <StatusPill status='Terbatas' label='Di-rollback' />
        ),
    },
    {
      key: 'actions',
      header: 'Aksi',
      actions: true,
      cell: (r) =>
        r.kind === 'SCHEDULES' && r.status === 'COMMITTED' ? (
          <Button variant='secondary' size='sm' onClick={() => setRolling(r)}>
            <Undo2 />
            Rollback
            <span className='sr-only'> {r.file_name}</span>
          </Button>
        ) : null,
    },
  ]

  return (
    <section
      aria-label={title}
      className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
    >
      <h2 className='type-card-title text-fg'>{title}</h2>
      {imports.isError && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(imports.error)}</AlertDescription>
        </Alert>
      )}
      {imports.isPending ? (
        <Skeleton className='h-40' />
      ) : rows.length === 0 ? (
        <EmptyState
          icon={History}
          title='Belum ada impor'
          description='Impor yang sudah diterapkan tampil di sini.'
          className='py-4'
        />
      ) : (
        <>
          <ResponsiveDataList
            caption='Riwayat impor'
            rows={rows}
            columns={columns}
            rowKey={(r) => r.id}
          />
          <ListFooter
            page={page}
            perPage={PER_PAGE}
            total={imports.data?.meta.total ?? 0}
            onPageChange={setPage}
          />
        </>
      )}

      <ConfirmDialog
        open={rolling !== null}
        onOpenChange={(o) => !o && !rollback.isPending && setRolling(null)}
        title='Rollback impor jadwal?'
        desc={
          <p>
            Pertemuan dari “{rolling?.file_name}” yang belum dimulai dihapus;
            yang sudah berjalan tetap menjadi riwayat. Booking yang dibatalkan
            oleh impor ini <strong>tidak</strong> dikembalikan.
          </p>
        }
        confirmText='Rollback'
        destructive
        isLoading={rollback.isPending}
        handleConfirm={() => rolling && rollback.mutate(rolling.id)}
      />
    </section>
  )
}
