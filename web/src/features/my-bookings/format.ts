import { formatDateShort, formatTimeRange } from '@/lib/datetime'

/** "Sen, 19 Okt" for a timestamp: the short date without its year. */
export function dayShort(value: string): string {
  return formatDateShort(value).replace(/ \d{4}$/, '')
}

/** "Sen, 19 Okt · 14:00 – 15:30": the "Waktu" column of the lists. */
export function whenShort(start: string, end: string): string {
  return `${dayShort(start)} · ${formatTimeRange(start, end)}`
}
