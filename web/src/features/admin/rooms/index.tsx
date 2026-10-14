import { useState } from 'react'
import { Link, getRouteApi } from '@tanstack/react-router'
import { AlertCircle, CalendarDays, Pencil, Plus, Trash2 } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type Room, type RoomType } from '@/lib/api-types'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
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
import { ConfirmDialog } from '@/components/confirm-dialog'
import { Page } from '@/components/layout/page'
import { Pager } from '@/components/pager'
import { SearchInput } from '@/components/search-input'
import { useRooms } from '@/features/rooms/api'
import { ROOM_TYPE_LABELS } from '@/features/rooms/labels'
import { useDeleteRoom } from './api'
import { RoomFormDialog } from './room-form-dialog'

const route = getRouteApi('/_authenticated/admin/rooms')
const ALL = 'ALL'

type Editing = { mode: 'create' } | { mode: 'edit'; room: Room } | null

export function AdminRooms() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const [editing, setEditing] = useState<Editing>(null)
  const [deleting, setDeleting] = useState<Room | null>(null)
  const rooms = useRooms({
    page: search.page ?? 1,
    per_page: 20,
    q: search.q,
    type: search.type,
    is_active:
      search.status === undefined ? undefined : search.status === 'active',
  })
  // Errors (e.g. 409 ROOM_HAS_ACTIVE_BOOKINGS) are shown by the default toast handler.
  const remove = useDeleteRoom({ onSuccess: () => setDeleting(null) })

  const filter = (patch: Partial<typeof search>) =>
    navigate({ search: (prev) => ({ ...prev, ...patch, page: undefined }) })
  const rows = rooms.data?.data ?? []

  return (
    <Page
      title='Kelola Ruang'
      description='Data ruang kelas dan laboratorium yang bisa dipinjam.'
      actions={
        <Button onClick={() => setEditing({ mode: 'create' })}>
          <Plus />
          Tambah Ruang
        </Button>
      }
    >
      <Card className='mb-6'>
        <CardContent className='grid gap-4 sm:grid-cols-3'>
          <SearchInput
            label='Cari ruang'
            placeholder='Cari kode atau nama'
            value={search.q}
            onCommit={(q) => filter({ q })}
          />
          <Select
            value={search.type ?? ALL}
            onValueChange={(v) =>
              filter({ type: v === ALL ? undefined : (v as RoomType) })
            }
          >
            <SelectTrigger className='w-full' aria-label='Tipe ruang'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Semua tipe</SelectItem>
              <SelectItem value='KELAS'>{ROOM_TYPE_LABELS.KELAS}</SelectItem>
              <SelectItem value='LAB'>{ROOM_TYPE_LABELS.LAB}</SelectItem>
            </SelectContent>
          </Select>
          <Select
            value={search.status ?? ALL}
            onValueChange={(v) =>
              filter({
                status: v === ALL ? undefined : (v as 'active' | 'inactive'),
              })
            }
          >
            <SelectTrigger className='w-full' aria-label='Status ruang'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Semua status</SelectItem>
              <SelectItem value='active'>Aktif</SelectItem>
              <SelectItem value='inactive'>Nonaktif</SelectItem>
            </SelectContent>
          </Select>
        </CardContent>
      </Card>

      {rooms.isError && (
        <Alert variant='destructive' className='mb-4'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(rooms.error)}</AlertDescription>
        </Alert>
      )}

      <div className='overflow-x-auto rounded-xl border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Kode</TableHead>
              <TableHead>Nama</TableHead>
              <TableHead>Tipe</TableHead>
              <TableHead className='text-end'>Kapasitas</TableHead>
              <TableHead>Fasilitas</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>
                <span className='sr-only'>Aksi</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rooms.isPending &&
              Array.from({ length: 4 }, (_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={7}>
                    <Skeleton className='h-6' />
                  </TableCell>
                </TableRow>
              ))}
            {rooms.isSuccess && rows.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className='py-10 text-center text-muted-foreground'
                >
                  Tidak ada ruang yang cocok.
                </TableCell>
              </TableRow>
            )}
            {rows.map((room) => (
              <TableRow key={room.id}>
                <TableCell className='font-medium'>{room.code}</TableCell>
                <TableCell>{room.name}</TableCell>
                <TableCell>{ROOM_TYPE_LABELS[room.type]}</TableCell>
                <TableCell className='text-end tabular-nums'>
                  {room.capacity}
                </TableCell>
                <TableCell className='max-w-64 truncate text-muted-foreground'>
                  {room.facilities.join(', ') || '–'}
                </TableCell>
                <TableCell>
                  {room.is_active ? (
                    <Badge variant='secondary'>Aktif</Badge>
                  ) : (
                    <Badge variant='outline'>Nonaktif</Badge>
                  )}
                </TableCell>
                <TableCell>
                  <div className='flex justify-end gap-1'>
                    <Button variant='ghost' size='icon' asChild>
                      <Link
                        to='/rooms/$id'
                        params={{ id: room.id }}
                        aria-label={`Jadwal ${room.code}`}
                      >
                        <CalendarDays />
                      </Link>
                    </Button>
                    <Button
                      variant='ghost'
                      size='icon'
                      aria-label={`Ubah ${room.code}`}
                      onClick={() => setEditing({ mode: 'edit', room })}
                    >
                      <Pencil />
                    </Button>
                    <Button
                      variant='ghost'
                      size='icon'
                      aria-label={`Hapus ${room.code}`}
                      onClick={() => setDeleting(room)}
                    >
                      <Trash2 />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <Pager
        meta={rooms.data?.meta}
        onPageChange={(page) =>
          navigate({ search: (prev) => ({ ...prev, page }) })
        }
      />

      <RoomFormDialog
        open={editing !== null}
        room={editing?.mode === 'edit' ? editing.room : undefined}
        onOpenChange={(o) => !o && setEditing(null)}
      />

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
