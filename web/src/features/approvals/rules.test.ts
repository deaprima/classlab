import { describe, expect, it } from 'vitest'
import { type Booking } from '@/lib/api-types'
import { makeBooking } from '@/features/my-bookings/fixtures'
import { expiryLabel, groupQueue, matchesQueue } from './rules'

const NOW = new Date('2026-10-22T10:00:00+07:00')
const at = (hhmm: string) => `2026-10-22T${hhmm}:00+07:00`

describe('groupQueue', () => {
  const group = (id: string, groupId: string | null): Booking =>
    makeBooking({ id, group_id: groupId, occurrence_count: 2 })

  it('keeps single bookings as they are, in order', () => {
    const rows = [group('a', null), group('b', null)]
    expect(groupQueue(rows).map((i) => [i.lead.id, i.count])).toEqual([
      ['a', 1],
      ['b', 1],
    ])
  })

  it('folds the dates of one request into its first row', () => {
    const rows = [group('a', 'g1'), group('b', null), group('c', 'g1')]
    expect(groupQueue(rows).map((i) => [i.lead.id, i.count])).toEqual([
      ['a', 2],
      ['b', 1],
    ])
  })

  it('keeps different groups apart', () => {
    const rows = [group('a', 'g1'), group('b', 'g2'), group('c', 'g1')]
    expect(groupQueue(rows).map((i) => [i.lead.id, i.count])).toEqual([
      ['a', 2],
      ['b', 1],
    ])
  })
})

describe('expiryLabel', () => {
  it('writes hours and minutes', () => {
    expect(expiryLabel(at('15:20'), NOW)).toBe('5 j 20 m')
  })

  it('drops zero parts', () => {
    expect(expiryLabel(at('15:00'), NOW)).toBe('5 j')
    expect(expiryLabel(at('10:20'), NOW)).toBe('20 m')
  })

  it('is null at and after the deadline', () => {
    expect(expiryLabel(at('10:00'), NOW)).toBeNull()
    expect(expiryLabel(at('09:00'), NOW)).toBeNull()
  })
})

describe('matchesQueue', () => {
  const b = makeBooking({ purpose: 'Seminar Nasional HMIF' })

  it('matches everything without a query', () => {
    expect(matchesQueue(b, undefined)).toBe(true)
    expect(matchesQueue(b, '  ')).toBe(true)
  })

  it('looks in room, requester name and number, and activity, ignoring case', () => {
    expect(matchesQueue(b, b.room.code.toLowerCase())).toBe(true)
    expect(matchesQueue(b, b.requester.name.slice(0, 4).toUpperCase())).toBe(
      true
    )
    expect(matchesQueue(b, b.requester.identifier)).toBe(true)
    expect(matchesQueue(b, 'hmif')).toBe(true)
    expect(matchesQueue(b, 'tidak ada')).toBe(false)
  })
})
