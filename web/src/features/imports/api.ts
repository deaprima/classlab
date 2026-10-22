import {
  keepPreviousData,
  type QueryClient,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/lib/api-client'
import {
  type ImportKind,
  type ImportListResponse,
  type ImportUsersPreview,
  type ImportUsersResult,
  type RollbackResult,
  type ScheduleApplyResult,
  type SchedulePreview,
} from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

export type ImportPreview = ImportUsersPreview | SchedulePreview
export type ImportResult = ImportUsersResult | ScheduleApplyResult

type ImportRequest = {
  kind: ImportKind
  file: File
  /** Users only, e.g. "2026-GANJIL". */
  semester?: string
  /** From the preview; required to apply. */
  planHash?: string
}

const PATH: Record<ImportKind, string> = {
  USERS: '/imports/users',
  SCHEDULES: '/imports/schedules',
}

function body({ kind, file, semester, planHash }: ImportRequest) {
  const form = new FormData()
  form.append('file', file)
  if (kind === 'USERS' && semester) form.append('semester', semester)
  if (planHash) form.append('plan_hash', planHash)
  return form
}

/** Step 1 to 2: the file is only checked; nothing is stored (`dry_run`). */
export function usePreviewImport() {
  return useMutation({
    mutationFn: async (input: ImportRequest) =>
      (
        await api.post<ImportPreview>(PATH[input.kind], body(input), {
          params: { dry_run: true },
        })
      ).data,
  })
}

// An import changes the schedules and bookings, or the accounts, and adds to the history.
function invalidateAfterImport(queryClient: QueryClient, kind: ImportKind) {
  queryClient.invalidateQueries({ queryKey: queryKeys.imports.all })
  if (kind === 'USERS') {
    queryClient.invalidateQueries({ queryKey: queryKeys.users.all })
    return
  }
  queryClient.invalidateQueries({ queryKey: queryKeys.rooms.all })
  queryClient.invalidateQueries({ queryKey: queryKeys.availability.all })
  queryClient.invalidateQueries({ queryKey: queryKeys.bookings.all })
}

/** Step 2 to 3: applies the previewed plan in one transaction. */
export function useApplyImport() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: ImportRequest) =>
      (await api.post<ImportResult>(PATH[input.kind], body(input))).data,
    onSuccess: (_, input) => invalidateAfterImport(queryClient, input.kind),
  })
}

export function useImports(query: {
  kind?: ImportKind
  page?: number
  per_page?: number
}) {
  return useQuery({
    queryKey: queryKeys.imports.list(query),
    queryFn: async () =>
      (await api.get<ImportListResponse>('/imports', { params: query })).data,
    placeholderData: keepPreviousData,
  })
}

/** Removes the not-yet-started meetings of a schedule import; cancelled bookings stay cancelled. */
export function useRollbackImport(options: { onSuccess?: () => void } = {}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) =>
      (await api.post<RollbackResult>(`/imports/${id}/rollback`)).data,
    onSuccess: (result) => {
      invalidateAfterImport(queryClient, 'SCHEDULES')
      toast.success(
        `Impor di-rollback: ${result.deleted_occurrences} pertemuan dihapus.`
      )
      options.onSuccess?.()
    },
  })
}
