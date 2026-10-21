import { type TimelineItem } from '@/lib/api-types'
import { minutesOfDay, timeFromMinutes, toApiDateTime } from '@/lib/datetime'

export type SlotState = 'free' | 'used' | 'past'

export type DaySlot = {
  /** Start of the slot, "08:00". */
  time: string
  state: SlotState
  /** What occupies it, when `used`. */
  item?: TimelineItem
}

type Hours = { open: string; close: string }

/**
 * The 30-minute slots of one WIB day inside the room's hours, each marked free, used (a lecture
 * schedule or an approved booking covers it) or past. `now` is injected so tests are deterministic.
 */
export function buildDaySlots(input: {
  date: string
  hours: Hours
  items: readonly TimelineItem[]
  now: Date
  slotMinutes?: number
}): DaySlot[] {
  const step = input.slotMinutes ?? 30
  const open = minutesOfDay(input.hours.open)
  const close = minutesOfDay(input.hours.close)
  const slots: DaySlot[] = []

  for (let m = open; m + step <= close; m += step) {
    const time = timeFromMinutes(m)
    const start = Date.parse(toApiDateTime(input.date, time))
    const end = start + step * 60_000
    // Half-open intervals: a lecture ending at 10:00 does not occupy the 10:00 slot.
    const item = input.items.find(
      (i) => Date.parse(i.start_at) < end && Date.parse(i.end_at) > start
    )
    slots.push({
      time,
      state: item ? 'used' : start < input.now.getTime() ? 'past' : 'free',
      ...(item && { item }),
    })
  }
  return slots
}

/**
 * Click behaviour of the slot chips: a first click picks a start (one slot long), a second click
 * on a later free slot stretches the range to include it, but never across a used slot. Any other
 * click starts over from the clicked slot.
 */
export function pickSlot(
  slots: readonly DaySlot[],
  range: { start: string; end: string } | null,
  clicked: string,
  slotMinutes = 30
): { start: string; end: string } | null {
  const target = slots.find((s) => s.time === clicked)
  if (!target || target.state !== 'free') return range

  const fresh = {
    start: clicked,
    end: timeFromMinutes(minutesOfDay(clicked) + slotMinutes),
  }
  if (!range || minutesOfDay(clicked) < minutesOfDay(range.start)) return fresh
  if (minutesOfDay(clicked) < minutesOfDay(range.end)) return fresh

  const between = slots.filter(
    (s) =>
      minutesOfDay(s.time) >= minutesOfDay(range.start) &&
      minutesOfDay(s.time) <= minutesOfDay(clicked)
  )
  if (between.some((s) => s.state !== 'free')) return fresh
  return {
    start: range.start,
    end: timeFromMinutes(minutesOfDay(clicked) + slotMinutes),
  }
}

/** Whether a slot lies inside the picked range, for highlighting. */
export function inRange(
  time: string,
  range: { start: string; end: string } | null
): boolean {
  if (!range) return false
  const m = minutesOfDay(time)
  return m >= minutesOfDay(range.start) && m < minutesOfDay(range.end)
}

/**
 * Click on a slot of a day that may already have a picked range: the same single slot toggles off,
 * anything else follows `pickSlot`. Returns the new range of that day, or null for none.
 */
export function selectSlot(
  slots: readonly DaySlot[],
  current: { start: string; end: string } | null,
  clicked: string,
  slotMinutes = 30
): { start: string; end: string } | null {
  if (
    current &&
    current.start === clicked &&
    minutesOfDay(current.end) - minutesOfDay(current.start) === slotMinutes
  ) {
    return null
  }
  return pickSlot(slots, current, clicked, slotMinutes)
}
