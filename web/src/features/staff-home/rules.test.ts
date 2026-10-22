import { describe, expect, it } from 'vitest'
import { type AvailabilityRoom, type Room } from '@/lib/api-types'
import {
  inUseNow,
  occupancyPercent,
  roomStatusCounts,
  todayRange,
} from './rules'

const room = (type: Room['type']) => ({ type }) as Room
const at = (
  type: Room['type'],
  status: AvailabilityRoom['status']
): AvailabilityRoom => ({
  room: room(type),
  status,
  operating_hours: { open_time: '07:00', close_time: '21:00', custom: false },
  conflicts: [],
})

const ROOMS = [
  at('KELAS', 'DIGUNAKAN'),
  at('KELAS', 'TERSEDIA'),
  at('LAB', 'DIAJUKAN'),
  at('LAB', 'DIGUNAKAN'),
  at('AUDITORIUM', 'TERSEDIA'),
]

describe('roomStatusCounts', () => {
  it('counts every room by status', () => {
    expect(roomStatusCounts(ROOMS)).toEqual({
      total: 5,
      inUse: 2,
      available: 2,
      requested: 1,
    })
  })

  it('limits the count to the room types the viewer manages', () => {
    expect(roomStatusCounts(ROOMS, ['LAB'])).toEqual({
      total: 2,
      inUse: 1,
      available: 0,
      requested: 1,
    })
  })

  it('computes occupancy, 0 without rooms', () => {
    expect(occupancyPercent(roomStatusCounts(ROOMS))).toBe(40)
    expect(occupancyPercent(roomStatusCounts([]))).toBe(0)
  })
})

describe('todayRange', () => {
  it('spans the WIB day of the given moment', () => {
    // 2026-10-21 18:00 UTC is already 22 Oct 01:00 WIB.
    const { from, to } = todayRange(new Date('2026-10-21T18:00:00Z'))
    expect(Date.parse(from)).toBe(Date.parse('2026-10-21T17:00:00Z'))
    expect(Date.parse(to)).toBe(Date.parse('2026-10-22T17:00:00Z'))
  })
})

describe('inUseNow', () => {
  const now = new Date('2026-10-22T03:00:00Z')
  const used = (
    code: string,
    type: Room['type'],
    start: string,
    end: string,
    kind: 'SCHEDULE' | 'BOOKING' = 'SCHEDULE'
  ): AvailabilityRoom => ({
    ...at(type, 'DIGUNAKAN'),
    room: { code, type } as Room,
    conflicts: [
      {
        kind,
        title: `Kegiatan ${code}`,
        ...(kind === 'BOOKING' && { status: 'BERLANGSUNG' as const }),
        start_at: start,
        end_at: end,
      },
    ],
  })

  it('lists rooms in use, the ones ending first on top', () => {
    const rooms = [
      used('R-2', 'KELAS', '2026-10-22T02:00:00Z', '2026-10-22T05:00:00Z'),
      used('R-1', 'KELAS', '2026-10-22T02:30:00Z', '2026-10-22T04:00:00Z'),
      at('KELAS', 'TERSEDIA'),
    ]
    expect(inUseNow(rooms, now).map((i) => i.room.code)).toEqual(['R-1', 'R-2'])
  })

  it('keeps to the types the viewer manages', () => {
    const rooms = [
      used('R-1', 'KELAS', '2026-10-22T02:00:00Z', '2026-10-22T05:00:00Z'),
      used('LAB-1', 'LAB', '2026-10-22T02:00:00Z', '2026-10-22T05:00:00Z'),
    ]
    expect(inUseNow(rooms, now, ['LAB']).map((i) => i.room.code)).toEqual([
      'LAB-1',
    ])
  })

  it('shows the entry that covers now, not a later one the same day', () => {
    const room: AvailabilityRoom = {
      ...used('R-1', 'KELAS', '2026-10-22T08:00:00Z', '2026-10-22T09:00:00Z'),
    }
    room.conflicts.unshift({
      kind: 'BOOKING',
      status: 'BERLANGSUNG',
      title: 'Sekarang',
      start_at: '2026-10-22T02:00:00Z',
      end_at: '2026-10-22T04:00:00Z',
    })
    expect(inUseNow([room], now)[0]?.title).toBe('Sekarang')
  })
})
