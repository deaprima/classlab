import { useMemo, useState } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { AlertCircle, ClipboardCheck } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage } from '@/lib/api-error'
import { formatTimeRange } from '@/lib/datetime'
import { useMediaQuery } from '@/hooks/use-media-query'
import { useNow } from '@/hooks/use-now'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/empty-state'
import { FilterChip } from '@/components/filter-chip'
import { Page } from '@/components/layout/page'
import { ListFooter } from '@/components/list-footer'
import {
  ResponsiveDataList,
  type DataColumn,
} from '@/components/responsive-data-list'
import { SearchInput } from '@/components/search-input'
import { SegmentedControl } from '@/components/segmented-control'
import { BookingStatusBadge } from '@/components/status-badge'
import { useBookings } from '@/features/bookings/api'
import { dayShort } from '@/features/my-bookings/format'
import { useRoomTypes } from '@/features/room-types/api'
import { ROOM_TYPE_LABELS } from '@/features/rooms/labels'
import { ApprovalDetail, ReasonPill } from './approval-detail'
import { type QueueItem, groupQueue, matchesQueue } from './rules'

const route = getRouteApi('/_authenticated/approvals/')
const PER_PAGE = 10
// The API has no text search, so the queue of a tab is loaded whole (up to this many) and
// searched and paged here. That also lets every date of a multi-date request fold into one line.
const LOAD_LIMIT = 100

type Tab = 'MENUNGGU' | 'DISETUJUI' | 'DITOLAK'

