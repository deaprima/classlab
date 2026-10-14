import { formatSlot, formatTimeRange } from '@/lib/datetime'
import { cn } from '@/lib/utils'

export type ConflictItem = {
  kind?: string
  title?: string
  status?: string
  start_at?: string
  end_at?: string
}

function describeKind(item: ConflictItem): string {
  if (item.kind === 'SCHEDULE') return 'Jadwal kuliah'
  if (item.kind === 'BOOKING') {
    if (item.status === 'MENUNGGU') return 'Pengajuan menunggu persetujuan'
    if (item.status === 'DISETUJUI') return 'Peminjaman disetujui'
    return 'Peminjaman'
  }
  return 'Kegiatan lain'
}

/** What occupies a slot: used for availability cards and for 409 BOOKING_CONFLICT details. */
export function ConflictList({
  items,
  withDate = false,
  className,
}: {
  items: ConflictItem[]
  withDate?: boolean
  className?: string
}) {
  if (items.length === 0) return null
  return (
    <ul className={cn('space-y-1 text-sm', className)}>
      {items.map((item, i) => (
        <li key={`${item.kind}-${item.start_at}-${i}`}>
          {item.start_at && item.end_at && (
            <span className='font-medium tabular-nums'>
              {withDate
                ? formatSlot(item.start_at, item.end_at)
                : formatTimeRange(item.start_at, item.end_at)}
            </span>
          )}{' '}
          <span className='text-muted-foreground'>
            · {describeKind(item)}
            {item.title ? `: ${item.title}` : ''}
          </span>
        </li>
      ))}
    </ul>
  )
}
