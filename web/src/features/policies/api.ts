import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/lib/api-client'
import {
  type RoomType,
  type RoomTypePolicy,
  type RoomTypeUpdate,
} from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

/** Applies from the next request on; bookings already decided keep their decision. */
export function useUpdatePolicy(
  options: { onSuccess?: () => void; onError?: (e: unknown) => void } = {}
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      type,
      body,
    }: {
      type: RoomType
      body: RoomTypeUpdate
    }) => (await api.patch<RoomTypePolicy>(`/room-types/${type}`, body)).data,
    onSuccess: (policy) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.roomTypes.all })
      toast.success(`Kebijakan ${policy.label} diperbarui.`)
      options.onSuccess?.()
    },
    ...(options.onError && { onError: options.onError }),
  })
}
