import { cn } from '@/lib/utils'

type SegmentOption<T extends string> = {
  value: T
  label: string
  /** Figma `Show Count`. */
  count?: number
}

type SegmentedControlProps<T extends string> = {
  value: T
  onValueChange: (value: T) => void
  options: readonly SegmentOption<T>[]
  'aria-label': string
  className?: string
}

/** Figma `Segmented Control`: 2 to 4 Tab Items; the active one is a black pill. Switches a view in place. */
export function SegmentedControl<T extends string>({
  value,
  onValueChange,
  options,
  className,
  'aria-label': ariaLabel,
}: SegmentedControlProps<T>) {
  return (
    <div
      role='radiogroup'
      aria-label={ariaLabel}
      data-slot='segmented-control'
      className={cn(
        'inline-flex max-w-full items-center gap-1 overflow-x-auto rounded-full border bg-surface p-1',
        className
      )}
    >
      {options.map((option) => {
        const active = option.value === value
        return (
          <button
            key={option.value}
            type='button'
            role='radio'
            aria-checked={active}
            onClick={() => onValueChange(option.value)}
            className={cn(
              'inline-flex h-8 min-w-10 items-center justify-center gap-1.5 rounded-full px-3 type-label whitespace-nowrap transition-colors',
              active
                ? 'bg-primary text-primary-foreground'
                : 'text-fg-secondary hover:text-fg'
            )}
          >
            {option.label}
            {option.count !== undefined && (
              <span
                className={cn(
                  'type-label-sm',
                  active ? 'opacity-70' : 'text-fg-tertiary'
                )}
              >
                {option.count}
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}
