import { beforeEach, describe, expect, it, vi } from 'vitest'
import { type AuthUser } from './auth-store'

async function importAuthStore() {
  const { useAuthStore } = await import('./auth-store')
  return useAuthStore
}

const sampleUser: AuthUser = {
  id: '018f23a4-8f0b-789a-bcde-0123456789ab',
  org_unit_id: '018f23a4-8f0b-789a-bcde-000000000001',
  identifier: '220101001',
  name: 'Budi Santoso',
  email: 'budi@student.univ.ac.id',
  role: 'MAHASISWA',
  is_active: true,
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

    useAuthStore.getState().auth.setUser(sampleUser)
    useAuthStore.getState().auth.setAccessToken('mock_jwt_token')

    expect(useAuthStore.getState().auth.user).toEqual(sampleUser)
    expect(useAuthStore.getState().auth.accessToken).toBe('mock_jwt_token')
    expect(useAuthStore.getState().auth.isAuthenticated).toBe(true)
  })

  it('resets authentication state cleanly', async () => {
    const useAuthStore = await importAuthStore()

    useAuthStore.getState().auth.setUser(sampleUser)
    useAuthStore.getState().auth.setAccessToken('mock_jwt_token')
    useAuthStore.getState().auth.reset()

    expect(useAuthStore.getState().auth.user).toBeNull()
    expect(useAuthStore.getState().auth.accessToken).toBeNull()
    expect(useAuthStore.getState().auth.isAuthenticated).toBe(false)
  })
})
