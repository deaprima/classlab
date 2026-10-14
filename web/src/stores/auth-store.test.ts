import { sampleUser } from '@/test-utils/mock-adapter'
import { beforeEach, describe, expect, it, vi } from 'vitest'

async function importAuthStore() {
  const { useAuthStore } = await import('./auth-store')
  return useAuthStore
}

describe('useAuthStore (in-memory)', () => {
  beforeEach(() => {
    vi.resetModules()
  })

  it('starts unauthenticated with null user and null token', async () => {
    const useAuthStore = await importAuthStore()

    expect(useAuthStore.getState().auth.accessToken).toBeNull()
    expect(useAuthStore.getState().auth.user).toBeNull()
    expect(useAuthStore.getState().auth.isAuthenticated).toBe(false)
  })

  it('sets user and access token and updates isAuthenticated', async () => {
    const useAuthStore = await importAuthStore()

    useAuthStore.getState().auth.setUser({ ...sampleUser })
    expect(useAuthStore.getState().auth.isAuthenticated).toBe(false)
    useAuthStore.getState().auth.setAccessToken('mock_jwt_token')

    expect(useAuthStore.getState().auth.user).toEqual(sampleUser)
    expect(useAuthStore.getState().auth.accessToken).toBe('mock_jwt_token')
    expect(useAuthStore.getState().auth.isAuthenticated).toBe(true)
  })

  it('sets a whole session at once', async () => {
    const useAuthStore = await importAuthStore()

    useAuthStore.getState().auth.setSession('jwt', { ...sampleUser })

    expect(useAuthStore.getState().auth.isAuthenticated).toBe(true)
    expect(useAuthStore.getState().auth.accessToken).toBe('jwt')
  })

  it('resets authentication state cleanly', async () => {
    const useAuthStore = await importAuthStore()

    useAuthStore.getState().auth.setSession('mock_jwt_token', { ...sampleUser })
    useAuthStore.getState().auth.reset()

    expect(useAuthStore.getState().auth.user).toBeNull()
    expect(useAuthStore.getState().auth.accessToken).toBeNull()
    expect(useAuthStore.getState().auth.isAuthenticated).toBe(false)
  })

  it('never writes the token to web storage', async () => {
    const useAuthStore = await importAuthStore()
    const before = {
      local: localStorage.length,
      session: sessionStorage.length,
    }

    useAuthStore.getState().auth.setSession('secret-token', { ...sampleUser })

    expect(localStorage.length).toBe(before.local)
    expect(sessionStorage.length).toBe(before.session)
    expect(document.cookie).not.toContain('secret-token')
  })
})
