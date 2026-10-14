import { AxiosError, type AxiosAdapter } from 'axios'
import {
  apiErrorBody,
  mockAdapter,
  sampleUser,
} from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth-store'
import { api, authClient, setSessionExpiredHandler } from './api-client'

const originalApiAdapter = api.defaults.adapter
const originalAuthAdapter = authClient.defaults.adapter

function useAdapters(apiAdapter: AxiosAdapter, authAdapter: AxiosAdapter) {
  api.defaults.adapter = apiAdapter
  authClient.defaults.adapter = authAdapter
}

const unauthorized = {
  status: 401,
  data: apiErrorBody('UNAUTHENTICATED', 'Sesi tidak valid.'),
}

beforeEach(() => {
  useAuthStore.getState().auth.setSession('old-token', { ...sampleUser })
})

afterEach(() => {
  api.defaults.adapter = originalApiAdapter
  authClient.defaults.adapter = originalAuthAdapter
  setSessionExpiredHandler(() => {})
  useAuthStore.getState().auth.reset()
})

describe('api client', () => {
  it('sends the access token as a bearer header', async () => {
    const seen = vi.fn()
    useAdapters(
      mockAdapter((config) => {
        seen(config.headers.get('Authorization'))
        return { status: 200, data: {} }
      }),
      mockAdapter(() => ({ status: 500 }))
    )

    await api.get('/rooms')

    expect(seen).toHaveBeenCalledWith('Bearer old-token')
  })

  it('refreshes once on 401 and retries the request with the new token', async () => {
    const refresh = vi.fn(() => ({
      status: 200,
      data: { access_token: 'new-token', expires_in: 900 },
    }))
    useAdapters(
      mockAdapter((config) =>
        config.headers.get('Authorization') === 'Bearer new-token'
          ? { status: 200, data: { ok: true } }
          : unauthorized
      ),
      mockAdapter(refresh)
    )

    const { data } = await api.get('/bookings')

    expect(data).toEqual({ ok: true })
    expect(refresh).toHaveBeenCalledOnce()
    expect(useAuthStore.getState().auth.accessToken).toBe('new-token')
  })

  it('shares one refresh between requests that fail at the same time', async () => {
    let release: () => void = () => {}
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    const refresh = vi.fn(async () => {
      await gate
      return {
        status: 200,
        data: { access_token: 'new-token', expires_in: 900 },
      }
    })
    useAdapters(
      mockAdapter((config) =>
        config.headers.get('Authorization') === 'Bearer new-token'
          ? { status: 200, data: config.url }
          : unauthorized
      ),
      mockAdapter(refresh)
    )

    const pending = Promise.all([
      api.get('/rooms'),
      api.get('/bookings'),
      api.get('/notifications'),
    ])
    await vi.waitFor(() => expect(refresh).toHaveBeenCalled())
    release()
    const results = await pending

    expect(refresh).toHaveBeenCalledOnce()
    expect(results.map((r) => r.data)).toEqual([
      '/rooms',
      '/bookings',
      '/notifications',
    ])
  })

  it('reports an expired session when the refresh fails', async () => {
    const expired = vi.fn()
    setSessionExpiredHandler(expired)
    useAdapters(
      mockAdapter(() => unauthorized),
      mockAdapter(() => unauthorized)
    )

    const error = await api.get('/bookings').catch((e: unknown) => e)

    expect(error).toBeInstanceOf(AxiosError)
    expect((error as AxiosError).response?.status).toBe(401)
    expect(expired).toHaveBeenCalledOnce()
  })

  it('does not refresh when login itself answers 401', async () => {
    const refresh = vi.fn(() => ({ status: 200, data: {} }))
    useAdapters(
      mockAdapter(() => unauthorized),
      mockAdapter(refresh)
    )

    await expect(
      api.post('/auth/login', { identifier: 'x', password: 'y' })
    ).rejects.toBeInstanceOf(AxiosError)
    expect(refresh).not.toHaveBeenCalled()
  })

  it('passes non-401 errors through untouched', async () => {
    const refresh = vi.fn(() => ({ status: 200, data: {} }))
    useAdapters(
      mockAdapter(() => ({
        status: 409,
        data: apiErrorBody('BOOKING_CONFLICT', 'Bentrok.'),
      })),
      mockAdapter(refresh)
    )

    const error = await api.post('/bookings', {}).catch((e: unknown) => e)

    expect((error as AxiosError).response?.status).toBe(409)
    expect(refresh).not.toHaveBeenCalled()
  })
})
