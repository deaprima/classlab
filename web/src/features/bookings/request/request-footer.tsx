import { CheckCircle2, Info, XCircle } from 'lucide-react'
import { cn } from '@/lib/utils'
import { type ConflictItem, ConflictList } from '@/components/conflict-list'
import { type Outcome } from './policy'

type RequestFooterProps = {
  outcome: Outcome
  /** What occupies the slots, from the live check or the server's 409. */
  conflicts: ConflictItem[]
  /** Message of a failed submit that has no list of clashes (rules). */
  problem?: string
  /** Batal and Kirim Pengajuan. */
  actions: React.ReactNode
}

const TONE: Record<Outcome['kind'], string> = {
  auto: 'text-fg',
  manual: 'text-fg',
  unknown: 'text-fg-secondary',
  conflict: 'text-fg-error',
  blocked: 'text-fg-error',
}

/**
 * The foot of the request popup (docs/DESIGN.md "Ajukan Ruang"): what the answer is likely to be,
 * and the actions. It stays in view while the form above scrolls.
 */
export function RequestFooter({
  outcome,
  conflicts,
  problem,
  actions,
}: RequestFooterProps) {
  const Icon =
    outcome.kind === 'conflict' || outcome.kind === 'blocked'
      ? XCircle
      : outcome.kind === 'unknown'
        ? Info
        : CheckCircle2

  return (
    <footer className='flex flex-col gap-3 border-t bg-card px-6 py-4 sm:flex-row sm:items-center sm:justify-between sm:gap-6'>
      <div
        role='status'
        aria-live='polite'
        className={cn('flex min-w-0 gap-3', TONE[outcome.kind])}
      >
        <Icon aria-hidden className='mt-0.5 size-5 shrink-0' />
        <div className='min-w-0'>
          <p className='type-card-title'>{outcome.title}</p>
          <p className='type-body-sm opacity-80'>{outcome.description}</p>
          {conflicts.length > 0 && (
            <ConflictList
              items={conflicts}
              withDate
              className='mt-2 max-h-24 overflow-y-auto'
            />
          )}
          {problem && (
            <p role='alert' className='mt-1 type-body-sm text-fg-error'>
              {problem}
            </p>
          )}
        </div>
      </div>
      <div className='flex shrink-0 gap-3 max-sm:*:flex-1'>{actions}</div>
    </footer>
  )
}
