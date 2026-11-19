import { type Booking } from '@/lib/api-types'

const MINUTE = 60_000

/** One line of the queue: a single booking, or the first date of a multi-date request (BR-27). */
export type QueueItem = {
  /** The booking the row stands for; approving or rejecting it acts on the whole group. */
  lead: Booking
  /** Dates of the same request on the page, including `lead`. */
  count: number
}

/**
 * Folds the dates of one multi-date request into a single line. The API lists one row per date and
 * approving or rejecting any of them decides the whole group, so the queue shows it once. Only rows
 * of the same page are folded; the rest of the group shows up on its own page.
 */
export function groupQueue(rows: readonly Booking[]): QueueItem[] {
  const items: QueueItem[] = []
  const byGroup = new Map<string, QueueItem>()
  for (const booking of rows) {
    const groupId = booking.group_id
    const existing = groupId ? byGroup.get(groupId) : undefined
    if (existing) {
      existing.count += 1
      continue
    }
    const item: QueueItem = { lead: booking, count: 1 }
    items.push(item)
    if (groupId) byGroup.set(groupId, item)
  }
  return items
}

/** "5 j 20 m", "20 m", or null once the deadline has passed (BR-22). */
export function expiryLabel(expiresAt: string, now: Date): string | null {
  const minutes = Math.floor((Date.parse(expiresAt) - now.getTime()) / MINUTE)
  if (minutes <= 0) return null
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  if (hours === 0) return `${rest} m`
  return rest === 0 ? `${hours} j` : `${hours} j ${rest} m`
}

export const MANUAL_REASON_LABELS: Record<
  NonNullable<Booking['manual_reason']>,
  string
> = {
  ROOM_POLICY: 'Kebijakan ruang',
  ORGANIZATION: 'Organisasi',
  VIOLATION_HISTORY: 'Riwayat buruk',
}

/** Free-text filter of the queue: room, requester (name or NIM/code), activity. */
export function matchesQueue(
  booking: Booking,
  query: string | undefined
): boolean {
  const q = query?.trim().toLowerCase()
  if (!q) return true
  return [
    booking.room.code,
    booking.room.name,
    booking.requester.name,
    booking.requester.identifier,
    booking.purpose,
  ].some((v) => v.toLowerCase().includes(q))
}
