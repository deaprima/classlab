import { AxiosError } from 'axios'
import { useAuthStore } from '@/stores/auth-store'
import { api, refreshAccessToken } from './api-client'
import { type User } from './api-types'

let restoring: Promise<boolean> | null = null

/**
 * Makes sure there is a session in memory, restoring it from the refresh cookie after a reload.
 * Resolves false when there is no usable session; network and server errors are rethrown so the
 * route shows an error instead of pretending the user is logged out.
 */
export function ensureSession(): Promise<boolean> {
  if (useAuthStore.getState().auth.isAuthenticated) return Promise.resolve(true)
  restoring ??= restoreSession().finally(() => {
    restoring = null
  })
  return restoring
}

async function restoreSession(): Promise<boolean> {
  const { auth } = useAuthStore.getState()
  try {
    await refreshAccessToken()
  } catch (error) {
    if (error instanceof AxiosError && error.response?.status === 401) {
      auth.reset()
      return false
    }
    throw error
  }

  try {
    const { data } = await api.get<User>('/auth/me')
    useAuthStore.getState().auth.setUser(data)
    return true
  } catch (error) {
    useAuthStore.getState().auth.reset()
    if (error instanceof AxiosError && error.response?.status === 401) {
      return false
    }
    throw error
  }
}

/** Only same-app paths are accepted as a post-login redirect, so a crafted link cannot send users elsewhere. */
export function safeRedirectPath(redirect: string | undefined): string {
  if (!redirect || !redirect.startsWith('/') || redirect.startsWith('//')) {
    return '/'
  }
  if (redirect.startsWith('/sign-in')) return '/'
  return redirect
}
