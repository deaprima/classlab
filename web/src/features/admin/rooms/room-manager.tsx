import { useMemo, useState } from 'react'
import { Link, getRouteApi } from '@tanstack/react-router'
import {
  AlertCircle,
  CalendarClock,
  CalendarDays,
  DoorOpen,
  MoreHorizontal,
  Pencil,
  Plus,
  QrCode,
  Trash2,
} from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage } from '@/lib/api-error'
import { type Room, type RoomType } from '@/lib/api-types'
import { useMediaQuery } from '@/hooks/use-media-query'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { FilterChip } from '@/components/filter-chip'
import { Page } from '@/components/layout/page'
import { ListFooter } from '@/components/list-footer'
import {
  type DataColumn,
  ResponsiveDataList,
} from '@/components/responsive-data-list'
import { SearchInput } from '@/components/search-input'
import { useRoomTypes } from '@/features/room-types/api'
import { useRooms } from '@/features/rooms/api'
import { ROOM_TYPE_LABELS } from '@/features/rooms/labels'
import { useDeleteRoom } from './api'
import { RoomQrDialog } from './qr-dialog'
import { RoomForm, RoomFormDialog } from './room-form-dialog'

const route = getRouteApi('/_authenticated/admin/rooms')
const PER_PAGE = 20

type Editing = { mode: 'create' } | { mode: 'edit'; room: Room } | null

