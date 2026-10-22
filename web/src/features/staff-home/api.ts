import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { type PaginationMeta } from '@/lib/api-types'

type Page<T> = { data: T[]; meta: PaginationMeta }

/** How many audit entries were written in [from, to) (ADMIN). */
export function useAuditCount(range: { from: string; to: string }) {
  return useQuery({
    queryKey: ['audit-logs', 'count', range] as const,
    queryFn: async () =>
      (
        await api.get<Page<unknown>>('/audit-logs', {
          params: { ...range, per_page: 1 },
        })
      ).data.meta.total,
  })
}
