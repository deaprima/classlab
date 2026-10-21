import { Users } from 'lucide-react'
import { cn } from '@/lib/utils'
import { StatusPill, type PillStatus } from './status-pill'

type RoomTileProps = {
  /** Room code, e.g. "R-301". */
  code: string
  capacity: number
  status: PillStatus
  className?: string
}

/** Figma `Room Tile`: code, capacity and current availability. Wrap it in a Link where it navigates. */
export function RoomTile({ code, capacity, status, className }: RoomTileProps) {
  return (
    <div
      data-slot='room-tile'
      className={cn(
        'flex flex-col gap-3 rounded-xl border bg-card p-4 transition-colors hover:border-line-hover',
        className
      )}
    >
      <div className='flex items-start justify-between gap-2'>
        <p className='type-section text-fg'>{code}</p>
        <StatusPill status={status} />
      </div>
      <p className='flex items-center gap-1.5 type-body-sm text-fg-secondary'>
        <Users aria-hidden className='size-4' />
        {capacity} kursi
      </p>
    </div>
  )
}
