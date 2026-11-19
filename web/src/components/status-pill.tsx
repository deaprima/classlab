import { cn } from '@/lib/utils'

/** The ten statuses of the Figma `Status Pill` component (DESIGN.md section 4). */
export type PillStatus =
  | 'Tersedia'
  | 'Digunakan'
  | 'Diajukan'
  | 'Terbatas'
  | 'Menunggu'
  | 'Disetujui'
  | 'Ditolak'
  | 'Diterima'
  | 'Diproses'
  | 'Selesai'

// Approved is green, refused or missed red, waiting yellow (the user's call, overriding the
// no-green rule of DESIGN.md section 9 for these three); the rest stays ink, outline and orange.
const PILL_STYLE: Record<PillStatus, { pill: string; dot: string }> = {
  Tersedia: { pill: 'border border-line bg-surface text-fg', dot: 'bg-fg' },
  Digunakan: { pill: 'bg-inverse text-fg-inverse', dot: 'bg-fg-inverse' },
  Disetujui: {
    pill: 'bg-success-subtle text-fg-success',
    dot: 'bg-fg-success',
  },
  Selesai: { pill: 'bg-inverse text-fg-inverse', dot: 'bg-fg-inverse' },
  Diajukan: { pill: 'bg-accent-subtle text-fg-accent', dot: 'bg-indicator' },
  Menunggu: {
    pill: 'bg-warning-subtle text-fg-warning',
    dot: 'bg-fg-warning',
  },
  Diproses: { pill: 'bg-accent-subtle text-fg-accent', dot: 'bg-indicator' },
  Terbatas: { pill: 'bg-muted text-fg-secondary', dot: 'bg-fg-secondary' },
  Ditolak: { pill: 'bg-error-subtle text-fg-error', dot: 'bg-fg-error' },
  Diterima: {
    pill: 'border border-line-strong bg-surface text-fg',
    dot: 'bg-fg',
  },
}

type StatusPillProps = {
  status: PillStatus
  /** Overrides the visible text, e.g. "Tanpa check-out" shown with the `Ditolak` style. */
  label?: string
  className?: string
}

export function StatusPill({ status, label, className }: StatusPillProps) {
  const style = PILL_STYLE[status]
  return (
    <span
      data-slot='status-pill'
      data-status={status}
      className={cn(
        'inline-flex h-7 w-fit shrink-0 items-center gap-1.5 rounded-full px-3 type-label whitespace-nowrap',
        style.pill,
        className
      )}
    >
      <span aria-hidden className={cn('size-1.5 rounded-full', style.dot)} />
      {label ?? status}
    </span>
  )
}
