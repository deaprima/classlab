// The only place that turns API timestamps into text and form values into API timestamps.
// The campus runs on WIB. Indonesia has no daylight saving, so WIB is always UTC+07:00 and a
// fixed offset is exact.

export const APP_TIME_ZONE = 'Asia/Jakarta'
const WIB_OFFSET = '+07:00'

type DateInput = string | Date

const toDate = (value: DateInput) =>
  typeof value === 'string' ? new Date(value) : value

const dateFormat = new Intl.DateTimeFormat('id-ID', {
  timeZone: APP_TIME_ZONE,
  weekday: 'long',
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})
const shortDateFormat = new Intl.DateTimeFormat('id-ID', {
  timeZone: APP_TIME_ZONE,
  weekday: 'short',
  day: 'numeric',
  month: 'short',
  year: 'numeric',
})
const timeFormat = new Intl.DateTimeFormat('en-GB', {
  timeZone: APP_TIME_ZONE,
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
})
// en-CA formats dates as YYYY-MM-DD.
const isoDateFormat = new Intl.DateTimeFormat('en-CA', {
  timeZone: APP_TIME_ZONE,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
})

/** "Selasa, 20 Oktober 2026" */
export function formatDate(value: DateInput): string {
  return dateFormat.format(toDate(value))
}

/** "Sel, 20 Okt 2026" */
export function formatDateShort(value: DateInput): string {
  return shortDateFormat.format(toDate(value))
}

/** "08:00" (WIB) */
export function formatTime(value: DateInput): string {
  return timeFormat.format(toDate(value))
}

/** "Sel, 20 Okt 2026, 08:00" */
export function formatDateTime(value: DateInput): string {
  return `${formatDateShort(value)}, ${formatTime(value)}`
}

/** "08:00–10:00", or with the date when the range crosses midnight. */
export function formatTimeRange(start: DateInput, end: DateInput): string {
  if (toWibDate(start) === toWibDate(end)) {
    return `${formatTime(start)}–${formatTime(end)}`
  }
  return `${formatDateTime(start)} – ${formatDateTime(end)}`
}

/** "Sel, 20 Okt 2026, 08:00–10:00" */
export function formatSlot(start: DateInput, end: DateInput): string {
  if (toWibDate(start) === toWibDate(end)) {
    return `${formatDateShort(start)}, ${formatTimeRange(start, end)}`
  }
  return formatTimeRange(start, end)
}

/** WIB calendar date of an instant, as YYYY-MM-DD. */
export function toWibDate(value: DateInput): string {
  return isoDateFormat.format(toDate(value))
}

/** WIB wall-clock time of an instant, as HH:mm. */
export function toWibTime(value: DateInput): string {
  return formatTime(value)
}

/** API timestamp for a WIB date (YYYY-MM-DD) and time (HH:mm), e.g. 2026-10-20T08:00:00+07:00. */
export function toApiDateTime(date: string, time: string): string {
  return `${date}T${time}:00${WIB_OFFSET}`
}

export function todayWib(now: Date = new Date()): string {
  return toWibDate(now)
}

/** Calendar arithmetic on YYYY-MM-DD strings, independent of the browser's time zone. */
export function addDays(date: string, days: number): string {
  const d = new Date(`${date}T00:00:00Z`)
  d.setUTCDate(d.getUTCDate() + days)
  return d.toISOString().slice(0, 10)
}

/** Monday of the week containing `date` (YYYY-MM-DD). */
export function startOfWeek(date: string): string {
  const day = new Date(`${date}T00:00:00Z`).getUTCDay() // 0 = Sunday
  return addDays(date, day === 0 ? -6 : 1 - day)
}

/** "08:30" -> 510 */
export function minutesOfDay(time: string): number {
  const [h, m] = time.split(':').map(Number)
  return h * 60 + m
}

/** 510 -> "08:30" */
export function timeFromMinutes(minutes: number): string {
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`
}

const relativeFormat = new Intl.RelativeTimeFormat('id-ID', { numeric: 'auto' })

/** "5 menit yang lalu", "kemarin"; older than a week falls back to the date. */
export function formatRelative(
  value: DateInput,
  now: Date = new Date()
): string {
  const seconds = Math.round((toDate(value).getTime() - now.getTime()) / 1000)
  const abs = Math.abs(seconds)
  if (abs < 60) return 'baru saja'
  if (abs < 3600)
    return relativeFormat.format(Math.round(seconds / 60), 'minute')
  if (abs < 86400)
    return relativeFormat.format(Math.round(seconds / 3600), 'hour')
  if (abs < 7 * 86400)
    return relativeFormat.format(Math.round(seconds / 86400), 'day')
  return formatDateTime(value)
}
