import { create } from 'zustand'
import { type User, type UserRole } from '@/lib/api-types'

export type AuthUser = User
export type { UserRole }

interface AuthState {
  auth: {
    user: AuthUser | null
    accessToken: string | null
    isAuthenticated: boolean
    setSession: (accessToken: string, user: AuthUser) => void
    setUser: (user: AuthUser | null) => void
    setAccessToken: (accessToken: string | null) => void
    reset: () => void
  }
}

// Session state lives in memory only. The access token never touches localStorage or a JS cookie
// (agent.md 6); after a reload the session is restored from the httpOnly refresh cookie.
export const useAuthStore = create<AuthState>()((set) => ({
  auth: {
    user: null,
    accessToken: null,
    isAuthenticated: false,
    setSession: (accessToken, user) =>
      set((state) => ({
        auth: { ...state.auth, accessToken, user, isAuthenticated: true },
      })),
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
