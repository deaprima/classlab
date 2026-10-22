import { useMemo } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { AlertCircle, Clock } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type DamageReport, type ReportStatus } from '@/lib/api-types'
import { formatRelative } from '@/lib/datetime'
import { REPORT_STATUS_LABELS } from '@/lib/status-labels'
import { useNow } from '@/hooks/use-now'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { FilterChip } from '@/components/filter-chip'
import { ReportStatusBadge } from '@/components/status-badge'
import { useManagedTypes } from '@/features/room-types/api'
import { ROOM_TYPE_LABELS } from '@/features/rooms/labels'
import { useReports } from '../api'
import { REPORT_FLOW } from '../report-rules'

const route = getRouteApi('/_authenticated/reports/')
// Each column loads up to this many; the heading shows the real total of the column.
const COLUMN_LIMIT = 50

/** Laporan Kerusakan for managers (Figma "TU – Laporan"): one column per step, a card per report. */
export function ReportBoard() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const types = useManagedTypes()
  const now = useNow(60_000)
  const type = search.type

  const columns = [
    useReports({ status: 'BARU', per_page: COLUMN_LIMIT }),
    useReports({ status: 'DITERIMA', per_page: COLUMN_LIMIT }),
    useReports({ status: 'DIPROSES', per_page: COLUMN_LIMIT }),
    useReports({ status: 'SELESAI', per_page: COLUMN_LIMIT }),
  ]
  const failed = columns.find((c) => c.isError)

  const filter = (next: typeof type) =>
    navigate({ search: (prev) => ({ ...prev, type: next }) })

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-wrap items-center gap-2'>
        <FilterChip active={!type} onClick={() => filter(undefined)}>
          Semua tipe
        </FilterChip>
        {types.map((t) => (
          <FilterChip key={t} active={type === t} onClick={() => filter(t)}>
            {ROOM_TYPE_LABELS[t]}
          </FilterChip>
        ))}
      </div>

      {failed && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(failed.error)}</AlertDescription>
        </Alert>
      )}

      <div className='grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4'>
        {REPORT_FLOW.map((status, i) => (
          <Column
            key={status}
            status={status}
            reports={columns[i].data?.data}
            total={columns[i].data?.meta.total}
            isPending={columns[i].isPending}
            type={type}
            now={now}
            onOpen={(id) =>
              navigate({ search: (prev) => ({ ...prev, report: id }) })
            }
          />
        ))}
      </div>
    </div>
  )
}

function Column({
  status,
  reports,
  total,
  isPending,
  type,
  now,
  onOpen,
}: {
  status: ReportStatus
  reports: DamageReport[] | undefined
  total: number | undefined
  isPending: boolean
  type: DamageReport['room']['type'] | undefined
  now: Date
  onOpen: (id: string) => void
}) {
  const shown = useMemo(
    () => (reports ?? []).filter((r) => !type || r.room.type === type),
    [reports, type]
  )
  // With a type chosen only the loaded cards can be counted; otherwise the API's total is exact.
  const count = type ? shown.length : (total ?? shown.length)

  return (
    <section
      aria-label={`Kolom ${REPORT_STATUS_LABELS[status]}`}
      className='flex min-h-48 min-w-0 flex-col gap-3 rounded-2xl bg-subtle p-3'
    >
      <h2 className='px-2 pt-1 type-card-title text-fg'>
        {REPORT_STATUS_LABELS[status]} · {isPending ? '–' : count}
      </h2>
      {isPending ? (
        <Skeleton className='h-24 rounded-2xl' />
      ) : shown.length === 0 ? (
        <p className='px-2 type-body-sm text-fg-tertiary'>Tidak ada laporan.</p>
      ) : (
        <ul className='flex flex-col gap-3'>
          {shown.map((r) => (
            <li key={r.id}>
              <button
                type='button'
                onClick={() => onOpen(r.id)}
                className='flex w-full flex-col gap-2 rounded-2xl border bg-card p-4 text-start transition-colors hover:border-line-hover focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'
              >
                <span className='flex items-center justify-between gap-2'>
                  <span className='truncate type-card-title text-fg'>
                    {r.room.code}
                  </span>
                  <ReportStatusBadge status={r.status} />
                </span>
                <span className='line-clamp-2 type-body text-fg'>
                  {r.description}
                </span>
                <span className='flex items-center gap-1.5 type-body-sm text-fg-secondary'>
                  <Clock className='size-3.5 shrink-0' aria-hidden />
                  <span className='truncate'>
                    {formatRelative(r.created_at, now)} · {r.reporter.name}
                  </span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
      {!isPending && !type && (total ?? 0) > (reports?.length ?? 0) && (
        <p className='px-2 type-body-sm text-fg-tertiary'>
          Menampilkan {reports?.length} laporan terbaru.
        </p>
      )}
    </section>
  )
}
