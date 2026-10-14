import { describe, expect, it } from 'vitest'
import { type TimelineItem } from '@/lib/api-types'
import { type GridSpec, placeItems, rowCount } from './timeline-layout'

const spec: GridSpec = {
  days: ['2026-10-19', '2026-10-20'],
  openTime: '07:00',
  closeTime: '21:00',
  slotMinutes: 30,
}

function item(start: string, end: string, id = start): TimelineItem {
  return {
    id,
    kind: 'SCHEDULE',
    title: 'Basis Data',
    start_at: start,
    end_at: end,
  }
}

describe('timeline layout', () => {
  it('has one row per slot in operating hours', () => {
    expect(rowCount(spec)).toBe(28)
  })

  it('places an item on its WIB day and slot rows', () => {
    const [placed] = placeItems(
      [item('2026-10-20T08:00:00+07:00', '2026-10-20T10:00:00+07:00')],
      spec
    )
    expect(placed).toMatchObject({ day: 1, rowStart: 2, rowEnd: 6 })
  })

  it('uses WIB even when the API sends UTC', () => {
    const [placed] = placeItems(
      [item('2026-10-19T00:00:00Z', '2026-10-19T01:30:00Z')],
      spec
    )
    expect(placed).toMatchObject({ day: 0, rowStart: 0, rowEnd: 3 })
  })

  it('clips items to operating hours and drops what falls outside', () => {
    const placed = placeItems(
      [
        item('2026-10-19T06:00:00+07:00', '2026-10-19T08:00:00+07:00', 'a'),
        item('2026-10-19T21:00:00+07:00', '2026-10-19T22:00:00+07:00', 'b'),
        item('2026-10-25T08:00:00+07:00', '2026-10-25T09:00:00+07:00', 'c'),
      ],
      spec
    )
    expect(placed).toHaveLength(1)
    expect(placed[0]).toMatchObject({ rowStart: 0, rowEnd: 2 })
  })
})
