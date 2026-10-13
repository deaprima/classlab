import { create } from 'zustand'

export type UserRole = 'MAHASISWA' | 'TU_KALAB' | 'OB' | 'KAPRODI'

export interface AuthUser {
  id: string
  org_unit_id: string
  identifier: string
  name: string
  email: string
  role: UserRole
  is_active: boolean
}

interface AuthState {
  auth: {
    user: AuthUser | null
    accessToken: string | null
    isAuthenticated: boolean
    setUser: (user: AuthUser | null) => void
    setAccessToken: (accessToken: string | null) => void
    reset: () => void
  }
}

// In-memory auth store: access token is NOT stored in localStorage or JS cookies
export const useAuthStore = create<AuthState>()((set) => ({
  auth: {
    user: null,
    accessToken: null,
    isAuthenticated: false,
    setUser: (user) =>
      set((state) => ({
        auth: {
          ...state.auth,
          user,
          isAuthenticated: !!user && !!state.auth.accessToken,
        },
      })),
    setAccessToken: (accessToken) =>
      set((state) => ({
        auth: {
          ...state.auth,
          accessToken,
          isAuthenticated: !!state.auth.user && !!accessToken,
        },
      })),
    reset: () =>
      set((state) => ({
        auth: {
          ...state.auth,
          user: null,
          accessToken: null,
          isAuthenticated: false,
        },
      })),
  },
}))
