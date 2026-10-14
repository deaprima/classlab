import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderHook } from 'vitest-browser-react'
import { useAuthStore } from '@/stores/auth-store'
import { authClient } from '@/lib/api-client'
import { queryKeys } from '@/lib/query-keys'
import { useNotificationStreamStore } from './stream-store'
import { useNotificationStream } from './use-notification-stream'

const toast = vi.hoisted(() => vi.fn())
vi.mock('sonner', () => ({ toast }))

const navigate = vi.fn()
vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return { ...actual, useRouter: () => ({ navigate }) }
})

const originalAuthAdapter = authClient.defaults.adapter

/** An SSE response that sends `text` and then stays open, like the real stream. */
function sseResponse(text: string) {
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(new TextEncoder().encode(text))
    },
  })
  return new Response(body, {
    status: 200,
    headers: { 'Content-Type': 'text/event-stream' },
  })
}

const notification = {
  id: 'n1',
  type: 'BOOKING_APPROVED',
  title: 'Pengajuan disetujui',
  body: 'LAB-2, Sel 27 Okt 13:00–15:00 disetujui.',
  ref_type: 'BOOKING',
  ref_id: '12b750c3-627d-45be-b3d3-cb7cc41845d1',
  created_at: '2026-10-14T16:37:00+07:00',
}

async function setup() {
  const queryClient = new QueryClient()
  // Seed queries so invalidation can be observed.
  queryClient.setQueryData(queryKeys.bookings.list({}), { data: [] })
  queryClient.setQueryData(queryKeys.notifications.list({}), { data: [] })
  const hook = await renderHook(() => useNotificationStream(), {
    wrapper: ({ children }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
  })
  return { queryClient, hook }
}

describe('useNotificationStream', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore.getState().auth.setSession('token-1', { ...sampleUser })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    authClient.defaults.adapter = originalAuthAdapter
    useAuthStore.getState().auth.reset()
  })

  it('opens the stream with the bearer token and takes the unread count from ready', async () => {
    const fetchMock = vi.fn(async () =>
      sseResponse('event: ready\ndata: {"unread":3}\n\n')
    )
    vi.stubGlobal('fetch', fetchMock)
    const { queryClient, hook } = await setup()

    await vi.waitFor(() =>
      expect(
        queryClient.getQueryData(queryKeys.notifications.unreadCount)
      ).toBe(3)
    )
    expect(useNotificationStreamStore.getState().connected).toBe(true)
    const [url, init] = fetchMock.mock.calls[0] as unknown as [
      string,
      RequestInit,
    ]
    expect(url).toBe('/api/v1/notifications/stream')
    expect((init.headers as Record<string, string>).Authorization).toBe(
      'Bearer token-1'
    )

    await hook.unmount()
    expect(useNotificationStreamStore.getState().connected).toBe(false)
  })

  it('announces a notification and refreshes what it is about', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        sseResponse(
          'event: ready\ndata: {"unread":0}\n\n' +
            `id: n1\nevent: notification\ndata: ${JSON.stringify(notification)}\n\n`
        )
      )
    )
    const { queryClient, hook } = await setup()

    await vi.waitFor(() => expect(toast).toHaveBeenCalled())
    expect(toast).toHaveBeenCalledWith('Pengajuan disetujui', {
      description: notification.body,
      action: { label: 'Lihat', onClick: expect.any(Function) },
    })
    expect(
      queryClient.getQueryState(queryKeys.bookings.list({}))?.isInvalidated
    ).toBe(true)

    toast.mock.calls[0][1].action.onClick()
    expect(navigate).toHaveBeenCalledWith({
      to: '/bookings',
      search: { booking: notification.ref_id },
    })
    await hook.unmount()
  })

  it('refreshes the token once when the stream answers 401, then reconnects', async () => {
    authClient.defaults.adapter = mockAdapter(() => ({
      status: 200,
      data: { access_token: 'token-2', expires_in: 900 },
    }))
    const fetchMock = vi.fn(async (_url: string, init: RequestInit) => {
      const auth = (init.headers as Record<string, string>).Authorization
      return auth === 'Bearer token-2'
        ? sseResponse('event: ready\ndata: {"unread":1}\n\n')
        : new Response('{}', { status: 401 })
    })
    vi.stubGlobal('fetch', fetchMock)
    const { hook } = await setup()

    await vi.waitFor(() =>
      expect(useNotificationStreamStore.getState().connected).toBe(true)
    )
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(useAuthStore.getState().auth.accessToken).toBe('token-2')
    await hook.unmount()
  })
})
