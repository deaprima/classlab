import {
  type Room,
  type RoomTypePolicy,
  type TimelineItem,
} from '@/lib/api-types'
import { minutesOfDay, timeFromMinutes } from '@/lib/datetime'
import { effectiveHours } from '@/features/bookings/request/policy'
import {
  type PlacedItem,
  placeItems,
  rowCount,
} from '@/features/rooms/timeline-layout'

export const SLOT_MINUTES = 30
const FALLBACK = { open: '07:00', close: '21:00' }

/** Hours of a room: its own, else its type's; the campus default while the policies load. */
export function roomHours(
  room: Pick<Room, 'type' | 'open_time' | 'close_time'>,
  policies: readonly RoomTypePolicy[]
): { open: string; close: string } {
  const policy = policies.find((p) => p.type === room.type)
  return policy ? effectiveHours(room, policy) : FALLBACK
}

/** The earliest opening and the latest closing of the rooms, so every room fits on one time axis. */
export function boardHours(
  rooms: readonly Pick<Room, 'type' | 'open_time' | 'close_time'>[],
  policies: readonly RoomTypePolicy[]
): { open: string; close: string } {
  if (rooms.length === 0) return FALLBACK
  let open = Infinity
  let close = -Infinity
  for (const room of rooms) {
    const h = roomHours(room, policies)
    open = Math.min(open, minutesOfDay(h.open))
    close = Math.max(close, minutesOfDay(h.close))
  }
  return { open: timeFromMinutes(open), close: timeFromMinutes(close) }
}

/** Where the items of one room sit in its column: row range of the board's 30-minute rows. */
export function placeInColumn(
  items: readonly TimelineItem[],
  date: string,
  hours: { open: string; close: string }
): PlacedItem[] {
  return placeItems([...items], {
    days: [date],
    openTime: hours.open,
    closeTime: hours.close,
    slotMinutes: SLOT_MINUTES,
  })
}

export function boardRows(hours: { open: string; close: string }): number {
  return rowCount({
    days: [],
    openTime: hours.open,
    closeTime: hours.close,
    slotMinutes: SLOT_MINUTES,
  })
}

/** Row (0-based) holding `now`, or null when `now` is outside the board's day and hours. */
export function currentRow(
  now: Date,
  date: string,
  hours: { open: string; close: string }
): number | null {
  const start = Date.parse(`${date}T${hours.open}:00+07:00`)
  const row = Math.floor((now.getTime() - start) / (SLOT_MINUTES * 60_000))
  return row >= 0 && row < boardRows(hours) ? row : null
}
