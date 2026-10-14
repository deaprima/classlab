import { type TimelineItem } from '@/lib/api-types'
import { minutesOfDay, toWibDate, toWibTime } from '@/lib/datetime'

export type GridSpec = {
  /** YYYY-MM-DD, one column each. */
  days: string[]
  openTime: string
  closeTime: string
  slotMinutes: number
}

export type PlacedItem = {
  item: TimelineItem
  /** 0-based column. */
  day: number
  /** 0-based slot rows, end exclusive. */
  rowStart: number
  rowEnd: number
}

export function rowCount(spec: GridSpec): number {
  return (
    (minutesOfDay(spec.closeTime) - minutesOfDay(spec.openTime)) /
    spec.slotMinutes
  )
}

/**
 * Where each item sits on the day x slot grid. Items are clipped to operating hours and dropped when
 * nothing of them is visible. Schedules and approved bookings never overlap (exclusion constraints),
 * so one column needs no lane splitting.
 */
export function placeItems(
  items: TimelineItem[],
  spec: GridSpec
): PlacedItem[] {
  const open = minutesOfDay(spec.openTime)
  const close = minutesOfDay(spec.closeTime)
  const placed: PlacedItem[] = []

  for (const item of items) {
    const date = toWibDate(item.start_at)
    const day = spec.days.indexOf(date)
    if (day === -1) continue

    const startMin = minutesOfDay(toWibTime(item.start_at))
    const endMin =
      toWibDate(item.end_at) === date
        ? minutesOfDay(toWibTime(item.end_at))
        : 24 * 60
    const from = Math.max(startMin, open)
    const to = Math.min(endMin, close)
    if (to <= from) continue

    placed.push({
      item,
      day,
      rowStart: Math.floor((from - open) / spec.slotMinutes),
      rowEnd: Math.ceil((to - open) / spec.slotMinutes),
    })
  }
  return placed
}
