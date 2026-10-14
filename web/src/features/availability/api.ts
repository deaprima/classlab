import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import {
  type AvailabilityQuery,
  type AvailabilityResponse,
} from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

/** Status of every bookable room for [start_at, end_at). Pass `null` to skip (e.g. invalid range). */
export function useAvailability(query: AvailabilityQuery | null) {
  return useQuery({
    queryKey: queryKeys.availability.check(
      query ?? { start_at: '', end_at: '' }
    ),
    queryFn: async () =>
      (await api.get<AvailabilityResponse>('/availability', { params: query }))
        .data.data,
    enabled: query !== null,
    placeholderData: keepPreviousData,
  })
}