/** "Ruang & Jadwal" (Figma "TU – Ruang & Jadwal"): rooms and manual schedules, with the room editor beside the list. */
export function RoomManager() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const wide = useMediaQuery('(min-width: 1024px)')
  const user = useAuthStore((s) => s.auth.user)
  const policies = useRoomTypes()
  const [dialog, setDialog] = useState<Editing>(null)
  const [qrRoom, setQrRoom] = useState<Room | null>(null)
  const [deleting, setDeleting] = useState<Room | null>(null)
  // Errors (e.g. 409 ROOM_HAS_ACTIVE_BOOKINGS) are shown by the default toast handler.
  const remove = useDeleteRoom({ onSuccess: () => setDeleting(null) })

  const type = search.type
  const rooms = useRooms({
    page: search.page ?? 1,
    per_page: PER_PAGE,
    q: search.q,
    type,
  })
  const total = useRooms({ per_page: 1 })
  const rows = useMemo(() => rooms.data?.data ?? [], [rooms.data])
  const selected =
    rows.find((r) => r.id === search.room) ?? (wide ? rows[0] : undefined)

  // Chips offer the types this manager runs; the server enforces the same scope (ACCESS.manageRooms).
  const types = useMemo(
    () =>
      (policies.data ?? [])
        .filter((p) => !!user && p.approver_roles.includes(user.role))
        .map((p) => p.type),
    [policies.data, user]
  )
  const hoursOf = (t: RoomType) => {
    const p = policies.data?.find((x) => x.type === t)
    return p ? `${p.open_time}–${p.close_time}` : undefined
  }

  const refine = (patch: Partial<typeof search>) =>
    navigate({
      search: (prev) => ({
        ...prev,
        ...patch,
        page: undefined,
        room: undefined,
      }),
    })
  const edit = (room: Room) => {
    if (wide) navigate({ search: (prev) => ({ ...prev, room: room.id }) })
    else setDialog({ mode: 'edit', room })
  }

  const columns: DataColumn<Room>[] = [
    {
      key: 'code',
      header: 'Kode',
      primary: true,
      cell: (r) => <span className='type-card-title text-fg'>{r.code}</span>,
    },
    {
      key: 'type',
      header: 'Tipe',
      cell: (r) => (
        <span className='text-fg-secondary'>{ROOM_TYPE_LABELS[r.type]}</span>
      ),
    },
    {
      key: 'hours',
      header: 'Jam Operasional',
      cell: (r) => (
        <span className='text-fg-secondary tabular-nums'>
          {r.open_time && r.close_time
            ? `${r.open_time}–${r.close_time} (khusus)`
            : (hoursOf(r.type) ?? '–')}
        </span>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      cell: (r) =>
        r.is_active ? (
          <Badge variant='secondary'>Aktif</Badge>
        ) : (
          <Badge variant='outline'>Nonaktif</Badge>
        ),
    },
    {
      key: 'actions',
      header: 'Aksi',
      actions: true,
      cell: (r) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant='ghost'
              size='icon'
              aria-label={`Aksi ${r.code}`}
              onClick={(e) => e.stopPropagation()}
            >
              <MoreHorizontal />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align='end' onClick={(e) => e.stopPropagation()}>
            <DropdownMenuItem onSelect={() => edit(r)}>
              <Pencil />
              Ubah
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setQrRoom(r)}>
              <QrCode />
              QR check-in
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <Link to='/rooms/$id' params={{ id: r.id }}>
                <CalendarDays />
                Lihat timeline
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              variant='destructive'
              onSelect={() => setDeleting(r)}
            >
              <Trash2 />
              Hapus
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ]

  return (
    <Page
      title='Ruang & Jadwal'
      description='Kelola ruang, jam operasional, QR check-in, dan jadwal tetap.'
    >
      <div className='grid grid-cols-1 items-start gap-5 lg:grid-cols-[minmax(0,1fr)_340px]'>
        <section
          aria-label='Kelola Ruang'
          className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
        >
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <h2 className='type-card-title text-fg'>
              Kelola Ruang
              {total.data && (
                <span className='ms-2 type-body-sm text-fg-secondary'>
                  {total.data.meta.total} ruang
                </span>
              )}
            </h2>
            <div className='flex flex-wrap items-center gap-2'>
              <Button variant='secondary' asChild>
                <Link to='/today'>
                  <CalendarClock />
                  Papan Ruang
                </Link>
              </Button>
              <Button onClick={() => setDialog({ mode: 'create' })}>
                <Plus />
                Tambah Ruang
              </Button>
            </div>
          </div>

          {
            <>
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
                <div className='min-w-48 flex-1 sm:ms-auto sm:max-w-64'>
                  <SearchInput
                    label='Cari ruang'
                    placeholder='Cari kode atau nama'
                    value={search.q}
                    onCommit={(q) => refine({ q })}
                  />
                </div>
              </div>

              {rooms.isError && (
                <Alert variant='error'>
                  <AlertCircle />
                  <AlertDescription>
                    {getErrorMessage(rooms.error)}
                  </AlertDescription>
                </Alert>
              )}
              {rooms.isPending ? (
                <Skeleton className='h-64' />
              ) : rows.length === 0 ? (
                <EmptyState
                  icon={DoorOpen}
                  title='Tidak ada ruang yang cocok'
                  description='Ubah filter atau tambahkan ruang baru.'
                />
              ) : (
                <>
                  <ResponsiveDataList
                    caption='Daftar ruang'
                    rows={rows}
                    columns={columns}
                    rowKey={(r) => r.id}
                    onRowClick={edit}
                    isRowActive={
                      wide ? (r) => r.id === selected?.id : undefined
                    }
                  />
                  <ListFooter
                    page={rooms.data?.meta.page ?? 1}
                    perPage={PER_PAGE}
                    total={rooms.data?.meta.total ?? 0}
                    onPageChange={(page) =>
                      navigate({
                        search: (prev) => ({ ...prev, page, room: undefined }),
                      })
                    }
                  />
                </>
              )}
            </>
          }
        </section>

        {wide && selected && (
          <aside
            aria-label='Edit ruang'
            className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
          >
            <h2 className='type-card-title text-fg'>Edit Ruang</h2>
            <RoomForm
              key={`${selected.id}:${selected.open_time}:${selected.close_time}`}
              room={selected}
              defaultHours={hoursOf}
              onDone={() => undefined}
              onCancel={() =>
                navigate({ search: (prev) => ({ ...prev, room: undefined }) })
              }
              extra={
                <QrCard room={selected} onOpen={() => setQrRoom(selected)} />
              }
            />
          </aside>
        )}
      </div>

      <RoomFormDialog
        open={dialog !== null}
        room={dialog?.mode === 'edit' ? dialog.room : undefined}
        defaultHours={hoursOf}
        onOpenChange={(o) => !o && setDialog(null)}
      />
      <RoomQrDialog room={qrRoom} onOpenChange={(o) => !o && setQrRoom(null)} />
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(o) => !o && !remove.isPending && setDeleting(null)}
        title={`Hapus ${deleting?.code ?? ''}?`}
        desc={
          <p>
            Ruang tidak lagi tampil dan tidak bisa dibooking. Jadwal dan riwayat
            peminjamannya tetap tersimpan. Penghapusan ditolak bila masih ada
            peminjaman aktif yang akan datang; nonaktifkan ruang bila hanya
            ingin menutupnya sementara.
          </p>
        }
        confirmText='Hapus'
        destructive
        isLoading={remove.isPending}
        handleConfirm={() => deleting && remove.mutate(deleting)}
      />
    </Page>
  )
}

/** The QR card of the side panel (Figma "QR Check-in"); printing and replacing the token live in its dialog. */
function QrCard({ room, onOpen }: { room: Room; onOpen: () => void }) {
  return (
    <div className='flex flex-col gap-3 rounded-xl border p-4'>
      <h3 className='type-card-title text-fg'>QR Check-in</h3>
      <p className='type-body-sm text-fg-secondary'>
        Ditempel di pintu. Ganti token bila QR bocor; token lama langsung tidak
        berlaku.
      </p>
      <Button variant='outline' onClick={onOpen}>
        <QrCode />
        Cetak QR / Ganti Token
        <span className='sr-only'> {room.code}</span>
      </Button>
    </div>
  )
}
