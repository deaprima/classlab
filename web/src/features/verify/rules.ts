import { type Checkout } from '@/lib/api-types'
import {
  addDays,
  formatDateShort,
  formatTime,
  toApiDateTime,
  toWibDate,
  todayWib,
} from '@/lib/datetime'

export const RANGES = ['today', 'yesterday', 'week', 'month'] as const
export type RangeKey = (typeof RANGES)[number]

export const RANGE_LABELS: Record<RangeKey, string> = {
  today: 'Hari ini',
  yesterday: 'Kemarin',
  week: '7 hari terakhir',
  month: '30 hari terakhir',
}

/** [from, to) of the chosen range in WIB days, ending with today (or with yesterday for "Kemarin"). */
export function rangeFor(
  key: RangeKey,
  now: Date
): { from: string; to: string } {
  const today = todayWib(now)
  const first = {
    today,
    yesterday: addDays(today, -1),
    week: addDays(today, -6),
    month: addDays(today, -29),
  }[key]
  const last = key === 'yesterday' ? today : addDays(today, 1)
  return {
    from: toApiDateTime(first, '00:00'),
    to: toApiDateTime(last, '00:00'),
  }
}

/** Free filter of the loaded rows by room type; the API only filters by room and time. */
export function filterByType(
  rows: readonly Checkout[],
  type: Checkout['room_type'] | undefined
): Checkout[] {
  return type ? rows.filter((c) => c.room_type === type) : [...rows]
}

/** "Hari ini · 10:02", "Kemarin · 16:10", else "Jum, 16 Okt · 11:58". */
export function whenLabel(value: string, now: Date): string {
  const day = toWibDate(value)
  const today = todayWib(now)
  const time = formatTime(value)
  if (day === today) return `Hari ini · ${time}`
  if (day === addDays(today, -1)) return `Kemarin · ${time}`
  return `${formatDateShort(value).replace(/ \d{4}$/, '')} · ${time}`
}
