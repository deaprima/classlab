import { useQueries } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { type Room, type RoomScheduleResponse } from '@/lib/api-types'
import { addDays, toApiDateTime } from '@/lib/datetime'
import { queryKeys } from '@/lib/query-keys'

/**
 * The timeline of every room for one day. The API has no all-rooms timeline, so each room is read
 * on its own; the keys match `useRoomSchedule`, so pages that show the same day share the cache.
 */
export function useRoomsDay(rooms: readonly Room[], date: string) {
  const from = toApiDateTime(date, '00:00')
  const to = toApiDateTime(addDays(date, 1), '00:00')
  return useQueries({
    queries: rooms.map((room) => ({
      queryKey: queryKeys.rooms.schedule(room.id, from, to),
      queryFn: async () =>
        (
          await api.get<RoomScheduleResponse>(`/rooms/${room.id}/schedule`, {
            params: { from, to },
          })
        ).data.data,
      refetchInterval: 60_000,
    })),
  })
}
