import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { type ViolationListResponse } from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

/** The reader's own latest violations (a requester may read only their own record). */
export function useMyViolations(userId: string | undefined, limit: number) {
  return useQuery({
    queryKey: [...queryKeys.users.violations(userId ?? ''), limit],
    queryFn: async () =>
      (
        await api.get<ViolationListResponse>(`/users/${userId}/violations`, {
          params: { per_page: limit },
        })
      ).data,
    enabled: !!userId,
  })
}
