import { describe, expect, it } from 'vitest'
import { rangesToOccurrences, withRange } from './selection'

describe('selection', () => {
  it('turns ranges into occurrences, earliest date first', () => {
    expect(
      rangesToOccurrences({
        '2026-10-23': { start: '09:00', end: '11:00' },
        '2026-10-21': { start: '08:00', end: '10:00' },
      })
    ).toEqual([
      { date: '2026-10-21', start_time: '08:00', end_time: '10:00' },
      { date: '2026-10-23', start_time: '09:00', end_time: '11:00' },
    ])
  })

  it('sets, replaces and removes the range of a date without touching the others', () => {
    const a = withRange({}, '2026-10-21', { start: '08:00', end: '09:00' })
    const b = withRange(a, '2026-10-22', { start: '10:00', end: '11:00' })
    const c = withRange(b, '2026-10-21', { start: '08:00', end: '10:00' })
    expect(c['2026-10-21']).toEqual({ start: '08:00', end: '10:00' })
    expect(Object.keys(withRange(c, '2026-10-21', null))).toEqual([
      '2026-10-22',
    ])
    expect(a).toEqual({ '2026-10-21': { start: '08:00', end: '09:00' } })
  })
})
