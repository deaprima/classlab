import { cn } from '@/lib/utils'

type StatCardProps = {
  title: string
  value: React.ReactNode
  /** Short change note such as "+2 hari ini". */
  trend?: React.ReactNode
  caption?: React.ReactNode
  /** Orange dot: something here needs attention. */
  showDot?: boolean
  tone?: 'default' | 'subtle'
  className?: string
}

/** Figma `Stat Card`: a big number with a title, optional trend and caption. */
export function StatCard({
  title,
  value,
  trend,
  caption,
  showDot,
  tone = 'default',
  className,
}: StatCardProps) {
  return (
    <div
      data-slot='stat-card'
      className={cn(
        'flex min-w-0 flex-col gap-2 rounded-2xl border p-5',
        tone === 'default' ? 'bg-card' : 'bg-subtle',
        className
      )}
    >
      <div className='flex items-center justify-between gap-2'>
        <p className='type-card-title text-fg-secondary'>{title}</p>
        {showDot && (
          <span
            role='img'
            aria-label='Perlu perhatian'
            className='size-2 rounded-full bg-indicator'
          />
        )}
      </div>
      <p className='type-stat-l text-fg'>{value}</p>
      {(trend || caption) && (
        <div className='flex flex-wrap items-center gap-x-2 type-body-sm text-fg-secondary'>
          {trend && <span className='font-medium text-fg'>{trend}</span>}
          {caption && <span>{caption}</span>}
        </div>
      )}
    </div>
  )
}
