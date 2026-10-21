import { AlertCircle, CalendarPlus, X } from 'lucide-react'
import { type Room } from '@/lib/api-types'
import { dateParts, formatTimeRange } from '@/lib/datetime'
import { Button } from '@/components/ui/button'
import { useSlotCheck } from '@/features/bookings/request/slot-checks'
import { rangesToOccurrences, type Ranges } from './selection'

type SelectionBarProps = {
  room: Room
  ranges: Ranges
  onRemove: (date: string) => void
  onClear: () => void
  onRequest: () => void
}

/**
 * The bar fixed to the bottom of the screen: what is picked on the schedule and the one button to
 * request it. It stays on screen so the button is always in reach (it sits beside the sidebar).
 */
export function SelectionBar({
  room,
  ranges,
  onRemove,
  onClear,
  onRequest,
}: SelectionBarProps) {
  const occurrences = rangesToOccurrences(ranges)
  const check = useSlotCheck(room.id, occurrences)
  const empty = occurrences.length === 0
  const taken = check.free === false

  return (
    <div
      role='region'
      aria-label='Pengajuan ruang'
      className='fixed inset-x-4 bottom-4 z-40 md:right-6 md:left-[calc(var(--sidebar-width)+1.5rem)]'
    >
      <div className='mx-auto flex max-w-full flex-wrap items-center gap-3 rounded-2xl border border-line-strong bg-card p-3 shadow-sm md:px-4'>
        <div className='flex min-w-0 grow flex-wrap items-center gap-2'>
          {empty ? (
            <p className='type-body text-fg-secondary'>
              Pilih jam kosong di jadwal {room.code} untuk mengajukan.
            </p>
          ) : (
            <>
              <p className='shrink-0 type-card-title text-fg'>{room.code}</p>
              <ul className='flex flex-wrap gap-2'>
                {occurrences.map((o) => {
                  const { weekday, day } = dateParts(o.date)
                  const when = formatTimeRange(
                    `${o.date}T${o.start_time}:00+07:00`,
                    `${o.date}T${o.end_time}:00+07:00`
                  )
                  return (
                    <li
                      key={o.date}
                      className='flex items-center gap-1 rounded-full border border-line-accent-subtle bg-accent-subtle py-1 ps-3 pe-1 type-label text-fg-accent'
                    >
                      <span>
                        {weekday} {day} · {when}
                      </span>
                      <button
                        type='button'
                        aria-label={`Hapus pilihan ${weekday} ${day}`}
                        onClick={() => onRemove(o.date)}
                        className='flex size-6 items-center justify-center rounded-full hover:bg-surface'
                      >
                        <X aria-hidden className='size-3.5' />
                      </button>
                    </li>
                  )
                })}
              </ul>
              {taken && (
                <p
                  role='status'
                  className='flex items-center gap-1.5 type-body-sm text-fg-error'
                >
                  <AlertCircle aria-hidden className='size-4 shrink-0' />
                  Ada slot yang sedang diajukan atau dipakai orang lain.
                </p>
              )}
            </>
          )}
        </div>
        <div className='flex shrink-0 items-center gap-2'>
          {!empty && (
            <Button type='button' variant='ghost' size='sm' onClick={onClear}>
              Hapus semua
            </Button>
          )}
          <Button
            type='button'
            variant='accent'
            disabled={empty || taken}
            onClick={onRequest}
          >
            <CalendarPlus />
            Ajukan Ruang
          </Button>
        </div>
      </div>
    </div>
  )
}
