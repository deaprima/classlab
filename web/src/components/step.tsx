import { Check } from 'lucide-react'
import { cn } from '@/lib/utils'

type StepState = 'done' | 'current' | 'upcoming'

type StepProps = {
  state: StepState
  title: string
  description?: React.ReactNode
  /** Right-aligned note such as a time. */
  meta?: React.ReactNode
  /** Draws the connector to the next step. */
  hasNext?: boolean
}

const MARKER: Record<StepState, string> = {
  done: 'bg-inverse text-fg-inverse',
  current: 'border-2 border-line-accent bg-accent-subtle text-fg-accent',
  upcoming: 'border border-line-strong bg-surface text-fg-tertiary',
}

/** Figma `Stepper Step` / `Timeline Step`: marker, connector, title and note. Render inside an <ol>. */
export function Step({ state, title, description, meta, hasNext }: StepProps) {
  return (
    <li
      data-slot='step'
      data-state={state}
      aria-current={state === 'current' ? 'step' : undefined}
      className='flex gap-3'
    >
      <div className='flex flex-col items-center'>
        <span
          className={cn(
            'flex size-6 shrink-0 items-center justify-center rounded-full',
            MARKER[state]
          )}
        >
          {state === 'done' ? (
            <Check aria-hidden className='size-3.5' />
          ) : (
            <span
              aria-hidden
              className={cn(
                'size-2 rounded-full',
                state === 'current' ? 'bg-indicator' : 'bg-line-strong'
              )}
            />
          )}
        </span>
        {hasNext && (
          <span
            aria-hidden
            className={cn(
              'my-1 w-px grow',
              state === 'done' ? 'bg-inverse' : 'bg-line-strong'
            )}
          />
        )}
      </div>
      <div className={cn('flex min-w-0 grow gap-3 pb-4', !hasNext && 'pb-0')}>
        <div className='min-w-0 grow'>
          <p
            className={cn(
              'type-card-title',
              state === 'upcoming' ? 'text-fg-tertiary' : 'text-fg'
            )}
          >
            {title}
          </p>
          {description && (
            <p className='type-body-sm text-fg-secondary'>{description}</p>
          )}
        </div>
        {meta && (
          <p className='shrink-0 type-label text-fg-secondary'>{meta}</p>
        )}
      </div>
    </li>
  )
}
