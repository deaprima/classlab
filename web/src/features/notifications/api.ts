import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import {
  type AppNotification,
  type NotificationListQuery,
  type NotificationListResponse,
} from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'
import { useNotificationStreamStore } from './stream-store'

/** brief.md 6.2: fallback polling every 30 s when the stream is not available. */
const POLL_INTERVAL_MS = 30_000

export function useNotifications(query: NotificationListQuery) {
  const connected = useNotificationStreamStore((s) => s.connected)
  return useQuery({
    queryKey: queryKeys.notifications.list(query),
    queryFn: async () =>
      (
        await api.get<NotificationListResponse>('/notifications', {
          params: query,
        })
      ).data,
    placeholderData: keepPreviousData,
    refetchInterval: connected ? false : POLL_INTERVAL_MS,
  })
}

/** Unread count for the header badge: `meta.total` of the unread filter (decisions.md, Phase 4). */
export function useUnreadCount() {
  const connected = useNotificationStreamStore((s) => s.connected)
  return useQuery({
    queryKey: queryKeys.notifications.unreadCount,
    queryFn: async () =>
      (
        await api.get<NotificationListResponse>('/notifications', {
          params: { unread: true, per_page: 1 },
        })
      ).data.meta.total ?? 0,
    refetchInterval: connected ? false : POLL_INTERVAL_MS,
  })
}

export function useMarkRead() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) =>
      (await api.post<AppNotification>(`/notifications/${id}/read`)).data,
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all }),
  })
}

export function useMarkAllRead() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async () =>
      (await api.post<{ updated: number }>('/notifications/read-all')).data,
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all }),
  })
}
