import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { type RoomTypeListResponse } from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

/**
 * Policy of every room type (approvers, requesters, hours, limits). The server decides on each
 * action; the client only uses this to shape menus and forms. It changes rarely, so it stays fresh
 * for the whole session.
 */
export function useRoomTypes() {
  return useQuery({
    queryKey: queryKeys.roomTypes.all,
    queryFn: async () =>
      (await api.get<RoomTypeListResponse>('/room-types')).data.data,
    staleTime: Infinity,
  })
}
