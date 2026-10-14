import { create } from 'zustand'

// UI state only: whether the live stream is up. While it is down, the unread badge polls.
export const useNotificationStreamStore = create<{
  connected: boolean
  setConnected: (connected: boolean) => void
}>()((set) => ({
  connected: false,
  setConnected: (connected) => set({ connected }),
}))
