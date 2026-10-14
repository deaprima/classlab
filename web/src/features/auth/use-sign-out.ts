import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { getErrorMessage } from '@/lib/api-error'

/**
 * Revokes the refresh session on the server, then forgets everything locally. If the server cannot
 * be reached the user stays signed in and is told so: clearing only the local state would leave a
 * live refresh cookie behind, which matters on shared lab computers.
 */
export function useSignOut() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => api.post('/auth/logout'),
    onSuccess: () => {
      useAuthStore.getState().auth.reset()
      queryClient.clear()
      navigate({ to: '/sign-in', replace: true })
    },
    onError: (error) => {
      toast.error(`Gagal keluar. ${getErrorMessage(error)}`)
    },
  })
}
