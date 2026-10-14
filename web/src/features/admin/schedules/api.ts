import {
  type QueryClient,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/lib/api-client'
import {
  type CreateScheduleRequest,
  type Schedule,
  type UpdateScheduleRequest,
} from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

// Schedules feed room timelines (under the rooms key) and availability.
function invalidateSchedules(queryClient: QueryClient) {
  queryClient.invalidateQueries({ queryKey: queryKeys.rooms.all })
  queryClient.invalidateQueries({ queryKey: queryKeys.availability.all })
}

// onError is only passed through when given: an explicit undefined would replace the default toast
// handler, since TanStack Query merges mutation options with a spread.
type MutationOptions = {
  onSuccess?: () => void
  onError?: (e: unknown) => void
}

export function useCreateSchedule(options: MutationOptions = {}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: CreateScheduleRequest) =>
      (await api.post<{ data: Schedule[] }>('/schedules', body)).data.data,
    onSuccess: (created) => {
      invalidateSchedules(queryClient)
      toast.success(
        created.length > 1
          ? `${created.length} sesi jadwal mingguan dibuat.`
          : 'Jadwal dibuat.'
      )
      options.onSuccess?.()
    },
    ...(options.onError && { onError: options.onError }),
  })
}

export function useUpdateSchedule(options: MutationOptions = {}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      id,
      body,
    }: {
      id: string
      body: UpdateScheduleRequest
    }) => (await api.patch<Schedule>(`/schedules/${id}`, body)).data,
    onSuccess: () => {
      invalidateSchedules(queryClient)
      toast.success('Jadwal diperbarui.')
      options.onSuccess?.()
    },
    ...(options.onError && { onError: options.onError }),
  })
}

export function useDeleteSchedule(options: MutationOptions = {}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => {
      await api.delete(`/schedules/${id}`)
    },
    onSuccess: () => {
      invalidateSchedules(queryClient)
      toast.success('Jadwal dihapus.')
      options.onSuccess?.()
    },
    ...(options.onError && { onError: options.onError }),
  })
}
