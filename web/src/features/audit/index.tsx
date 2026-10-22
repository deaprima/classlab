import { useMemo, useState } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { AlertCircle, ScrollText } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type AuditLog } from '@/lib/api-types'
import { formatDateTime } from '@/lib/datetime'
import { ROLE_LABELS } from '@/lib/rbac'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/empty-state'
import { FilterChip } from '@/components/filter-chip'
import { Page } from '@/components/layout/page'
import { ListFooter } from '@/components/list-footer'
import {
  type DataColumn,
  ResponsiveDataList,
} from '@/components/responsive-data-list'
import { useAuditLogs } from './api'
import {
  ENTITY_GROUPS,
  RANGES,
  RANGE_LABELS,
  type RangeKey,
  actionLabel,
  actorLabel,
  entityLabel,
  rangeStart,
  stateLines,
} from './rules'

const route = getRouteApi('/_authenticated/admin/audit')
const PER_PAGE = 10

/** "Audit Log" (Figma): who did what and when, with the state before and after of the selected entry. */
export function Audit() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const wide = useMediaQuery('(min-width: 1024px)')
  const now = useNow(60_000)
  const [popup, setPopup] = useState<AuditLog | null>(null)
  const range: RangeKey = search.range ?? 'week'
  // A minute is fine-grained enough to keep the query key stable while the page stays open.
  const from = useMemo(
    () =>
      rangeStart(range, new Date(Math.floor(now.getTime() / 60_000) * 60_000)),
    [range, now]
  )
  const logs = useAuditLogs({
    entity_type: search.entity,
    from,
    page: search.page ?? 1,
    per_page: PER_PAGE,
  })
  const rows = logs.data?.data ?? []
  const selected = rows.find((r) => r.id === search.log) ?? rows[0]

  const refine = (patch: Partial<typeof search>) =>
    navigate({
      search: (prev) => ({
        ...prev,
        page: undefined,
        log: undefined,
        ...patch,
      }),
    })
  const select = (log: AuditLog) => {
    if (wide) navigate({ search: (prev) => ({ ...prev, log: log.id }) })
    else setPopup(log)
  }

  const columns: DataColumn<AuditLog>[] = [
    {
      key: 'when',
      header: 'Waktu',
      primary: true,
      cell: (l) => (
        <span className='type-card-title text-fg'>
          {formatDateTime(l.created_at)}
        </span>
      ),
    },
    {
      key: 'actor',
      header: 'Aktor',
      cell: (l) => <span className='text-fg-secondary'>{actorLabel(l)}</span>,
    },
    {
      key: 'action',
      header: 'Aksi',
      cell: (l) => <span className='text-fg'>{actionLabel(l.action)}</span>,
    },
    {
      key: 'role',
      header: 'Peran',
      cell: (l) => (
        <span className='text-fg-secondary'>
          {l.actor_role ? ROLE_LABELS[l.actor_role] : 'Sistem'}
        </span>
      ),
    },
  ]

  return (
    <Page
      title='Audit Log'
      description='Catatan perubahan oleh pengguna dan sistem. Hanya bisa dibaca, tidak bisa diubah.'
    >
      <div className='grid grid-cols-1 items-start gap-5 lg:grid-cols-[minmax(0,1fr)_340px]'>
        <section
          aria-label='Aktivitas Administrasi'
          className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
        >
          <h2 className='type-card-title text-fg'>Aktivitas Administrasi</h2>
          <div className='flex flex-wrap items-center gap-2'>
            <FilterChip
              active={!search.entity}
              onClick={() => refine({ entity: undefined })}
            >
              Semua aksi
            </FilterChip>
            {ENTITY_GROUPS.map((g) => (
              <FilterChip
                key={g.value}
                active={search.entity === g.value}
                onClick={() => refine({ entity: g.value })}
              >
                {g.label}
              </FilterChip>
            ))}
            <Select
              value={range}
              onValueChange={(v) =>
                refine({ range: v === 'week' ? undefined : (v as RangeKey) })
              }
            >
              <SelectTrigger
                className='w-44 sm:ms-auto'
                aria-label='Rentang waktu'
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {RANGES.map((r) => (
                  <SelectItem key={r} value={r}>
                    {RANGE_LABELS[r]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {logs.isError && (
            <Alert variant='error'>
              <AlertCircle />
              <AlertDescription>{getErrorMessage(logs.error)}</AlertDescription>
            </Alert>
          )}

          {logs.isPending ? (
            <Skeleton className='h-80' />
          ) : rows.length === 0 ? (
            <EmptyState
              icon={ScrollText}
              title='Tidak ada catatan'
              description='Tidak ada aktivitas pada filter dan rentang ini.'
            />
          ) : (
            <>
              <ResponsiveDataList
                caption='Audit log'
                rows={rows}
                columns={columns}
                rowKey={(l) => l.id}
                onRowClick={select}
                isRowActive={wide ? (l) => l.id === selected?.id : undefined}
              />
              <ListFooter
                page={logs.data?.meta.page ?? 1}
                perPage={PER_PAGE}
                total={logs.data?.meta.total ?? 0}
                onPageChange={(page) =>
                  navigate({
                    search: (prev) => ({ ...prev, page, log: undefined }),
                  })
                }
              />
            </>
          )}
        </section>

        {wide && selected && (
          <aside
            aria-label='Detail aktivitas'
            className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
          >
            <h2 className='type-card-title text-fg'>Detail Aktivitas</h2>
            <LogDetail log={selected} />
          </aside>
        )}
      </div>

      <Dialog open={!!popup} onOpenChange={(o) => !o && setPopup(null)}>
        <DialogContent className='max-h-[90dvh] overflow-y-auto sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>Detail Aktivitas</DialogTitle>
            <DialogDescription className='sr-only'>
              Rincian satu catatan audit.
            </DialogDescription>
          </DialogHeader>
          {popup && <LogDetail log={popup} />}
        </DialogContent>
      </Dialog>
    </Page>
  )
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className='flex items-baseline justify-between gap-4'>
      <dt className='type-body text-fg-secondary'>{label}</dt>
      <dd className='min-w-0 text-end type-card-title break-words text-fg'>
        {value}
      </dd>
    </div>
  )
}

function LogDetail({ log }: { log: AuditLog }) {
  return (
    <div className='flex flex-col gap-4'>
      <p className='type-section text-fg'>{actionLabel(log.action)}</p>
      <dl className='flex flex-col gap-2'>
        <Row label='Waktu' value={formatDateTime(log.created_at)} />
        <Row
          label='Aktor'
          value={
            <>
              {actorLabel(log)}
              {log.actor_role && (
                <span className='block type-body-sm font-normal text-fg-secondary'>
                  {ROLE_LABELS[log.actor_role]}
                </span>
              )}
            </>
          }
        />
        <Row
          label='Entitas'
          value={
            <>
              {entityLabel(log.entity_type)}
              {log.entity_id && (
                <span className='block type-body-sm font-normal break-all text-fg-secondary'>
                  {log.entity_id}
                </span>
              )}
            </>
          }
        />
        {log.ip && <Row label='IP' value={log.ip} />}
        {log.request_id && <Row label='Request ID' value={log.request_id} />}
      </dl>
      <State title='Sebelum' lines={stateLines(log.before)} />
      <State title='Sesudah' lines={stateLines(log.after)} />
    </div>
  )
}

function State({ title, lines }: { title: string; lines: string[] }) {
  return (
    <section className='rounded-xl bg-subtle p-3'>
      <h3 className='type-overline text-fg-secondary'>{title}</h3>
      {lines.length === 0 ? (
        <p className='type-body text-fg-tertiary'>Tidak ada data.</p>
      ) : (
        <ul className='type-body break-words text-fg'>
          {lines.map((l, i) => (
            <li key={i}>{l}</li>
          ))}
        </ul>
      )}
    </section>
  )
}