/** "Approval Pengajuan" (Figma "TU – Approval"): the queue on the left, the selected request on the right. */
export function Approvals() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const now = useNow()
  const wide = useMediaQuery('(min-width: 1024px)')
  const user = useAuthStore((s) => s.auth.user)
  const policies = useRoomTypes()
  const [popupId, setPopupId] = useState<string | null>(null)

  const tab: Tab = search.status ?? 'MENUNGGU'
  const type = search.type

  // The chips offer the room types this approver decides on; the server enforces the same scope.
  const types = useMemo(
    () =>
      (policies.data ?? [])
        .filter((p) => !!user && p.approver_roles.includes(user.role))
        .map((p) => p.type),
    [policies.data, user]
  )

  const queue = useBookings({ status: tab, type, per_page: LOAD_LIMIT })
  const waiting = useBookings({ status: 'MENUNGGU', type, per_page: 1 })
  const approved = useBookings({ status: 'DISETUJUI', type, per_page: 1 })
  const refused = useBookings({ status: 'DITOLAK', type, per_page: 1 })

  const all = useMemo(
    () =>
      groupQueue(
        (queue.data?.data ?? []).filter((b) => matchesQueue(b, search.q))
      ),
    [queue.data, search.q]
  )
  const pages = Math.max(1, Math.ceil(all.length / PER_PAGE))
  const page = Math.min(search.page ?? 1, pages)
  const items = all.slice((page - 1) * PER_PAGE, page * PER_PAGE)
  const loadedAll =
    (queue.data?.meta.total ?? 0) <= (queue.data?.data.length ?? 0)
  const selectedId =
    items.find((i) => i.lead.id === search.booking)?.lead.id ??
    items[0]?.lead.id

  const refine = (patch: Partial<typeof search>) =>
    navigate({
      search: (prev) => ({
        ...prev,
        page: undefined,
        booking: undefined,
        ...patch,
      }),
    })
  const select = (item: QueueItem) => {
    if (wide)
      navigate({ search: (prev) => ({ ...prev, booking: item.lead.id }) })
    else setPopupId(item.lead.id)
  }

  const columns: DataColumn<QueueItem>[] = [
    {
      key: 'room',
      header: 'Ruang',
      primary: true,
      cell: ({ lead }) => (
        <span className='type-card-title text-fg'>{lead.room.code}</span>
      ),
    },
    {
      key: 'requester',
      header: 'Peminjam',
      cell: ({ lead }) => (
        <span className='text-fg-secondary'>{lead.requester.name}</span>
      ),
    },
    {
      key: 'when',
      header: 'Waktu',
      cell: ({ lead, count }) => (
        <span className='text-fg-secondary'>
          {dayShort(lead.start_at)} ·{' '}
          {formatTimeRange(lead.start_at, lead.end_at)}
          {count > 1 && (
            <span className='block type-body-sm text-fg-tertiary'>
              +{count - 1} tanggal lain
            </span>
          )}
        </span>
      ),
    },
    tab === 'MENUNGGU'
      ? {
          key: 'reason',
          header: 'Alasan Manual',
          cell: ({ lead }) => <ReasonPill reason={lead.manual_reason} />,
        }
      : {
          key: 'status',
          header: 'Status',
          cell: ({ lead }) => <BookingStatusBadge status={lead.status} />,
        },
  ]

  return (
    <Page
      title='Approval Pengajuan'
      description={
        waiting.data
          ? `${waiting.data.meta.total ?? 0} pengajuan menunggu keputusan.`
          : 'Pengajuan yang menunggu keputusanmu.'
      }
    >
      <div className='grid grid-cols-1 items-start gap-5 lg:grid-cols-[minmax(0,1fr)_340px]'>
        <section
          aria-label='Antrean Approval'
          className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
        >
          <h2 className='type-card-title text-fg'>Antrean Approval</h2>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <SegmentedControl<Tab>
              aria-label='Filter status'
              value={tab}
              onValueChange={(t) =>
                refine({ status: t === 'MENUNGGU' ? undefined : t })
              }
              options={[
                {
                  value: 'MENUNGGU',
                  label: 'Menunggu',
                  count: waiting.data?.meta.total,
                },
                {
                  value: 'DISETUJUI',
                  label: 'Disetujui',
                  count: approved.data?.meta.total,
                },
                {
                  value: 'DITOLAK',
                  label: 'Ditolak',
                  count: refused.data?.meta.total,
                },
              ]}
            />
            <div className='min-w-48 flex-1 sm:max-w-64'>
              <SearchInput
                label='Cari pengajuan'
                placeholder='Cari ruang atau peminjam'
                value={search.q}
                onCommit={(q) => refine({ q })}
              />
            </div>
          </div>

          <div className='flex flex-wrap items-center gap-2'>
            <FilterChip
              active={!type}
              onClick={() => refine({ type: undefined })}
            >
              Semua tipe
            </FilterChip>
            {types.map((t) => (
              <FilterChip
                key={t}
                active={type === t}
                onClick={() => refine({ type: t })}
              >
                {ROOM_TYPE_LABELS[t]}
              </FilterChip>
            ))}
            {tab === 'MENUNGGU' && (
              <span className='type-body-sm text-fg-secondary sm:ms-auto'>
                Urut: sisa waktu kedaluwarsa
              </span>
            )}
          </div>

          {queue.isError && (
            <Alert variant='error'>
              <AlertCircle />
              <AlertDescription>
                {getErrorMessage(queue.error)}
              </AlertDescription>
            </Alert>
          )}

          {queue.isPending ? (
            <Skeleton className='h-64' />
          ) : items.length === 0 ? (
            <EmptyState
              icon={ClipboardCheck}
              title={
                search.q
                  ? 'Tidak ada yang cocok'
                  : tab === 'MENUNGGU'
                    ? 'Tidak ada yang menunggu'
                    : 'Belum ada data'
              }
              description={
                search.q
                  ? 'Ubah kata kunci atau hapus pencarian.'
                  : tab === 'MENUNGGU'
                    ? 'Semua pengajuan sudah diputuskan.'
                    : 'Pengajuan dengan status ini tampil di sini.'
              }
            />
          ) : (
            <>
              <ResponsiveDataList
                caption='Antrean approval'
                rows={items}
                columns={columns}
                rowKey={(i) => i.lead.id}
                onRowClick={select}
                isRowActive={wide ? (i) => i.lead.id === selectedId : undefined}
              />
              {!loadedAll && (
                <p className='type-body-sm text-fg-tertiary'>
                  Menampilkan {queue.data?.data.length} pengajuan teratas;
                  persempit dengan filter.
                </p>
              )}
              <ListFooter
                page={page}
                perPage={PER_PAGE}
                total={all.length}
                onPageChange={(p) =>
                  navigate({
                    search: (prev) => ({
                      ...prev,
                      page: p,
                      booking: undefined,
                    }),
                  })
                }
              />
            </>
          )}
        </section>

        {wide && selectedId && (
          <aside
            aria-label='Detail pengajuan'
            className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
          >
            <h2 className='type-card-title text-fg'>Detail Pengajuan</h2>
            <ApprovalDetail key={selectedId} bookingId={selectedId} now={now} />
          </aside>
        )}
      </div>

      <Dialog open={!!popupId} onOpenChange={(o) => !o && setPopupId(null)}>
        <DialogContent className='max-h-[90dvh] overflow-y-auto sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>Detail Pengajuan</DialogTitle>
            <DialogDescription className='sr-only'>
              Rincian pengajuan dan tombol keputusan.
            </DialogDescription>
          </DialogHeader>
          {popupId && (
            <ApprovalDetail
              key={popupId}
              bookingId={popupId}
              now={now}
              onDecided={() => setPopupId(null)}
            />
          )}
        </DialogContent>
      </Dialog>
    </Page>
  )
}
