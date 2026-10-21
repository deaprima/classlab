import { dateParts } from '@/lib/datetime'
import { cn } from '@/lib/utils'

type DateBadgeProps = {
  /** WIB calendar date, YYYY-MM-DD. */
  date: string
  className?: string
}

/** Figma `Date Badge`: weekday, day number and month, in WIB. */
export function DateBadge({ date, className }: DateBadgeProps) {
  const { day, weekday, month } = dateParts(date)
  return (
    <div
      data-slot='date-badge'
      className={cn(
        'flex size-14 shrink-0 flex-col items-center justify-center rounded-xl bg-muted',
        className
      )}
    >
      <span className='type-overline text-fg-secondary'>{weekday}</span>
      <span className='type-section text-fg'>{day}</span>
      <span className='type-label-sm text-fg-secondary'>{month}</span>
    </div>
  )
}
