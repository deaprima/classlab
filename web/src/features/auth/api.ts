import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { type User } from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

/**
 * The profile as the server has it now. The login response omits `active_points`, which only
 * `GET /auth/me` returns (for MAHASISWA and ORGANISASI), so pages that show points read it here.
 */
export function useMe() {
  return useQuery({
    queryKey: queryKeys.auth.me,
    queryFn: async () => (await api.get<User>('/auth/me')).data,
  })
}
