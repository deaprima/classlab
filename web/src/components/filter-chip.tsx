import { cn } from '@/lib/utils'

/** Figma `Chip` (filter): a pill that is black while selected. */
export function FilterChip({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      type='button'
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        'inline-flex h-8 items-center rounded-full border px-3 type-label whitespace-nowrap transition-colors',
        active
          ? 'border-transparent bg-primary text-primary-foreground'
          : 'bg-surface text-fg hover:border-line-hover'
      )}
    >
      {children}
    </button>
  )
}
