import { bookingRules } from '@/config/booking-rules'
import {
  addDays,
  minutesOfDay,
  timeFromMinutes,
  toWibDate,
  toWibTime,
} from '@/lib/datetime'

type TimeRange = { date: string; start: string; end: string }

const DEFAULT_LENGTH_MINUTES = 120

/**
 * The range shown when the user has not picked one: the next slot boundary from now, two hours
 * long, inside operating hours. Late in the day it moves to tomorrow morning.
 */
export function defaultRange(now: Date = new Date()): TimeRange {
  const { slotMinutes, openTime, closeTime } = bookingRules
  const open = minutesOfDay(openTime)
  const close = minutesOfDay(closeTime)
  const nowMinutes = minutesOfDay(toWibTime(now))
  const rounded = Math.ceil(nowMinutes / slotMinutes) * slotMinutes

  let date = toWibDate(now)
  let start = Math.max(open, rounded)
  if (start + slotMinutes > close) {
    date = addDays(date, 1)
    start = open
  }
  const end = Math.min(start + DEFAULT_LENGTH_MINUTES, close)
  return { date, start: timeFromMinutes(start), end: timeFromMinutes(end) }
}

export function isValidRange(start: string, end: string): boolean {
  return minutesOfDay(end) > minutesOfDay(start)
}
