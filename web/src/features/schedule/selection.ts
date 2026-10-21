import { type OccurrenceValues } from '@/features/bookings/request/request-schema'

/** The slots picked on the schedule: one time range per date (YYYY-MM-DD -> range). */
export type Ranges = Record<string, { start: string; end: string }>

/** The picked ranges as the dates of one request, earliest first (BR-27). */
export function rangesToOccurrences(ranges: Ranges): OccurrenceValues[] {
  return Object.keys(ranges)
    .sort()
    .map((date) => ({
      date,
      start_time: ranges[date].start,
      end_time: ranges[date].end,
    }))
}

/** `ranges` with the range of `date` replaced, or removed when `next` is null. */
export function withRange(
  ranges: Ranges,
  date: string,
  next: { start: string; end: string } | null
): Ranges {
  const { [date]: _removed, ...rest } = ranges
  return next ? { ...rest, [date]: next } : rest
}
