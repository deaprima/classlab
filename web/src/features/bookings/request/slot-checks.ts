import { useQueries } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import {
  type AvailabilityQuery,
  type AvailabilityResponse,
} from '@/lib/api-types'
import { minutesOfDay, toApiDateTime } from '@/lib/datetime'
import { queryKeys } from '@/lib/query-keys'
import { type ConflictItem } from '@/components/conflict-list'
import { type OccurrenceValues } from './request-schema'

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/
const TIME_RE = /^([01]\d|2[0-3]):[0-5]\d$/

type SlotCheck = {
  /** True when every date is free, false when any is taken, null while unknown. */
  free: boolean | null
  /** What occupies the slots, with the date, for the summary. */
  conflicts: ConflictItem[]
}

function complete(o: OccurrenceValues): boolean {
  return (
    DATE_RE.test(o.date) &&
    TIME_RE.test(o.start_time) &&
    TIME_RE.test(o.end_time) &&
    minutesOfDay(o.end_time) > minutesOfDay(o.start_time)
  )
}

/**
 * Advisory answer to "is this room free on every chosen date?" before the user submits (the
 * "Slot tersedia" / "Bentrok" card of the summary). The server checks again and stays authoritative.
 */
export function useSlotCheck(
  roomId: string,
  occurrences: readonly OccurrenceValues[]
): SlotCheck {
  const ready = occurrences.filter(complete)
  const results = useQueries({
    queries: ready.map((o) => {
      const query: AvailabilityQuery = {
        start_at: toApiDateTime(o.date, o.start_time),
        end_at: toApiDateTime(o.date, o.end_time),
      }
      return {
        queryKey: queryKeys.availability.check(query),
        queryFn: async () =>
          (
            await api.get<AvailabilityResponse>('/availability', {
              params: query,
            })
          ).data.data,
        enabled: !!roomId,
      }
    }),
  })

  if (!roomId || ready.length === 0 || ready.length !== occurrences.length) {
    return { free: null, conflicts: [] }
  }
  if (results.some((r) => r.data === undefined)) {
    return { free: null, conflicts: [] }
  }

  const conflicts: ConflictItem[] = []
  let taken = false
  for (const r of results) {
    const entry = r.data?.find((a) => a.room.id === roomId)
    if (!entry) return { free: null, conflicts: [] }
    if (entry.status !== 'TERSEDIA') {
      taken = true
      conflicts.push(...entry.conflicts)
    }
  }
  return { free: !taken, conflicts }
}
