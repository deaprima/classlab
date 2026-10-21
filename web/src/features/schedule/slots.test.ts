import { describe, expect, it } from 'vitest'
import { type TimelineItem } from '@/lib/api-types'
import { buildDaySlots, inRange, pickSlot } from './slots'

const hours = { open: '07:00', close: '12:00' }
const date = '2026-10-21'
const at = (hhmm: string) => new Date(`${date}T${hhmm}:00+07:00`)

function item(start: string, end: string): TimelineItem {
  return {
    id: `${start}-${end}`,
    kind: 'SCHEDULE',
    title: 'Kalkulus',
    start_at: `${date}T${start}:00+07:00`,
    end_at: `${date}T${end}:00+07:00`,
  }
}

describe('buildDaySlots', () => {
  it('makes one slot per 30 minutes inside the hours', () => {
    const slots = buildDaySlots({ date, hours, items: [], now: at('06:00') })
    expect(slots).toHaveLength(10)
    expect(slots[0].time).toBe('07:00')
    expect(slots[9].time).toBe('11:30')
    expect(slots.every((s) => s.state === 'free')).toBe(true)
  })

  it('marks slots covered by a lecture or booking as used, with the item', () => {
    const k = item('08:00', '09:30')
    const slots = buildDaySlots({ date, hours, items: [k], now: at('06:00') })
    const used = slots.filter((s) => s.state === 'used').map((s) => s.time)
    expect(used).toEqual(['08:00', '08:30', '09:00'])
    expect(slots.find((s) => s.time === '08:30')?.item).toBe(k)
  })

  it('does not occupy the slot where an item ends (half-open interval)', () => {
    const slots = buildDaySlots({
      date,
      hours,
      items: [item('08:00', '10:00')],
      now: at('06:00'),
    })
    expect(slots.find((s) => s.time === '10:00')?.state).toBe('free')
  })

  it('marks slots that already started as past, but used wins over past', () => {
    const slots = buildDaySlots({
      date,
      hours,
      items: [item('08:00', '09:00')],
      now: at('09:00'),
    })
    const state = (t: string) => slots.find((s) => s.time === t)?.state
    expect(state('07:30')).toBe('past')
    expect(state('08:00')).toBe('used')
    expect(state('09:00')).toBe('free')
  })

  it('clips an item that spans past the closing hour', () => {
    const slots = buildDaySlots({
      date,
      hours,
      items: [item('11:00', '15:00')],
      now: at('06:00'),
    })
    expect(slots.filter((s) => s.state === 'used').map((s) => s.time)).toEqual([
      '11:00',
      '11:30',
    ])
  })
})

describe('pickSlot', () => {
  const slots = buildDaySlots({
    date,
    hours,
    items: [item('09:00', '10:00')],
    now: at('06:00'),
  })

  it('a first click picks one slot', () => {
    expect(pickSlot(slots, null, '07:30')).toEqual({
      start: '07:30',
      end: '08:00',
    })
  })

  it('a second click on a later free slot stretches the range', () => {
    const first = pickSlot(slots, null, '07:30')
    expect(pickSlot(slots, first, '08:30')).toEqual({
      start: '07:30',
      end: '09:00',
    })
  })

  it('does not stretch across a used slot but starts over there', () => {
    const first = pickSlot(slots, null, '08:00')
    expect(pickSlot(slots, first, '10:30')).toEqual({
      start: '10:30',
      end: '11:00',
    })
  })

  it('starts over when clicking before the start or inside the range', () => {
    const range = { start: '08:00', end: '08:30' }
    expect(pickSlot(slots, range, '07:00')).toEqual({
      start: '07:00',
      end: '07:30',
    })
    expect(pickSlot(slots, range, '08:00')).toEqual({
      start: '08:00',
      end: '08:30',
    })
  })

  it('ignores a click on a used slot', () => {
    const range = { start: '07:00', end: '07:30' }
    expect(pickSlot(slots, range, '09:00')).toBe(range)
  })
})

describe('inRange', () => {
  it('is true from the start up to, not including, the end', () => {
    const range = { start: '08:00', end: '09:00' }
    expect(inRange('08:00', range)).toBe(true)
    expect(inRange('08:30', range)).toBe(true)
    expect(inRange('09:00', range)).toBe(false)
    expect(inRange('08:00', null)).toBe(false)
  })
})
