import { describe, expect, it } from 'vitest'
import { type Checkout } from '@/lib/api-types'
import { filterByType, rangeFor, whenLabel } from './rules'

// 2026-10-22 12:00 WIB
const NOW = new Date('2026-10-22T05:00:00Z')

describe('rangeFor', () => {
  const ms = (key: Parameters<typeof rangeFor>[0]) => {
    const { from, to } = rangeFor(key, NOW)
    return [Date.parse(from), Date.parse(to)]
  }
  const wib = (date: string) => Date.parse(`${date}T00:00:00+07:00`)

  it('covers today, yesterday and the trailing weeks in WIB days', () => {
    expect(ms('today')).toEqual([wib('2026-10-22'), wib('2026-10-23')])
    expect(ms('yesterday')).toEqual([wib('2026-10-21'), wib('2026-10-22')])
    expect(ms('week')).toEqual([wib('2026-10-16'), wib('2026-10-23')])
    expect(ms('month')).toEqual([wib('2026-09-23'), wib('2026-10-23')])
  })
})

describe('filterByType', () => {
  const rows = [
    { booking_id: '1', room_type: 'KELAS' },
    { booking_id: '2', room_type: 'LAB' },
  ] as Checkout[]

  it('keeps every row without a type, else only that type', () => {
    expect(filterByType(rows, undefined)).toHaveLength(2)
    expect(filterByType(rows, 'LAB').map((c) => c.booking_id)).toEqual(['2'])
  })
})

describe('whenLabel', () => {
  it('says today and yesterday, else the short date', () => {
    expect(whenLabel('2026-10-22T03:02:00Z', NOW)).toBe('Hari ini · 10:02')
    expect(whenLabel('2026-10-21T09:10:00Z', NOW)).toBe('Kemarin · 16:10')
    expect(whenLabel('2026-10-16T04:58:00Z', NOW)).toMatch(/16 Okt · 11:58$/)
  })
})
