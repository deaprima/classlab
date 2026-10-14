import { useEffect } from 'react'
import { type QueryClient, useQueryClient } from '@tanstack/react-query'
import { useRouter } from '@tanstack/react-router'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { API_BASE_URL, refreshAccessToken } from '@/lib/api-client'
import { type AppNotification } from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'
import { createSseParser, type SseEvent } from '@/lib/sse'
import { notificationTarget } from './links'
import { useNotificationStreamStore } from './stream-store'

const MIN_BACKOFF_MS = 1_000
const MAX_BACKOFF_MS = 30_000

/**
 * Keeps one live notification stream open while signed in (decisions.md, Phase 4). It reads the SSE
 * response with fetch, because EventSource cannot send the bearer token. The server closes the
 * stream before the token expires, so reconnecting usually hits 401 once and refreshes. While the
 * stream is down, `connected` is false and the notification queries fall back to polling.
 */
export function useNotificationStream() {
  const queryClient = useQueryClient()
  const router = useRouter()
  const userId = useAuthStore((s) => s.auth.user?.id)

  useEffect(() => {
    if (!userId) return
    const controller = new AbortController()
    const { setConnected } = useNotificationStreamStore.getState()

    const onEvent = (e: SseEvent) => {
      if (e.event === 'ready') {
        setConnected(true)
        const { unread } = JSON.parse(e.data) as { unread: number }
        queryClient.setQueryData(queryKeys.notifications.unreadCount, unread)
        // Events are not replayed, so anything that arrived while disconnected is fetched again.
        queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all })
      } else if (e.event === 'notification') {
        const n = JSON.parse(e.data) as AppNotification
        invalidateFor(queryClient, n)
        announce(n)
      } else if (e.event === 'resync') {
        queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all })
      }
    }

    const announce = (n: AppNotification) => {
      const target = notificationTarget(
        n,
        useAuthStore.getState().auth.user?.role
      )
      toast(n.title, {
        description: n.body,
        action: target
          ? { label: 'Lihat', onClick: () => router.navigate(target) }
          : undefined,
      })
    }

    void runStream(controller.signal, onEvent, setConnected)
    return () => {
      controller.abort()
      setConnected(false)
    }
  }, [userId, queryClient, router])
}

/** A notification means something else changed too: refresh what it is about. */
function invalidateFor(queryClient: QueryClient, n: AppNotification) {
  queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all })
  if (n.ref_type === 'BOOKING') {
    queryClient.invalidateQueries({ queryKey: queryKeys.bookings.all })
    queryClient.invalidateQueries({ queryKey: queryKeys.availability.all })
    queryClient.invalidateQueries({ queryKey: queryKeys.rooms.all })
  } else if (n.ref_type === 'REPORT') {
    queryClient.invalidateQueries({ queryKey: queryKeys.reports.all })
  }
}

async function runStream(
  signal: AbortSignal,
  onEvent: (e: SseEvent) => void,
  setConnected: (connected: boolean) => void
) {
  let backoff = MIN_BACKOFF_MS
  let refreshedOnce = false

  while (!signal.aborted) {
    let waitMs = backoff
    try {
      const token = useAuthStore.getState().auth.accessToken
      if (!token) return
      const res = await fetch(`${API_BASE_URL}/notifications/stream`, {
        headers: {
          Authorization: `Bearer ${token}`,
          Accept: 'text/event-stream',
        },
        signal,
      })

      if (res.status === 401) {
        // Expected after the server ends a stream at token expiry. A second 401 in a row means the
        // session itself is gone; the axios interceptor handles the redirect on the next request.
        if (refreshedOnce) return
        refreshedOnce = true
        await refreshAccessToken()
        continue
      }
      if (res.status === 429) {
        // Too many tabs open; poll instead and try again later.
        waitMs = Number(res.headers.get('Retry-After') ?? 30) * 1000
        throw new Error('notification stream limit reached')
      }
      if (!res.ok || !res.body) {
        throw new Error(`notification stream answered ${res.status}`)
      }

      refreshedOnce = false
      let gotReady = false
      const parse = createSseParser((e) => {
        if (e.event === 'ready') {
          gotReady = true
          backoff = MIN_BACKOFF_MS
        }
        onEvent(e)
      })
      const reader = res.body.pipeThrough(new TextDecoderStream()).getReader()
      for (;;) {
        const { value, done } = await reader.read()
        if (done) break
        parse(value)
      }
      // A healthy stream the server ended (reconnect event, shutdown) is reopened right away; one
      // that closed before becoming ready backs off like an error.
      if (gotReady) waitMs = 0
      else backoff = Math.min(backoff * 2, MAX_BACKOFF_MS)
    } catch (error) {
      if (signal.aborted) return
      if (import.meta.env.DEV) {
        // eslint-disable-next-line no-console
        console.warn('notification stream:', error)
      }
      backoff = Math.min(backoff * 2, MAX_BACKOFF_MS)
    }
    setConnected(false)
    await sleep(waitMs, signal)
  }
}

function sleep(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve) => {
    if (ms <= 0 || signal.aborted) return resolve()
    const timer = setTimeout(resolve, ms)
    signal.addEventListener(
      'abort',
      () => {
        clearTimeout(timer)
        resolve()
      },
      { once: true }
    )
  })
}
