import {
  type QueryClient,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/lib/api-client'
import {
  type CreateRoomRequest,
  type Room,
  type UpdateRoomRequest,
} from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

// Rooms appear in availability, timelines, booking and report forms: refresh all of them.
function invalidateRooms(queryClient: QueryClient) {
  queryClient.invalidateQueries({ queryKey: queryKeys.rooms.all })
  queryClient.invalidateQueries({ queryKey: queryKeys.availability.all })
}

// onError is only passed through when given: an explicit undefined would replace the default toast
// handler, since TanStack Query merges mutation options with a spread.
type MutationOptions = {
  onSuccess?: () => void
  onError?: (e: unknown) => void
}

export function useCreateRoom(options: MutationOptions = {}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: CreateRoomRequest) =>
      (await api.post<Room>('/rooms', body)).data,
    onSuccess: (room) => {
      invalidateRooms(queryClient)
      toast.success(`Ruang ${room.code} ditambahkan.`)
      options.onSuccess?.()
    },
    ...(options.onError && { onError: options.onError }),
  })
}

export function useUpdateRoom(options: MutationOptions = {}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, body }: { id: string; body: UpdateRoomRequest }) =>
      (await api.patch<Room>(`/rooms/${id}`, body)).data,
    onSuccess: (room) => {
      invalidateRooms(queryClient)
      toast.success(`Ruang ${room.code} diperbarui.`)
      options.onSuccess?.()
    },
    ...(options.onError && { onError: options.onError }),
  })
}

/** Soft delete (BR-07). A room with upcoming active bookings answers 409 ROOM_HAS_ACTIVE_BOOKINGS. */
export function useDeleteRoom(options: MutationOptions = {}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (room: Room) => {
      await api.delete(`/rooms/${room.id}`)
      return room
    },
    onSuccess: (room) => {
      invalidateRooms(queryClient)
      toast.success(`Ruang ${room.code} dihapus.`)
      options.onSuccess?.()
    },
    ...(options.onError && { onError: options.onError }),
  })
}
