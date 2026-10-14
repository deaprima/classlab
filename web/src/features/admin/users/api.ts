import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import {
  type CreateUserRequest,
  type UpdateUserRequest,
  type User,
  type UserListQuery,
  type UserListResponse,
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

/** Also used to reset a password: deactivation and password resets end the user's sessions. */
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
