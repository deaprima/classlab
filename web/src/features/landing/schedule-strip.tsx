import { formatTime, minutesOfDay } from '@/lib/datetime'
import { cn } from '@/lib/utils'
import { useNow } from '@/hooks/use-now'
import { useInView } from './use-in-view'

const OPEN = 7 * 60
const CLOSE = 21 * 60
const HOURS = Array.from({ length: 8 }, (_, i) => 7 + i * 2)

type Block = {
  from: string
  to: string
  kind: 'used' | 'requested'
  label: string
}

// Sample day, only to show how the schedule reads: dark = in use, orange = requested, empty = free.
const LANES: { room: string; blocks: Block[] }[] = [
  {
    room: 'R-301',
    blocks: [
      { from: '08:00', to: '10:00', kind: 'used', label: 'Kalkulus' },
      { from: '10:00', to: '12:00', kind: 'requested', label: 'Rapat' },
      { from: '13:00', to: '15:00', kind: 'used', label: 'Fisika' },
    ],
  },
  {
    room: 'R-302',
    blocks: [
      { from: '07:00', to: '09:00', kind: 'used', label: 'Basis Data' },
      { from: '13:00', to: '16:00', kind: 'requested', label: 'Sidang' },
    ],
  },
  {
    room: 'LAB-2',
    blocks: [
      { from: '09:00', to: '12:00', kind: 'used', label: 'Praktikum' },
      { from: '15:00', to: '18:00', kind: 'used', label: 'Jaringan' },
    ],
  },
]

const pct = (time: string) =>
  ((minutesOfDay(time) - OPEN) / (CLOSE - OPEN)) * 100

/** "Pita jadwal": three rooms across the day. The orange line is the real time, clamped to the hours shown. */
export function ScheduleStrip({ className }: { className?: string }) {
  const [ref, seen] = useInView<HTMLDivElement>(0.3)
  const now = useNow(30_000)
  const clock = formatTime(now)
  const nowPct = Math.min(100, Math.max(0, pct(clock)))

  return (
    <div
      ref={ref}
      className={cn('rounded-3xl border bg-card p-5 sm:p-6', className)}
    >
      <div className='grid grid-cols-[3.5rem_1fr] gap-x-3 sm:grid-cols-[4.5rem_1fr]'>
        {/* Hour labels */}
        <span />
        <div className='relative mb-2 h-5'>
          {HOURS.map((h) => (
            <span
              key={h}
              className='absolute -translate-x-1/2 type-label-sm text-fg-tertiary tabular-nums'
              style={{ left: `${((h * 60 - OPEN) / (CLOSE - OPEN)) * 100}%` }}
            >
              {String(h).padStart(2, '0')}
            </span>
          ))}
        </div>

        {/* Room names */}
        <div className='flex flex-col gap-2'>
          {LANES.map((lane) => (
            <span
              key={lane.room}
              className='flex h-11 items-center type-label text-fg'
            >
              {lane.room}
            </span>
          ))}
        </div>

        {/* Lanes, with the "now" line laid over all of them */}
        <div className='relative flex flex-col gap-2'>
          {LANES.map((lane, li) => (
            <div key={lane.room} className='relative h-11 rounded-xl bg-subtle'>
              {lane.blocks.map((b, bi) => (
                <div
                  key={b.from}
                  title={`${lane.room} ${b.from}–${b.to} · ${b.label}`}
                  className={cn(
                    'absolute inset-y-1 origin-left overflow-hidden rounded-lg px-2 transition-transform duration-700 ease-out motion-reduce:transition-none',
                    b.kind === 'used'
                      ? 'bg-inverse text-fg-inverse'
                      : 'border border-line-accent bg-accent-subtle text-fg-accent',
                    seen ? 'scale-x-100' : 'scale-x-0'
                  )}
                  style={{
                    left: `${pct(b.from)}%`,
                    width: `${pct(b.to) - pct(b.from)}%`,
                    transitionDelay: seen
                      ? `${300 + li * 150 + bi * 120}ms`
                      : '0ms',
                  }}
                >
                  <span className='block truncate pt-2.5 type-label-sm'>
                    {b.label}
                  </span>
                </div>
              ))}
            </div>
          ))}

          <div
            aria-hidden
            className='pointer-events-none absolute inset-y-[-0.25rem] w-px bg-brand transition-[left] duration-[1400ms] ease-out motion-reduce:transition-none'
            style={{ left: seen ? `${nowPct}%` : '0%' }}
          >
            {/* Near either end the label hangs inward, so it never pokes out of the card. */}
            <span
              className={cn(
                'absolute bottom-0 inline-flex translate-y-full items-center gap-1.5 rounded-full bg-brand px-2.5 py-0.5 type-label-sm whitespace-nowrap text-on-brand',
                nowPct > 80
                  ? 'right-0'
                  : nowPct < 20
                    ? 'left-0'
                    : 'left-1/2 -translate-x-1/2'
              )}
            >
              <span className='relative flex size-1.5'>
                <span className='absolute inset-0 rounded-full bg-on-brand motion-safe:animate-landing-ping' />
                <span className='relative size-1.5 rounded-full bg-on-brand' />
              </span>
              Sekarang {clock}
            </span>
          </div>
        </div>
      </div>

      <ul className='mt-10 flex flex-wrap gap-x-5 gap-y-1 type-body-sm text-fg-secondary'>
        <li className='flex items-center gap-2'>
          <span className='size-3 rounded-sm bg-inverse' /> Terisi
        </li>
        <li className='flex items-center gap-2'>
          <span className='size-3 rounded-sm border border-line-accent bg-accent-subtle' />{' '}
          Diajukan
        </li>
        <li className='flex items-center gap-2'>
          <span className='size-3 rounded-sm bg-subtle ring-1 ring-line' />{' '}
          Kosong
        </li>
      </ul>
    </div>
  )
}
