import { AxiosError } from 'axios'
import {
  apiErrorBody,
  mockAdapter,
  sampleUser,
} from '@/test-utils/mock-adapter'
import { afterEach, describe, expect, it } from 'vitest'
import { useAuthStore } from '@/stores/auth-store'
import { api, authClient } from './api-client'
import { ensureSession, safeRedirectPath } from './session'

const originalApiAdapter = api.defaults.adapter
const originalAuthAdapter = authClient.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalApiAdapter
  authClient.defaults.adapter = originalAuthAdapter
  useAuthStore.getState().auth.reset()
})

describe('ensureSession', () => {
  it('restores the session from the refresh cookie and loads the profile', async () => {
    authClient.defaults.adapter = mockAdapter(() => ({
      status: 200,
      data: { access_token: 'restored', expires_in: 900 },
    }))
    api.defaults.adapter = mockAdapter((config) =>
      config.url === '/auth/me' &&
      config.headers.get('Authorization') === 'Bearer restored'
        ? { status: 200, data: sampleUser }
        : { status: 404 }
    )

    await expect(ensureSession()).resolves.toBe(true)

    const { auth } = useAuthStore.getState()
    expect(auth.isAuthenticated).toBe(true)
    expect(auth.user?.identifier).toBe(sampleUser.identifier)
  })

  it('resolves false when there is no valid refresh cookie', async () => {
    authClient.defaults.adapter = mockAdapter(() => ({
      status: 401,
      data: apiErrorBody('UNAUTHENTICATED', 'Sesi tidak valid.'),
    }))

    await expect(ensureSession()).resolves.toBe(false)
    expect(useAuthStore.getState().auth.isAuthenticated).toBe(false)
  })

  it('rethrows server failures instead of treating them as signed out', async () => {
    authClient.defaults.adapter = mockAdapter(() => ({ status: 503 }))

    await expect(ensureSession()).rejects.toBeInstanceOf(AxiosError)
  })

  it('does not call the server when a session is already in memory', async () => {
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
    authClient.defaults.adapter = mockAdapter(() => {
      throw new Error('should not refresh')
    })

    await expect(ensureSession()).resolves.toBe(true)
  })
})

describe('safeRedirectPath', () => {
  it.each([
    [undefined, '/'],
    ['/bookings?status=MENUNGGU', '/bookings?status=MENUNGGU'],
    ['https://evil.example/', '/'],
    ['//evil.example/path', '/'],
    ['/sign-in?redirect=/x', '/'],
    ['bookings', '/'],
  ])('%s -> %s', (input, expected) => {
    expect(safeRedirectPath(input)).toBe(expected)
  })
})
