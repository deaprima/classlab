import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { type Checkout, type PaginationMeta } from '@/lib/api-types'

type Page<T> = { data: T[]; meta: PaginationMeta }

/** Checkouts in [from, to), newest first (OB, TU, KALAB, ADMIN; the server scopes TU and KALAB to their types). */
export function useCheckouts(query: {
  from: string
  to: string
  per_page: number
}) {
  return useQuery({
    queryKey: ['checkouts', 'list', query] as const,
    queryFn: async () =>
      (await api.get<Page<Checkout>>('/checkouts', { params: query })).data,
    placeholderData: keepPreviousData,
  })
}

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
