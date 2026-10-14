import { minutesOfDay, timeFromMinutes } from '@/lib/datetime'

// BR-01 as the browser sees it. The server reads the same values from its own env and stays the
// source of truth; these only let the form reject obvious mistakes early. Keep the VITE_* values
// in step with OPERATING_HOURS_* / BOOKING_* in the API's .env.
const env = import.meta.env

function positiveInt(value: string | undefined, fallback: number): number {
  const n = Number(value)
  return Number.isInteger(n) && n > 0 ? n : fallback
}

function hhmm(value: string | undefined, fallback: string): string {
  return value && /^([01]\d|2[0-3]):[0-5]\d$/.test(value) ? value : fallback
}

export const bookingRules = {
  /** Fixed by BR-01, not configurable. */
  slotMinutes: 30,
  openTime: hhmm(env.VITE_OPERATING_HOURS_START, '07:00'),
  closeTime: hhmm(env.VITE_OPERATING_HOURS_END, '21:00'),
  minDurationMinutes: positiveInt(env.VITE_BOOKING_MIN_DURATION_MINUTES, 30),
  maxDurationMinutes: positiveInt(env.VITE_BOOKING_MAX_DURATION_HOURS, 8) * 60,
  maxFutureDays: positiveInt(env.VITE_BOOKING_MAX_FUTURE_DAYS, 30),
} as const

/** Every slot boundary inside operating hours: 07:00, 07:30, ..., 21:00. */
export function slotTimes(
  open: string = bookingRules.openTime,
  close: string = bookingRules.closeTime,
  step: number = bookingRules.slotMinutes
): string[] {
  const out: string[] = []
  for (let m = minutesOfDay(open); m <= minutesOfDay(close); m += step) {
    out.push(timeFromMinutes(m))
  }
  return out
}
