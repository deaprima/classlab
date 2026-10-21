import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { Notifications } from '.'

const mocks = vi.hoisted(() => ({
  search: {} as Record<string, unknown>,
  navigate: vi.fn(),
}))

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    getRouteApi: () => ({
      useSearch: () => mocks.search,
      useNavigate: () => mocks.navigate,
    }),
    useNavigate: () => vi.fn(),
  }
})

vi.mock('@/components/layout/page', () => ({
  Page: ({
    title,
    description,
    children,
  }: {
    title: string
    description?: string
    children: React.ReactNode
  }) => (
    <div>
      <h1>{title}</h1>
      {description && <p>{description}</p>}
      {children}
    </div>
  ),
}))

const originalAdapter = api.defaults.adapter

const note = (n: number, read = false) => ({
  id: `n${n}`,
  type: 'BOOKING_APPROVED',
  title: `Judul ${n}`,
  body: `Isi ${n}`,
  ref_type: 'BOOKING',
  ref_id: `b${n}`,
  ...(read && { read_at: '2026-10-21T09:00:00+07:00' }),
  created_at: '2026-10-21T08:00:00+07:00',
})

function serve(list: ReturnType<typeof note>[], unread = 1) {
  const seen: Record<string, unknown>[] = []
  api.defaults.adapter = mockAdapter((config) => {
    if (config.url === '/notifications') {
      seen.push(config.params ?? {})
      return {
        status: 200,
        data: {
          data: list,
          meta: { page: 1, per_page: 20, total: list.length },
        },
      }
    }
    if (config.url === '/notifications/unread-count') {
      return { status: 200, data: { count: unread } }
    }
    return { status: 200, data: {} }
  })
  return seen
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Notifications />
    </QueryClientProvider>
  )
}

describe('Notifications', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.search = {}
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
  })

  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('lists notifications and marks the unread ones', async () => {
    serve([note(1), note(2, true)])
    const screen = await renderPage()
    await expect.element(screen.getByText('Judul 1')).toBeVisible()
    await expect.element(screen.getByText('Judul 2')).toBeVisible()
    await expect.element(screen.getByText('(belum dibaca)')).toBeInTheDocument()
  })

  it('shows an empty state', async () => {
    serve([], 0)
    const screen = await renderPage()
    await expect.element(screen.getByText('Belum ada notifikasi')).toBeVisible()
  })

  it('asks only for unread ones on the unread tab', async () => {
    mocks.search = { filter: 'unread' }
    const seen = serve([note(1)])
    await renderPage()
    await vi.waitFor(() =>
      expect(seen.some((q) => q.unread === true)).toBe(true)
    )
  })

  it('switches the tab through the URL', async () => {
    serve([note(1)])
    const screen = await renderPage()
    await userEvent.click(screen.getByRole('radio', { name: /Belum dibaca/ }))
    expect(mocks.navigate).toHaveBeenCalledWith({
      search: { filter: 'unread' },
    })
  })
})
