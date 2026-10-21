import { CheckCircle2, Info, XCircle } from 'lucide-react'
import { type Room } from '@/lib/api-types'
import {
  formatDateShort,
  formatDuration,
  formatTimeRange,
  minutesOfDay,
} from '@/lib/datetime'
import { cn } from '@/lib/utils'
import { type ConflictItem, ConflictList } from '@/components/conflict-list'
import { type Outcome } from './policy'
import { type OccurrenceValues } from './request-schema'

type RequestSummaryProps = {
  room: Room | undefined
  occurrences: readonly OccurrenceValues[]
  participants: string
  outcome: Outcome
  /** What occupies the slots, from the live check or the server's 409. */
  conflicts: ConflictItem[]
  /** Message of a failed submit that has no list of clashes (quota, rules). */
  problem?: string
  /** Buttons; on mobile they become a bar fixed to the bottom, like the Figma mobile frames. */
  actions: React.ReactNode
  footnote?: string
}

const OUTCOME_STYLE: Record<Outcome['kind'], string> = {
  auto: 'border-line-strong bg-subtle text-fg',
  manual: 'border-line-strong bg-subtle text-fg',
  unknown: 'border-line bg-subtle text-fg-secondary',
  conflict: 'border-line-error bg-error-subtle text-fg-error',
  blocked: 'border-line-error bg-error-subtle text-fg-error',
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className='flex items-start justify-between gap-4'>
      <dt className='type-body text-fg-secondary'>{label}</dt>
      <dd className='text-end type-label text-fg'>{value}</dd>
    </div>
  )
}

function complete(o: OccurrenceValues): boolean {
  return (
    /^\d{4}-\d{2}-\d{2}$/.test(o.date) &&
    /^\d{2}:\d{2}$/.test(o.start_time) &&
    /^\d{2}:\d{2}$/.test(o.end_time) &&
    minutesOfDay(o.end_time) > minutesOfDay(o.start_time)
  )
}

/** "Ringkasan": what will be requested, and what the answer is likely to be (Figma, Ajukan Ruang). */
export function RequestSummary({
  room,
  occurrences,
  participants,
  outcome,
  conflicts,
  problem,
  actions,
  footnote,
}: RequestSummaryProps) {
  const ready = occurrences.filter(complete)
  const single = ready.length === 1 ? ready[0] : undefined
  const Icon =
    outcome.kind === 'conflict' || outcome.kind === 'blocked'
      ? XCircle
      : outcome.kind === 'unknown'
        ? Info
        : CheckCircle2
  const toIso = (date: string, time: string) => `${date}T${time}:00+07:00`

  return (
    <section
      aria-label='Ringkasan'
      className='flex flex-col gap-4 rounded-2xl border bg-card p-5'
    >
      <h2 className='type-section text-fg'>Ringkasan</h2>
      <dl className='flex flex-col gap-3'>
        <Row label='Ruang' value={room?.code ?? '—'} />
        {single ? (
          <>
            <Row
              label='Tanggal'
              value={formatDateShort(toIso(single.date, '00:00'))}
            />
            <Row
              label='Waktu'
              value={formatTimeRange(
                toIso(single.date, single.start_time),
                toIso(single.date, single.end_time)
              )}
            />
            <Row
              label='Durasi'
              value={formatDuration(
                minutesOfDay(single.end_time) - minutesOfDay(single.start_time)
              )}
            />
          </>
        ) : (
          <>
            <Row
              label='Tanggal'
              value={ready.length > 0 ? `${ready.length} tanggal` : '—'}
            />
            {ready.map((o, i) => (
              <Row
                key={`${o.date}-${o.start_time}-${i}`}
                label={formatDateShort(toIso(o.date, '00:00'))}
                value={formatTimeRange(
                  toIso(o.date, o.start_time),
                  toIso(o.date, o.end_time)
                )}
              />
            ))}
          </>
        )}
        {participants.trim() !== '' && (
          <Row label='Peserta' value={`${participants.trim()} orang`} />
        )}
      </dl>

      <div
        role='status'
        aria-live='polite'
        className={cn(
          'flex gap-3 rounded-xl border p-4',
          OUTCOME_STYLE[outcome.kind]
        )}
      >
        <Icon aria-hidden className='mt-0.5 size-5 shrink-0' />
        <div className='min-w-0'>
          <p className='type-card-title'>{outcome.title}</p>
          <p className='type-body-sm opacity-80'>{outcome.description}</p>
          {conflicts.length > 0 && (
            <ConflictList items={conflicts} withDate className='mt-2' />
          )}
        </div>
      </div>

      {problem && (
        <p role='alert' className='type-body-sm text-fg-error'>
          {problem}
        </p>
      )}

      <div className='flex flex-col gap-3 max-md:fixed max-md:inset-x-0 max-md:bottom-0 max-md:z-40 max-md:flex-row-reverse max-md:border-t max-md:bg-surface max-md:p-4 max-md:pb-[max(1rem,env(safe-area-inset-bottom))] max-md:*:flex-1'>
        {actions}
      </div>
      {footnote && <p className='type-body-sm text-fg-tertiary'>{footnote}</p>}
    </section>
  )
}
