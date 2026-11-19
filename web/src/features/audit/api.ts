import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import {
  type AuditLogListQuery,
  type AuditLogListResponse,
} from '@/lib/api-types'

/** Audit log, newest first (ADMIN). Append-only on the server; this page only reads it. */
export function useAuditLogs(query: AuditLogListQuery) {
  return useQuery({
    queryKey: ['audit-logs', 'list', query] as const,
    queryFn: async () =>
      (await api.get<AuditLogListResponse>('/audit-logs', { params: query }))
        .data,
    placeholderData: keepPreviousData,
  })
}
