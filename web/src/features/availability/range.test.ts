import { slotTimes } from '@/config/booking-rules'
import { describe, expect, it } from 'vitest'
import { defaultRange, isValidRange } from './range'

describe('defaultRange', () => {
  it('starts at the next half hour, two hours long', () => {
    expect(defaultRange(new Date('2026-10-20T09:10:00+07:00'))).toEqual({
      date: '2026-10-20',
      start: '09:30',
      end: '11:30',
    })
  })

  it('starts at opening time early in the morning', () => {
    expect(defaultRange(new Date('2026-10-20T05:00:00+07:00'))).toEqual({
      date: '2026-10-20',
      start: '07:00',
      end: '09:00',
    })
  })

  it('stops at closing time', () => {
    expect(defaultRange(new Date('2026-10-20T19:45:00+07:00'))).toEqual({
      date: '2026-10-20',
      start: '20:00',
      end: '21:00',
    })
  })

  it('moves to tomorrow morning after hours', () => {
    expect(defaultRange(new Date('2026-10-20T20:40:00+07:00'))).toEqual({
      date: '2026-10-21',
      start: '07:00',
      end: '09:00',
    })
  })
})

describe('time helpers', () => {
  it('lists every 30-minute boundary in operating hours', () => {
    const times = slotTimes()
    expect(times[0]).toBe('07:00')
    expect(times[times.length - 1]).toBe('21:00')
    expect(times).toHaveLength(29)
  })

  it('checks that a range moves forward', () => {
    expect(isValidRange('08:00', '10:00')).toBe(true)
    expect(isValidRange('10:00', '10:00')).toBe(false)
  })
})
