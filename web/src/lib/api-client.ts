import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/stores/auth-store'
import { type RefreshResponse } from './api-types'

export const API_BASE_URL: string =
  import.meta.env.VITE_API_BASE_URL || '/api/v1'

export const api = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true,
  timeout: 30_000,
})

// Separate instance without the 401 interceptor, so a failed refresh cannot trigger another refresh.
export const authClient = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true,
  timeout: 30_000,
})

// The refresh cookie is rotated on every use and replaying an old one revokes the whole session family
// (decisions.md, Phase 1). Refreshes must therefore never overlap: callers in this tab share one
// request, and a Web Lock makes other tabs wait so each refresh sends the newest cookie.
const REFRESH_LOCK = 'classlab-auth-refresh'
let refreshInFlight: Promise<string> | null = null

export function refreshAccessToken(): Promise<string> {
  refreshInFlight ??= runExclusive(requestNewAccessToken).finally(() => {
    refreshInFlight = null
  })
  return refreshInFlight
}

async function requestNewAccessToken(): Promise<string> {
  const { data } = await authClient.post<RefreshResponse>('/auth/refresh')
  useAuthStore.getState().auth.setAccessToken(data.access_token)
  return data.access_token
}

function runExclusive<T>(fn: () => Promise<T>): Promise<T> {
  if (typeof navigator !== 'undefined' && navigator.locks) {
    return navigator.locks.request(REFRESH_LOCK, fn)
  }
  return fn()
}

let onSessionExpired: () => void = () => {}

/** Called once a request got 401 and the refresh that followed failed too. */
export function setSessionExpiredHandler(handler: () => void) {
  onSessionExpired = handler
}

api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().auth.accessToken
  if (token) config.headers.set('Authorization', `Bearer ${token}`)
  return config
})

type RetriableConfig = InternalAxiosRequestConfig & { _retried?: boolean }

api.interceptors.response.use(undefined, async (error: unknown) => {
  if (!(error instanceof AxiosError) || error.response?.status !== 401) {
    throw error
  }
  const config: RetriableConfig | undefined = error.config
  // A 401 from login means wrong credentials, not an expired token.
  if (!config || config._retried || isCredentialEndpoint(config.url)) {
    throw error
  }
  config._retried = true

  try {
    await refreshAccessToken()
  } catch (_refreshError) {
    onSessionExpired()
    throw error
  }
  // The request interceptor attaches the new token.
  return api.request(config)
})

function isCredentialEndpoint(url: string | undefined): boolean {
  return url === '/auth/login' || url === '/auth/refresh'
}
