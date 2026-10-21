import { ChevronLeft, ChevronRight } from 'lucide-react'
import { type RoomType } from '@/lib/api-types'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ROOM_TYPES, ROOM_TYPE_LABELS } from '@/features/rooms/labels'

const ALL = 'ALL'

/** "Semua tipe" plus the five room types. */
export function RoomTypeFilter({
  value,
  onChange,
}: {
  value: RoomType | undefined
  onChange: (value: RoomType | undefined) => void
}) {
  return (
    <Select
      value={value ?? ALL}
      onValueChange={(v) => onChange(v === ALL ? undefined : (v as RoomType))}
    >
      <SelectTrigger aria-label='Tipe ruang' className='w-44'>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>Semua tipe</SelectItem>
        {ROOM_TYPES.map((t) => (
          <SelectItem key={t} value={t}>
            {ROOM_TYPE_LABELS[t]}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

/** "1–10 dari 23" with previous/next icon buttons; hidden when everything fits on one page. */
export function ListFooter({
  page,
  perPage,
  total,
  onPageChange,
}: {
  page: number
  perPage: number
  total: number
  onPageChange: (page: number) => void
}) {
  // One page needs no controls.
  if (total <= perPage) return null
  const pages = Math.ceil(total / perPage)
  const from = (page - 1) * perPage + 1
  const to = Math.min(total, page * perPage)
  return (
    <nav aria-label='Halaman' className='flex items-center justify-end gap-3'>
      <p className='type-body-sm text-fg-secondary'>
        {from}–{to} dari {total}
      </p>
      <Button
        variant='secondary'
        size='icon'
        aria-label='Halaman sebelumnya'
        disabled={page <= 1}
        onClick={() => onPageChange(page - 1)}
      >
        <ChevronLeft />
      </Button>
      <Button
        variant='secondary'
        size='icon'
        aria-label='Halaman berikutnya'
        disabled={page >= pages}
        onClick={() => onPageChange(page + 1)}
      >
        <ChevronRight />
      </Button>
    </nav>
  )
}
