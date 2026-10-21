import { cn } from '@/lib/utils'
import { StatusPill, type PillStatus } from './status-pill'

type ReportCardProps = {
  room: string
  damage: string
  status: PillStatus
  meta?: React.ReactNode
  className?: string
}

/** Figma `Report Card`: one damage report in the kanban or a list. */
export function ReportCard({
  room,
  damage,
  status,
  meta,
  className,
}: ReportCardProps) {
  return (
    <article
      data-slot='report-card'
      className={cn(
        'flex flex-col gap-2 rounded-xl border bg-card p-4',
        className
      )}
    >
      <div className='flex items-start justify-between gap-2'>
        <h3 className='type-card-title text-fg'>{room}</h3>
        <StatusPill status={status} />
      </div>
      <p className='type-body text-fg-secondary'>{damage}</p>
      {meta && <p className='type-body-sm text-fg-tertiary'>{meta}</p>}
    </article>
  )
}
