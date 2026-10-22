import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/lib/api-client'
import {
  type CreateUserRequest,
  type ResetPasswordResponse,
  type UpdateUserRequest,
  type User,
  type UserListQuery,
  type UserListResponse,
  type Violation,
  type ViolationListResponse,
} from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

export function useUsers(query: UserListQuery) {
  return useQuery({
    queryKey: queryKeys.users.list(query),
    queryFn: async () =>
      (await api.get<UserListResponse>('/users', { params: query })).data,
    placeholderData: keepPreviousData,
  })
}

// onError is only passed through when given: an explicit undefined would replace the default toast
// handler, since TanStack Query merges mutation options with a spread.
type MutationOptions = {
  onSuccess?: (user: User) => void
  onError?: (e: unknown) => void
}

export function useCreateUser(options: MutationOptions = {}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: CreateUserRequest) =>
      (await api.post<User>('/users', body)).data,
    onSuccess: (user) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.users.all })
      options.onSuccess?.(user)
    },
    ...(options.onError && { onError: options.onError }),
  })
}

export function useUpdateUser(options: MutationOptions = {}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, body }: { id: string; body: UpdateUserRequest }) =>
      (await api.patch<User>(`/users/${id}`, body)).data,
    onSuccess: (user) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.users.all })
      options.onSuccess?.(user)
    },
    ...(options.onError && { onError: options.onError }),
  })
}

/**
 * Replaces the password with a random temporary one and ends the user's sessions. The temporary
 * password is returned only once; keep it in the mutation state only, never in a cache or storage.
 */
export function useResetPassword(
  options: { onError?: (e: unknown) => void } = {}
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) =>
      (await api.post<ResetPasswordResponse>(`/users/${id}/reset-password`))
        .data,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.users.all })
    },
    ...(options.onError && { onError: options.onError }),
  })
}

/** A requester's violations, newest first, with the active points and any running booking block. */
export function useUserViolations(userId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.users.violations(userId ?? ''),
    queryFn: async () =>
      (
        await api.get<ViolationListResponse>(`/users/${userId}/violations`, {
          params: { per_page: 50 },
        })
      ).data,
    enabled: !!userId,
  })
}

/** Cancels one violation: its points stop counting. A block already running is lifted separately. */
export function useAnnulViolation(userId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, reason }: { id: string; reason: string }) =>
      (await api.post<Violation>(`/violations/${id}/annul`, { reason })).data,
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.users.violations(userId),
      })
      queryClient.invalidateQueries({ queryKey: queryKeys.users.all })
      toast.success('Pelanggaran di-annul.')
    },
  })
}
