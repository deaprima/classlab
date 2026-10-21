import { cn } from '@/lib/utils'

type SlotStatus = 'Tersedia' | 'Digunakan' | 'Diajukan' | 'Terbatas'

const SLOT_STYLE: Record<SlotStatus, string> = {
  Tersedia: 'border border-line bg-surface text-fg-secondary',
  Digunakan: 'bg-inverse text-fg-inverse',
  Diajukan: 'border border-line-accent-subtle bg-accent-subtle text-fg-accent',
  Terbatas: 'bg-muted text-fg-tertiary',
}

type SlotBlockProps = {
  status: SlotStatus
  time?: string
  label?: string
  className?: string
}

/** Figma `Slot Block`: one cell of the schedule grid. Status is also written out for screen readers. */
export function SlotBlock({ status, time, label, className }: SlotBlockProps) {
  return (
    <div
      data-slot='slot-block'
      data-status={status}
      className={cn(
        'flex min-h-10 flex-col justify-center rounded-md px-2 py-1 type-label',
        SLOT_STYLE[status],
        className
      )}
    >
      {time && <span>{time}</span>}
      {label && (
        <span className='truncate font-normal opacity-80'>{label}</span>
      )}
      <span className='sr-only'>{status}</span>
    </div>
  )
}
