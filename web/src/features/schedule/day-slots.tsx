import { dateParts } from '@/lib/datetime'
import { cn } from '@/lib/utils'
import { inRange, type DaySlot } from './slots'

type DayChipsProps = {
  days: readonly string[]
  value: string
  onChange: (day: string) => void
  /** Days that cannot be picked (before today). */
  disabledBefore?: string
}

/** A row of day chips: "Rab 21", "Kam 22", ... The selected one is the black pill. */
export function DayChips({
  days,
  value,
  onChange,
  disabledBefore,
}: DayChipsProps) {
  return (
    <div
      role='radiogroup'
      aria-label='Tanggal'
      className='flex gap-2 overflow-x-auto pb-1'
    >
      {days.map((day) => {
        const { weekday, day: n } = dateParts(day)
        const active = day === value
        const disabled = !!disabledBefore && day < disabledBefore
        return (
          <button
            key={day}
            type='button'
            role='radio'
            aria-checked={active}
            disabled={disabled}
            onClick={() => onChange(day)}
            className={cn(
              'inline-flex h-9 shrink-0 items-center rounded-full border px-4 type-label transition-colors',
              active
                ? 'border-transparent bg-primary text-primary-foreground'
                : 'bg-surface text-fg hover:border-line-hover',
              disabled && 'pointer-events-none text-fg-tertiary'
            )}
          >
            {weekday} {n}
          </button>
        )
      })}
    </div>
  )
}

type SlotChipsProps = {
  slots: readonly DaySlot[]
  range: { start: string; end: string } | null
  /** Called with the clicked free slot; omit for a display-only list. */
  onPick?: (time: string) => void
}

/** The slots of a day as chips (Figma "Detail Ruang"): free, used, past, and the picked range. */
export function SlotChips({ slots, range, onPick }: SlotChipsProps) {
  return (
    <ul className='flex flex-wrap gap-2'>
      {slots.map((slot) => {
        const picked = inRange(slot.time, range)
        const free = slot.state === 'free'
        const label =
          slot.state === 'used'
            ? `${slot.time}, terpakai${slot.item ? `: ${slot.item.title}` : ''}`
            : slot.state === 'past'
              ? `${slot.time}, sudah lewat`
              : `${slot.time}, tersedia`
        return (
          <li key={slot.time}>
            <button
              type='button'
              aria-label={label}
              aria-pressed={onPick && free ? picked : undefined}
              disabled={!onPick || !free}
              onClick={() => onPick?.(slot.time)}
              title={slot.item?.title}
              className={cn(
                'h-10 min-w-16 rounded-lg border px-3 type-label transition-colors',
                picked
                  ? 'border-line-accent bg-accent-subtle text-fg-accent'
                  : slot.state === 'used'
                    ? 'border-transparent bg-inverse text-fg-inverse'
                    : slot.state === 'past'
                      ? 'border-transparent bg-muted text-fg-tertiary'
                      : 'bg-surface text-fg-secondary',
                free && onPick && !picked && 'hover:border-line-hover'
              )}
            >
              {slot.time}
            </button>
          </li>
        )
      })}
    </ul>
  )
}
