import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import {
  type Room,
  type RoomListQuery,
  type RoomQR,
  type RoomListResponse,
  type RoomScheduleResponse,
} from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

export function useRooms(query: RoomListQuery) {
  return useQuery({
    queryKey: queryKeys.rooms.list(query),
    queryFn: async () =>
      (await api.get<RoomListResponse>('/rooms', { params: query })).data,
    placeholderData: keepPreviousData,
  })
}

export function useRoom(id: string) {
  return useQuery({
    queryKey: queryKeys.rooms.detail(id),
    queryFn: async () => (await api.get<Room>(`/rooms/${id}`)).data,
  })
}

/** Academic schedule plus approved bookings in [from, to). Pending bookings are not included. */
export function useRoomSchedule(id: string, from: string, to: string) {
  return useQuery({
    queryKey: queryKeys.rooms.schedule(id, from, to),
    queryFn: async () =>
      (
        await api.get<RoomScheduleResponse>(`/rooms/${id}/schedule`, {
          params: { from, to },
        })
      ).data.data,
    placeholderData: keepPreviousData,
  })
}

/** The check-in QR of a room (managers only). The token is secret, so it is never cached for long. */
export function useRoomQr(id: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.rooms.qr(id),
    queryFn: async () => (await api.get<RoomQR>(`/rooms/${id}/qr`)).data,
    enabled,
    gcTime: 0,
  })
}
