import { useMemo, useState } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { AlertCircle, ImageOff, ShieldCheck } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getApiError, getErrorMessage } from '@/lib/api-error'
import { type Checkout, type RoomType } from '@/lib/api-types'
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
import { StatusPill } from '@/components/status-pill'
import { useRoomTypes } from '@/features/room-types/api'
import { useRooms } from '@/features/rooms/api'
import { ROOM_TYPES, ROOM_TYPE_LABELS } from '@/features/rooms/labels'
import { useCheckoutPhoto, useCheckouts } from './api'
import {
  RANGES,
  RANGE_LABELS,
  type RangeKey,
  filterByType,
  rangeFor,
  whenLabel,
} from './rules'

const route = getRouteApi('/_authenticated/verify')
const PER_PAGE = 6
// The API has no type filter, so the range is loaded whole (up to this many) and the type chips
// and paging work on it here.
const LOAD_LIMIT = 100
const ALL_ROOMS = 'ALL'

/** "Verifikasi Check-out" (Figma): the latest check-outs with the photo of how each room was left. */
export function Verify() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const user = useAuthStore((s) => s.auth.user)
  const policies = useRoomTypes()
  const rooms = useRooms({ per_page: 100 })
  const now = useNow(60_000)
  const [opened, setOpened] = useState<Checkout | null>(null)

  const range: RangeKey = search.range ?? 'today'
  const type = search.type
  // A minute is fine-grained enough to keep the query key stable while the page stays open.
  const window = useMemo(
    () =>
      rangeFor(range, new Date(Math.floor(now.getTime() / 60_000) * 60_000)),
    [range, now]
  )
  const checkouts = useCheckouts({
    ...window,
    per_page: LOAD_LIMIT,
    room_id: search.room,
  })

  // TU and Kalab only get the types they approve; OB and Admin see every type.
  const types = useMemo<RoomType[]>(() => {
    if (user?.role !== 'TU' && user?.role !== 'KALAB') return [...ROOM_TYPES]
    return (policies.data ?? [])
      .filter((p) => p.approver_roles.includes(user.role))
      .map((p) => p.type)
  }, [policies.data, user])

  const all = useMemo(
    () => filterByType(checkouts.data?.data ?? [], type),
    [checkouts.data, type]
  )
  const pages = Math.max(1, Math.ceil(all.length / PER_PAGE))
  const page = Math.min(search.page ?? 1, pages)
  const shown = all.slice((page - 1) * PER_PAGE, page * PER_PAGE)
  const loadedAll =
    (checkouts.data?.meta.total ?? 0) <= (checkouts.data?.data.length ?? 0)

  const refine = (patch: Partial<typeof search>) =>
    navigate({ search: (prev) => ({ ...prev, page: undefined, ...patch }) })

  return (
    <Page
      title='Verifikasi Check-out'
      description='Foto kondisi ruang setelah dipakai, untuk memastikan ruang ditinggalkan dengan baik.'
    >
      <section
        aria-label='Check-out Terbaru'
        className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
      >
        <h2 className='type-card-title text-fg'>Check-out Terbaru</h2>
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
          <div className='flex gap-2 sm:ms-auto'>
            <Select
              value={search.room ?? ALL_ROOMS}
              onValueChange={(v) =>
                refine({ room: v === ALL_ROOMS ? undefined : v })
              }
            >
              <SelectTrigger className='w-40' aria-label='Ruang'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_ROOMS}>Semua ruang</SelectItem>
                {(rooms.data?.data ?? []).map((r) => (
                  <SelectItem key={r.id} value={r.id}>
                    {r.code}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              value={range}
              onValueChange={(v) =>
                refine({ range: v === 'today' ? undefined : (v as RangeKey) })
              }
            >
              <SelectTrigger className='w-40' aria-label='Rentang waktu'>
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
        </div>

        {checkouts.isError && (
          <Alert variant='error'>
            <AlertCircle />
            <AlertDescription>
              {getErrorMessage(checkouts.error)}
            </AlertDescription>
          </Alert>
        )}

        {checkouts.isPending ? (
          <Skeleton className='h-80' />
        ) : shown.length === 0 ? (
          <EmptyState
            icon={ShieldCheck}
            title='Belum ada check-out'
            description='Check-out pada rentang dan filter ini tampil di sini.'
          />
        ) : (
          <>
            <ul className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3'>
              {shown.map((c) => (
                <li key={c.booking_id}>
                  <CheckoutCard checkout={c} now={now} onOpen={setOpened} />
                </li>
              ))}
            </ul>
            {!loadedAll && (
              <p className='type-body-sm text-fg-tertiary'>
                Menampilkan {checkouts.data?.data.length} check-out terbaru;
                persempit dengan filter.
              </p>
            )}
            <ListFooter
              page={page}
              perPage={PER_PAGE}
              total={all.length}
              onPageChange={(p) =>
                navigate({ search: (prev) => ({ ...prev, page: p }) })
              }
            />
          </>
        )}
      </section>

      <Dialog open={!!opened} onOpenChange={(o) => !o && setOpened(null)}>
        <DialogContent className='max-h-[90dvh] overflow-y-auto sm:max-w-2xl'>
          {opened && (
            <>
              <DialogHeader>
                <DialogTitle>
                  {opened.room_code} · {opened.room_name}
                </DialogTitle>
                <DialogDescription>
                  {whenLabel(opened.checked_out_at, now)}
                </DialogDescription>
              </DialogHeader>
              <Photo checkout={opened} large />
              {opened.note && (
                <p className='type-body break-words text-fg-secondary'>
                  Catatan: {opened.note}
                </p>
              )}
            </>
          )}
        </DialogContent>
      </Dialog>
    </Page>
  )
}

function ConditionPill({ condition }: { condition: Checkout['condition'] }) {
  return condition === 'ADA_MASALAH' ? (
    <StatusPill status='Diajukan' label='Ada masalah' />
  ) : (
    <StatusPill status='Tersedia' label='Normal' />
  )
}

function CheckoutCard({
  checkout: c,
  now,
  onOpen,
}: {
  checkout: Checkout
  now: Date
  onOpen: (c: Checkout) => void
}) {
  return (
    <article className='flex h-full flex-col gap-3 rounded-2xl border p-3'>
      <button
        type='button'
        onClick={() => onOpen(c)}
        aria-label={`Perbesar foto ${c.room_code}`}
        className='block w-full overflow-hidden rounded-xl'
      >
        <Photo checkout={c} />
      </button>
      <div className='flex items-start justify-between gap-3 px-1'>
        <div className='min-w-0'>
          <p className='truncate type-card-title text-fg'>{c.room_code}</p>
          <p className='truncate type-body-sm text-fg-secondary'>
            {c.room_name} · {whenLabel(c.checked_out_at, now)}
          </p>
        </div>
        <ConditionPill condition={c.condition} />
      </div>
      {c.note && (
        <p className='line-clamp-2 px-1 type-body-sm text-fg-secondary'>
          {c.note}
        </p>
      )}
    </article>
  )
}

/** The photo of one check-out; a gray block while it loads, and a note when it is gone. */
function Photo({ checkout, large }: { checkout: Checkout; large?: boolean }) {
  const photo = useCheckoutPhoto(checkout.attachment_id)
  if (photo.error) {
    const gone = getApiError(photo.error)?.code === 'GONE'
    return (
      <div className='flex h-40 flex-col items-center justify-center gap-2 rounded-xl bg-subtle px-4 text-center type-body-sm text-fg-secondary'>
        <ImageOff className='size-6' />
        {gone
          ? 'Foto sudah dihapus sesuai kebijakan retensi.'
          : 'Foto gagal dimuat.'}
      </div>
    )
  }
  if (!photo.url) return <Skeleton className='h-40 w-full rounded-xl' />
  return (
    <img
      src={photo.url}
      alt={`Foto kondisi ruang ${checkout.room_code}`}
      className={
        large
          ? 'w-full rounded-xl object-contain'
          : 'h-40 w-full rounded-xl object-cover'
      }
    />
  )
}
