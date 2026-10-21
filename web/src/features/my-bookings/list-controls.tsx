import { type RoomType } from '@/lib/api-types'
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
