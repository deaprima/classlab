import { useState } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { AlertCircle, Camera, ClipboardList, TriangleAlert } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type DamageReport, type ReportStatus } from '@/lib/api-types'
import { formatDateTime } from '@/lib/datetime'
import { useHasRole } from '@/lib/rbac'
import { REPORT_STATUS_LABELS } from '@/lib/status-labels'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/layout/page'
import { ListFooter } from '@/components/list-footer'
import {
  ResponsiveDataList,
  type DataColumn,
} from '@/components/responsive-data-list'
import { SegmentedControl } from '@/components/segmented-control'
import { ReportStatusBadge } from '@/components/status-badge'
import { useRooms } from '@/features/rooms/api'
import { useReports } from './api'
import { ReportDetailSheet } from './components/report-detail-sheet'
import { ReportFormDialog } from './components/report-form-dialog'

const route = getRouteApi('/_authenticated/reports/')
const ALL = 'ALL'
const PER_PAGE = 10
// Who sees every report rather than only their own (brief.md 6.2).
const SEES_ALL = ['TU', 'KALAB', 'ADMIN'] as const

type Tab = ReportStatus | typeof ALL

/** Laporan Kerusakan (Figma): the reader's reports (managers: every room) with status and room filters. */
export function Reports() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const seesAll = useHasRole(SEES_ALL)
  const [formOpen, setFormOpen] = useState(false)
  const rooms = useRooms({ per_page: 100 })
  const page = search.page ?? 1
  const reports = useReports({
    page,
    per_page: PER_PAGE,
    status: search.status,
    room_id: search.room_id,
  })

  const filter = (patch: Partial<typeof search>) =>
    navigate({ search: (prev) => ({ ...prev, ...patch, page: undefined }) })
  const openDetail = (id: string | undefined) =>
    navigate({ search: (prev) => ({ ...prev, report: id }) })
  const hasFilters = !!(search.status || search.room_id)
  const rows = reports.data?.data ?? []

  const columns: DataColumn<DamageReport>[] = [
    {
      key: 'room',
      header: 'Ruang',
      primary: true,
      cell: (r) => (
        <span className='type-card-title text-fg'>{r.room.code}</span>
      ),
    },
    {
      key: 'damage',
      header: 'Kerusakan',
      cell: (r) => (
        <span className='flex items-center gap-2 text-fg-secondary'>
          <span className='line-clamp-2'>{r.description}</span>
          {r.attachment_id && (
            <Camera
              className='size-4 shrink-0 text-fg-tertiary'
              aria-label='Ada foto'
            />
          )}
        </span>
      ),
    },
    ...(seesAll
      ? [
          {
            key: 'reporter',
            header: 'Pelapor',
            cell: (r: DamageReport) => (
              <span className='text-fg-secondary'>{r.reporter.name}</span>
            ),
          },
        ]
      : []),
    {
      key: 'when',
      header: 'Dilaporkan',
      cell: (r) => (
        <span className='text-fg-secondary'>
          {formatDateTime(r.created_at)}
        </span>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      cell: (r) => <ReportStatusBadge status={r.status} />,
    },
  ]

  return (
    <Page
      title='Laporan Kerusakan'
      description={
        seesAll
          ? 'Semua laporan kondisi dan kerusakan ruang.'
          : 'Laporkan kerusakan ruang dan pantau penanganannya.'
      }
      actions={
        <Button onClick={() => setFormOpen(true)}>
          <TriangleAlert />
          Buat Laporan
        </Button>
      }
    >
      <section
        aria-label='Daftar laporan'
        className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
      >
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <SegmentedControl<Tab>
            aria-label='Filter status'
            value={search.status ?? ALL}
            onValueChange={(t) => filter({ status: t === ALL ? undefined : t })}
            options={[
              { value: ALL, label: 'Semua' },
              ...(
                Object.entries(REPORT_STATUS_LABELS) as [ReportStatus, string][]
              ).map(([value, label]) => ({ value, label })),
            ]}
          />
          <Select
            value={search.room_id ?? ALL}
            onValueChange={(v) =>
              filter({ room_id: v === ALL ? undefined : v })
            }
          >
            <SelectTrigger aria-label='Ruang' className='w-48'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Semua ruang</SelectItem>
              {rooms.data?.data.map((room) => (
                <SelectItem key={room.id} value={room.id}>
                  {room.code} · {room.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {reports.isError && (
          <Alert variant='error'>
            <AlertCircle />
            <AlertDescription>
              {getErrorMessage(reports.error)}
            </AlertDescription>
          </Alert>
        )}

        {reports.isPending ? (
          <Skeleton className='h-64' />
        ) : rows.length === 0 ? (
          <EmptyState
            icon={ClipboardList}
            title={
              hasFilters
                ? 'Tidak ada laporan yang cocok'
                : 'Belum ada laporan kerusakan'
            }
            description={
              hasFilters
                ? 'Ubah atau hapus filter untuk melihat laporan lain.'
                : 'Laporan yang kamu buat tampil di sini beserta penanganannya.'
            }
            action={
              hasFilters ? (
                <Button
                  variant='secondary'
                  onClick={() =>
                    filter({ status: undefined, room_id: undefined })
                  }
                >
                  Hapus filter
                </Button>
              ) : undefined
            }
          />
        ) : (
          <>
            <ResponsiveDataList
              caption='Daftar laporan kerusakan'
              rows={rows}
              columns={columns}
              rowKey={(r) => r.id}
              onRowClick={(r) => openDetail(r.id)}
              isRowActive={(r) => r.id === search.report}
            />
            <ListFooter
              page={page}
              perPage={PER_PAGE}
              total={reports.data?.meta?.total ?? 0}
              onPageChange={(p) =>
                navigate({ search: (prev) => ({ ...prev, page: p }) })
              }
            />
          </>
        )}
      </section>

      <ReportFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        onCreated={(report) => {
          setFormOpen(false)
          openDetail(report.id)
        }}
      />
      <ReportDetailSheet
        reportId={search.report}
        onClose={() => openDetail(undefined)}
      />
    </Page>
  )
}
