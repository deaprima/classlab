import { describe, expect, it } from 'vitest'
import {
  addDays,
  dateParts,
  formatDate,
  formatDuration,
  formatSlot,
  formatTime,
  formatTimeRange,
  minutesOfDay,
  startOfWeek,
  timeFromMinutes,
  toApiDateTime,
  toWibDate,
  toWibTime,
  todayWib,
} from './datetime'

describe('datetime (WIB)', () => {
  it('builds API timestamps with the +07:00 offset', () => {
    expect(toApiDateTime('2026-10-20', '08:00')).toBe(
      '2026-10-20T08:00:00+07:00'
    )
  })

  it('formats instants in WIB whatever their offset', () => {
    // 01:00 UTC = 08:00 WIB
    expect(formatTime('2026-10-20T01:00:00Z')).toBe('08:00')
    expect(formatTime('2026-10-20T08:00:00+07:00')).toBe('08:00')
    expect(toWibTime('2026-10-20T13:30:00Z')).toBe('20:30')
  })

  it('uses the WIB calendar date, not the UTC one', () => {
    // 18:00 UTC on the 19th is already 01:00 WIB on the 20th.
    expect(toWibDate('2026-10-19T18:00:00Z')).toBe('2026-10-20')
    expect(todayWib(new Date('2026-10-19T18:00:00Z'))).toBe('2026-10-20')
  })

  it('formats dates and ranges in Indonesian', () => {
    expect(formatDate('2026-10-20T08:00:00+07:00')).toBe(
      'Selasa, 20 Oktober 2026'
    )
    expect(
      formatTimeRange('2026-10-20T08:00:00+07:00', '2026-10-20T10:00:00+07:00')
    ).toBe('08:00 – 10:00')
    expect(
      formatSlot('2026-10-20T08:00:00+07:00', '2026-10-20T10:00:00+07:00')
    ).toBe('Sel, 20 Okt 2026, 08:00 – 10:00')
  })

  it('does calendar arithmetic across month and year ends', () => {
    expect(addDays('2026-10-31', 1)).toBe('2026-11-01')
    expect(addDays('2026-12-31', 1)).toBe('2027-01-01')
    expect(addDays('2026-03-01', -1)).toBe('2026-02-28')
  })

  it('finds the Monday of a week', () => {
    expect(startOfWeek('2026-10-20')).toBe('2026-10-19') // Tuesday
    expect(startOfWeek('2026-10-19')).toBe('2026-10-19') // Monday
    expect(startOfWeek('2026-10-25')).toBe('2026-10-19') // Sunday
  })

  it('converts between HH:mm and minutes', () => {
    expect(minutesOfDay('07:30')).toBe(450)
    expect(timeFromMinutes(450)).toBe('07:30')
    expect(timeFromMinutes(21 * 60)).toBe('21:00')
  })

  it('splits a calendar date for the date badge', () => {
    expect(dateParts('2026-10-19')).toEqual({
      day: '19',
      weekday: 'Sen',
      month: 'Okt',
    })
    expect(dateParts('2026-12-31').month).toBe('Des')
  })

  it('formats durations', () => {
    expect(formatDuration(120)).toBe('2 jam')
    expect(formatDuration(30)).toBe('30 menit')
    expect(formatDuration(90)).toBe('1 jam 30 menit')
  })
})
