import {
  type AvailabilityRoom,
  type Room,
  type RoomType,
} from '@/lib/api-types'
import { addDays, toApiDateTime, todayWib } from '@/lib/datetime'

export type RoomStatusCounts = {
  /** Rooms counted (those the viewer manages). */
  total: number
  inUse: number
  available: number
  requested: number
}

/** Counts the rooms by status; `types` limits them to the room types the viewer manages (all when undefined). */
export function roomStatusCounts(
  rooms: readonly AvailabilityRoom[],
  types?: readonly RoomType[]
): RoomStatusCounts {
  const counts: RoomStatusCounts = {
    total: 0,
    inUse: 0,
    available: 0,
    requested: 0,
  }
  for (const { room, status } of rooms) {
    if (types && !types.includes(room.type)) continue
    counts.total += 1
    if (status === 'DIGUNAKAN') counts.inUse += 1
    else if (status === 'DIAJUKAN') counts.requested += 1
    else counts.available += 1
  }
  return counts
}

/** "33%" of rooms in use, 0 when there are no rooms. */
export function occupancyPercent(counts: RoomStatusCounts): number {
  return counts.total === 0
    ? 0
    : Math.round((counts.inUse / counts.total) * 100)
}

/** Today in WIB as [from, to) for the API's RFC 3339 filters. */
export function todayRange(now: Date): { from: string; to: string } {
  const today = todayWib(now)
  return {
    from: toApiDateTime(today, '00:00'),
    to: toApiDateTime(addDays(today, 1), '00:00'),
  }
}

/** A room being used right now and by what. */
export type InUseItem = {
  room: Room
  /** Lecture title, or the purpose of the booking. */
  title: string
  kind: 'SCHEDULE' | 'BOOKING'
  start_at: string
  end_at: string
}

/**
 * The rooms in use at `now`, those ending first at the top. The entry shown is the one that covers
 * `now` (schedule or booking), so the times match what is happening in the room.
 */
export function inUseNow(
  rooms: readonly AvailabilityRoom[],
  now: Date,
  types?: readonly RoomType[]
): InUseItem[] {
  const at = now.getTime()
  const items: InUseItem[] = []
  for (const { room, status, conflicts } of rooms) {
    if (status !== 'DIGUNAKAN') continue
    if (types && !types.includes(room.type)) continue
    const current =
      conflicts.find(
        (c) =>
          (c.kind === 'SCHEDULE' || c.status !== 'MENUNGGU') &&
          Date.parse(c.start_at) <= at &&
          at < Date.parse(c.end_at)
      ) ??
      conflicts.find((c) => c.kind === 'SCHEDULE' || c.status !== 'MENUNGGU')
    if (current) items.push({ room, ...current })
  }
  return items.sort((a, b) => Date.parse(a.end_at) - Date.parse(b.end_at))
}
