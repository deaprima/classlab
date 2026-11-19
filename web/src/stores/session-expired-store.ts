import { create } from 'zustand'

interface SessionExpiredState {
  open: boolean
  /** Page the user was on, so signing in again returns there. */
  redirect: string | undefined
  show: (redirect: string) => void
  close: () => void
}

// UI state only: whether the "Sesi berakhir" dialog is showing. The session itself lives in the auth store.
export const useSessionExpiredStore = create<SessionExpiredState>()((set) => ({
  open: false,
  redirect: undefined,
  // Several requests can fail together; the first one fixes the page to return to.
  show: (redirect) =>
    set((state) => (state.open ? state : { open: true, redirect })),
  close: () => set({ open: false, redirect: undefined }),
}))
