import { cn } from '@/lib/utils'

// A decorative QR-like pattern: three finder squares and a fixed scatter of modules. It is not a
// real code (the real QR of a room is made from its token), just the picture of one.
const SIZE = 11
const FINDER = [
  [0, 0],
  [SIZE - 3, 0],
  [0, SIZE - 3],
] as const
const SCATTER = new Set(
  '3,0 5,0 4,1 6,1 3,2 5,3 7,3 9,3 4,4 6,4 8,5 3,5 5,6 7,6 9,6 3,7 6,7 8,8 5,8 7,9 9,9 4,9 3,10 6,10 8,10'
    .split(' ')
    .map((p) => p)
)

function isFinder(x: number, y: number) {
  return FINDER.some(
    ([fx, fy]) => x >= fx && x < fx + 3 && y >= fy && y < fy + 3
  )
}

function PixelQr({ className }: { className?: string }) {
  const cells: React.ReactNode[] = []
  for (let y = 0; y < SIZE; y++) {
    for (let x = 0; x < SIZE; x++) {
      const finder = isFinder(x, y)
      // Finder squares are rings: filled border, hollow middle.
      const fx = FINDER.find(
        ([a, b]) => x >= a && x < a + 3 && y >= b && y < b + 3
      )
      const ring = fx ? !(x === fx[0] + 1 && y === fx[1] + 1) : false
      const on = finder ? ring : SCATTER.has(`${x},${y}`)
      if (on) {
        cells.push(<rect key={`${x}-${y}`} x={x} y={y} width='1' height='1' />)
      }
    }
  }
  return (
    <svg
      viewBox={`0 0 ${SIZE} ${SIZE}`}
      role='img'
      aria-label='Contoh kode QR ruang'
      className={className}
      fill='currentColor'
      shapeRendering='crispEdges'
    >
      {cells}
    </svg>
  )
}

const SLOTS = [
  { time: '08:00 – 10:00', label: 'Basis Data', state: 'Terisi' },
  { time: '10:00 – 12:00', label: 'Rapat HIMA', state: 'Diajukan' },
  { time: '13:00 – 15:00', label: 'Masih kosong', state: 'Tersedia' },
] as const

/** The "plat nama pintu" of the hero: a door sign that sways a little, like it hangs on a cord. */
export function DoorPlate({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        'relative mx-auto w-full max-w-[min(420px,calc(100vw-3rem))] motion-safe:animate-landing-plate-in',
        className
      )}
      style={{ animationDelay: '250ms' }}
    >
      <div className='motion-safe:animate-landing-sway'>
        {/* The cord and the hole it goes through. */}
        <div aria-hidden className='mx-auto h-14 w-px bg-line-strong' />
        <div className='-rotate-3 rounded-[32px] bg-inverse p-7 text-fg-inverse shadow-xl'>
          <div className='mb-5 flex items-start justify-between gap-4'>
            <div>
              <span
                aria-hidden
                className='mb-4 block size-3 rounded-full border-2 border-fg-inverse/40'
              />
              <p className='type-overline text-fg-inverse/60'>Ruang</p>
              <p className='text-6xl leading-none font-semibold tracking-tight'>
                R-302
              </p>
              <p className='mt-1 type-body text-fg-inverse/70'>
                Kelas · 40 orang
              </p>
            </div>
            <div className='rounded-2xl bg-fg-inverse p-2 text-inverse'>
              <PixelQr className='size-20' />
            </div>
          </div>
          <ul className='flex flex-col gap-2'>
            {SLOTS.map((s) => (
              <li
                key={s.time}
                className='flex items-center justify-between gap-3 rounded-xl border border-fg-inverse/15 px-3 py-2'
              >
                <span className='min-w-0'>
                  <span className='block type-label tabular-nums'>
                    {s.time}
                  </span>
                  <span className='block truncate type-body-sm text-fg-inverse/60'>
                    {s.label}
                  </span>
                </span>
                <span
                  className={cn(
                    'rounded-full px-2.5 py-0.5 type-label-sm',
                    s.state === 'Diajukan'
                      ? 'bg-brand text-on-brand'
                      : s.state === 'Tersedia'
                        ? 'border border-fg-inverse/40'
                        : 'bg-fg-inverse text-inverse'
                  )}
                >
                  {s.state}
                </span>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  )
}
