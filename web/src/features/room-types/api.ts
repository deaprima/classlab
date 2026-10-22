import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { type RoomType, type RoomTypeListResponse } from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'
import { ROOM_TYPES } from '@/features/rooms/labels'

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

/**
 * The room types a manager works with: those TU and Kalab approve, every type for Admin and OB. For
 * filter chips only; the server limits what each role can read.
 */
export function useManagedTypes(): RoomType[] {
  const role = useAuthStore((s) => s.auth.user?.role)
  const policies = useRoomTypes()
  if (role !== 'TU' && role !== 'KALAB') return [...ROOM_TYPES]
  return (policies.data ?? [])
    .filter((p) => p.approver_roles.includes(role))
    .map((p) => p.type)
}
