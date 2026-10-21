import { type LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

type EmptyStateProps = {
  icon?: LucideIcon
  title: string
  description?: React.ReactNode
  /** Usually a single Button. */
  action?: React.ReactNode
  className?: string
}

/** Figma `Empty State`: centered, at most 360 px wide. */
export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      data-slot='empty-state'
      className={cn(
        'mx-auto flex w-full max-w-90 flex-col items-center gap-3 px-4 py-10 text-center',
        className
      )}
    >
      {Icon && (
        <span className='flex size-12 items-center justify-center rounded-full bg-muted text-fg-secondary'>
          <Icon aria-hidden className='size-5' />
        </span>
      )}
      <h3 className='type-section text-fg'>{title}</h3>
      {description && (
        <p className='type-body text-fg-secondary'>{description}</p>
      )}
      {action && <div className='pt-2'>{action}</div>}
    </div>
  )
}
