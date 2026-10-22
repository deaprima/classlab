import { useState } from 'react'
import { Link, getRouteApi } from '@tanstack/react-router'
import {
  AlertCircle,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  Pencil,
  Plus,
  Trash2,
} from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type TimelineItem } from '@/lib/api-types'
import {
  addDays,
  formatDate,
  formatDateShort,
  formatSlot,
  formatTimeRange,
  startOfWeek,
  toApiDateTime,
  todayWib,
} from '@/lib/datetime'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
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
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useRoomSchedule, useRooms } from '@/features/rooms/api'
import { useDeleteSchedule } from './api'
import {
  type ScheduleDialogState,
  ScheduleFormDialog,
} from './schedule-form-dialog'

const route = getRouteApi('/_authenticated/admin/rooms')

/**
 * The API has no schedule list endpoint; a room's week is read from its timeline
 * (`/rooms/{id}/schedule`), whose SCHEDULE items carry the schedule id (decisions.md, Phase 5).
 */
export function SchedulesTab() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const rooms = useRooms({ per_page: 100 })
  const roomList = rooms.data?.data ?? []
  const roomId = search.room_id ?? roomList[0]?.id
  const room = roomList.find((r) => r.id === roomId)
  const monday = startOfWeek(search.date ?? todayWib())
  const from = toApiDateTime(monday, '00:00')
  const to = toApiDateTime(addDays(monday, 7), '00:00')

  const [dialog, setDialog] = useState<ScheduleDialogState>(null)
  const [deleting, setDeleting] = useState<TimelineItem | null>(null)
  const remove = useDeleteSchedule({ onSuccess: () => setDeleting(null) })

  const go = (patch: Partial<typeof search>) =>
    navigate({ search: (prev) => ({ ...prev, ...patch }), replace: true })

  return (
    <div className='flex flex-col gap-4'>
      <p className='type-body-sm text-fg-secondary'>
        Jadwal kuliah dan kegiatan tetap per ruang. Peminjaman yang disetujui
        tampil terpisah dan tidak mengubah jadwal ini.
      </p>
      {roomId && (
        <div>
          <Button
            onClick={() =>
              setDialog({
                mode: 'create',
                roomId,
                date: monday < todayWib() ? todayWib() : monday,
              })
            }
          >
            <Plus />
            Tambah Jadwal
          </Button>
        </div>
      )}
      <Card>
        <CardContent className='flex flex-wrap items-end gap-4'>
          <div className='grid min-w-64 gap-2'>
            <Label htmlFor='sc-room'>Ruang</Label>
            <Select
              value={roomId ?? ''}
              onValueChange={(v) => go({ room_id: v })}
              disabled={rooms.isPending}
            >
              <SelectTrigger id='sc-room' className='w-full'>
                <SelectValue placeholder='Memuat ruang…' />
              </SelectTrigger>
              <SelectContent>
                {roomList.map((r) => (
                  <SelectItem key={r.id} value={r.id}>
                    {r.code} · {r.name}
                    {!r.is_active && ' (nonaktif)'}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className='flex items-end gap-1'>
            <Button
              variant='outline'
              size='icon'
              aria-label='Minggu sebelumnya'
              onClick={() => go({ date: addDays(monday, -7) })}
            >
              <ChevronLeft />
            </Button>
            <div className='grid gap-2'>
              <Label htmlFor='sc-date'>Minggu dari tanggal</Label>
              <Input
                id='sc-date'
                type='date'
                value={monday}
                onChange={(e) => e.target.value && go({ date: e.target.value })}
              />
            </div>
            <Button
              variant='outline'
              size='icon'
              aria-label='Minggu berikutnya'
              onClick={() => go({ date: addDays(monday, 7) })}
            >
              <ChevronRight />
            </Button>
          </div>
          {roomId && (
            <Button variant='ghost' asChild>
              <Link
                to='/rooms/$id'
                params={{ id: roomId }}
                search={{ date: monday }}
              >
                <CalendarDays />
                Lihat timeline
              </Link>
            </Button>
          )}
        </CardContent>
      </Card>

      {rooms.isError && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(rooms.error)}</AlertDescription>
        </Alert>
      )}

      {roomId && (
        <WeekTable
          roomId={roomId}
          from={from}
          to={to}
          title={`${room?.code ?? ''} · ${formatDate(from)} – ${formatDate(toApiDateTime(addDays(monday, 6), '12:00'))}`}
          onEdit={(item) => setDialog({ mode: 'edit', roomId, item })}
          onDelete={setDeleting}
        />
      )}

      <ScheduleFormDialog
        state={dialog}
        rooms={roomList}
        onClose={() => setDialog(null)}
      />
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(o) => !o && !remove.isPending && setDeleting(null)}
        title='Hapus jadwal?'
        desc={
          deleting ? (
            <p>
              “{deleting.title}”,{' '}
              {formatSlot(deleting.start_at, deleting.end_at)}. Hanya sesi ini
              yang dihapus; slotnya menjadi bisa dipinjam.
            </p>
          ) : (
            ''
          )
        }
        confirmText='Hapus'
        destructive
        isLoading={remove.isPending}
        handleConfirm={() => deleting && remove.mutate(deleting.id)}
      />
    </div>
  )
}

function WeekTable({
  roomId,
  from,
  to,
  title,
  onEdit,
  onDelete,
}: {
  roomId: string
  from: string
  to: string
  title: string
  onEdit: (item: TimelineItem) => void
  onDelete: (item: TimelineItem) => void
}) {
  const timeline = useRoomSchedule(roomId, from, to)
  const items = timeline.data ?? []
  const schedules = items.filter((i) => i.kind === 'SCHEDULE')
  const bookings = items.length - schedules.length

  return (
    <section>
      <div className='mb-2 flex flex-wrap items-baseline justify-between gap-2'>
        <h2 className='font-semibold'>{title}</h2>
        {bookings > 0 && (
          <p className='text-sm text-muted-foreground'>
            + {bookings} peminjaman disetujui minggu ini (lihat timeline).
          </p>
        )}
      </div>
      {timeline.isError && (
        <Alert variant='error' className='mb-4'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(timeline.error)}</AlertDescription>
        </Alert>
      )}
      <div className='overflow-x-auto rounded-xl border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Hari</TableHead>
              <TableHead>Jam</TableHead>
              <TableHead>Judul</TableHead>
              <TableHead>
                <span className='sr-only'>Aksi</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {timeline.isPending && (
              <TableRow>
                <TableCell colSpan={4}>
                  <Skeleton className='h-6' />
                </TableCell>
              </TableRow>
            )}
            {timeline.isSuccess && schedules.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={4}
                  className='py-10 text-center text-muted-foreground'
                >
                  Tidak ada jadwal di ruang ini pada minggu ini.
                </TableCell>
              </TableRow>
            )}
            {schedules.map((item) => (
              <TableRow key={item.id}>
                <TableCell className='whitespace-nowrap'>
                  {formatDateShort(item.start_at)}
                </TableCell>
                <TableCell className='whitespace-nowrap tabular-nums'>
                  {formatTimeRange(item.start_at, item.end_at)}
                </TableCell>
                <TableCell>{item.title}</TableCell>
                <TableCell>
                  <div className='flex justify-end gap-1'>
                    <Button
                      variant='ghost'
                      size='icon'
                      aria-label={`Ubah ${item.title}`}
                      onClick={() => onEdit(item)}
                    >
                      <Pencil />
                    </Button>
                    <Button
                      variant='ghost'
                      size='icon'
                      aria-label={`Hapus ${item.title}`}
                      onClick={() => onDelete(item)}
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
    </section>
  )
}
