import { describe, expect, it } from 'vitest'
import {
  type Room,
  type RoomTypePolicy,
  type TimelineItem,
} from '@/lib/api-types'
import { boardHours, boardRows, currentRow, placeInColumn } from './rules'

const policy = (type: Room['type'], open: string, close: string) =>
  ({ type, open_time: open, close_time: close }) as RoomTypePolicy
const POLICIES = [
  policy('KELAS', '07:00', '21:00'),
  policy('AUDITORIUM', '07:00', '22:00'),
  policy('RUANG_SIDANG', '08:00', '17:00'),
]
const room = (
  type: Room['type'],
  open_time: string | null = null,
  close_time: string | null = null
) => ({ type, open_time, close_time })

describe('boardHours', () => {
  it('spans the earliest opening to the latest closing of the rooms', () => {
    expect(
      boardHours(
        [room('KELAS'), room('AUDITORIUM'), room('RUANG_SIDANG')],
        POLICIES
      )
    ).toEqual({ open: '07:00', close: '22:00' })
  })

  it('lets a room with its own hours widen the board', () => {
    expect(boardHours([room('KELAS', '06:30', '23:00')], POLICIES)).toEqual({
      open: '06:30',
      close: '23:00',
    })
  })

  it('falls back to campus hours without rooms or policies', () => {
    expect(boardHours([], POLICIES)).toEqual({ open: '07:00', close: '21:00' })
    expect(boardHours([room('KELAS')], [])).toEqual({
      open: '07:00',
      close: '21:00',
    })
  })
})

describe('placeInColumn', () => {
  const hours = { open: '07:00', close: '21:00' }
  const item = (start: string, end: string): TimelineItem => ({
    id: start,
    kind: 'SCHEDULE',
    title: 'Kuliah',
    start_at: start,
    end_at: end,
  })

  it('puts an item on the 30-minute rows of its time', () => {
    // 08:00-10:00 WIB: rows 2 to 6 on a board that opens at 07:00.
    const [placed] = placeInColumn(
      [item('2026-10-22T08:00:00+07:00', '2026-10-22T10:00:00+07:00')],
      '2026-10-22',
      hours
    )
    expect([placed.rowStart, placed.rowEnd]).toEqual([2, 6])
  })

  it('drops items of another day', () => {
    expect(
      placeInColumn(
        [item('2026-10-23T08:00:00+07:00', '2026-10-23T10:00:00+07:00')],
        '2026-10-22',
        hours
      )
    ).toEqual([])
  })
})

describe('currentRow', () => {
  const hours = { open: '07:00', close: '21:00' }

  it('finds the row of now on the board day', () => {
    // 09:10 WIB is in the row of 09:00, the fifth after 07:00.
    expect(
      currentRow(new Date('2026-10-22T02:10:00Z'), '2026-10-22', hours)
    ).toBe(4)
  })

  it('is null on another day or outside the hours', () => {
    expect(
      currentRow(new Date('2026-10-22T02:10:00Z'), '2026-10-23', hours)
    ).toBeNull()
    expect(
      currentRow(new Date('2026-10-22T20:00:00Z'), '2026-10-22', hours)
    ).toBeNull()
    expect(boardRows(hours)).toBe(28)
  })
})
