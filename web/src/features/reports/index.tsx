import { useState } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { AlertCircle, Camera, TriangleAlert, X } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type ReportStatus } from '@/lib/api-types'
import { formatDateTime } from '@/lib/datetime'
import { useHasRole } from '@/lib/rbac'
import { REPORT_STATUS_LABELS } from '@/lib/status-labels'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Page } from '@/components/layout/page'
import { Pager } from '@/components/pager'
import { ReportStatusBadge } from '@/components/status-badge'
import { useRooms } from '@/features/rooms/api'
import { useReports } from './api'
import { ReportDetailSheet } from './components/report-detail-sheet'
import { ReportFormDialog } from './components/report-form-dialog'

const route = getRouteApi('/_authenticated/reports/')
const ALL = 'ALL'
const PER_PAGE = 20
// Who sees every report rather than only their own (brief.md 6.2).
const SEES_ALL = ['TU', 'KALAB', 'ADMIN'] as const

export function Reports() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const seesAll = useHasRole(SEES_ALL)
  const [formOpen, setFormOpen] = useState(false)
  const rooms = useRooms({ per_page: 100 })
  const reports = useReports({
    page: search.page ?? 1,
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
  const cols = seesAll ? 6 : 5

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
      <Card className='mb-6'>
        <CardContent className='grid items-end gap-4 sm:grid-cols-3'>
          <div className='grid gap-2'>
            <Label htmlFor='rp-status'>Status</Label>
            <Select
              value={search.status ?? ALL}
              onValueChange={(v) =>
                filter({ status: v === ALL ? undefined : (v as ReportStatus) })
              }
            >
              <SelectTrigger id='rp-status' className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL}>Semua status</SelectItem>
                {Object.entries(REPORT_STATUS_LABELS).map(([value, label]) => (
                  <SelectItem key={value} value={value}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className='grid gap-2'>
            <Label htmlFor='rp-room'>Ruang</Label>
            <Select
              value={search.room_id ?? ALL}
              onValueChange={(v) =>
                filter({ room_id: v === ALL ? undefined : v })
              }
            >
              <SelectTrigger id='rp-room' className='w-full'>
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
          {hasFilters && (
            <Button
              variant='ghost'
              className='justify-self-start'
              onClick={() => filter({ status: undefined, room_id: undefined })}
            >
              <X />
              Hapus filter
            </Button>
          )}
        </CardContent>
      </Card>

      {reports.isError && (
        <Alert variant='destructive' className='mb-4'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(reports.error)}</AlertDescription>
        </Alert>
      )}

      <div className='overflow-x-auto rounded-xl border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Dilaporkan</TableHead>
              <TableHead>Ruang</TableHead>
              <TableHead>Kerusakan</TableHead>
              {seesAll && <TableHead>Pelapor</TableHead>}
              <TableHead>Status</TableHead>
              <TableHead>
                <span className='sr-only'>Aksi</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {reports.isPending &&
              Array.from({ length: 4 }, (_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={cols}>
                    <Skeleton className='h-6' />
                  </TableCell>
                </TableRow>
              ))}
            {reports.isSuccess && rows.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={cols}
                  className='py-10 text-center text-muted-foreground'
                >
                  {hasFilters
                    ? 'Tidak ada laporan yang cocok dengan filter.'
                    : 'Belum ada laporan kerusakan.'}
                </TableCell>
              </TableRow>
            )}
            {rows.map((r) => (
              <TableRow
                key={r.id}
                className='cursor-pointer'
                onClick={() => openDetail(r.id)}
              >
                <TableCell className='whitespace-nowrap'>
                  {formatDateTime(r.created_at)}
                </TableCell>
                <TableCell className='whitespace-nowrap'>
                  {r.room.code}
                  <div className='text-muted-foreground'>{r.room.name}</div>
                </TableCell>
                <TableCell className='max-w-80'>
                  <span className='flex items-center gap-2'>
                    <span className='truncate'>{r.description}</span>
                    {r.photo_path && (
                      <Camera
                        className='size-4 shrink-0 text-muted-foreground'
                        aria-label='Ada foto'
                      />
                    )}
                  </span>
                </TableCell>
                {seesAll && (
                  <TableCell className='whitespace-nowrap'>
                    {r.reporter.name}
                    <div className='text-muted-foreground'>
                      {r.reporter.identifier}
                    </div>
                  </TableCell>
                )}
                <TableCell>
                  <ReportStatusBadge status={r.status} />
                </TableCell>
                <TableCell className='text-end'>
                  <Button
                    variant='ghost'
                    size='sm'
                    onClick={(e) => {
                      e.stopPropagation()
                      openDetail(r.id)
                    }}
                  >
                    Detail
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <Pager
        meta={reports.data?.meta}
        onPageChange={(page) =>
          navigate({ search: (prev) => ({ ...prev, page }) })
        }
      />

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
