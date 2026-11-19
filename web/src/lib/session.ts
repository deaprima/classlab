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

export const CHANGE_PASSWORD_PATH = '/change-password'

/**
 * Where an account with a temporary password must go instead of `pathname`: it may only use the
 * change-password page (the API answers 403 PASSWORD_CHANGE_REQUIRED elsewhere). Null when the
 * user may stay.
 */
export function passwordChangeRedirect(
  user: Pick<User, 'must_change_password'> | null,
  pathname: string
): string | null {
  if (!user?.must_change_password) return null
  return pathname === CHANGE_PASSWORD_PATH ? null : CHANGE_PASSWORD_PATH
}
