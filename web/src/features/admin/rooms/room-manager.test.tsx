import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { type Room } from '@/lib/api-types'
import { POLICIES } from '@/features/my-bookings/fixtures'
import { RoomManager } from './room-manager'

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
    Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
  }
})

vi.mock('@/hooks/use-media-query', () => ({ useMediaQuery: () => true }))
vi.mock('@/hooks/use-mobile', () => ({ useIsMobile: () => false }))
vi.mock('@/components/layout/page', () => ({
  Page: ({ title, children }: { title: string; children: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}))

const originalAdapter = api.defaults.adapter

const room = (over: Partial<Room>): Room => ({
  id: 'r1',
  org_unit_id: sampleUser.org_unit_id,
  code: 'R-301',
  name: 'Ruang 301',
  type: 'KELAS',
  capacity: 40,
  facilities: ['Proyektor'],
  is_active: true,
  open_time: null,
  close_time: null,
  ...over,
})
const ROOMS = [
  room({}),
  room({
    id: 'r2',
    code: 'R-402',
    name: 'Ruang 402',
    open_time: '07:00',
    close_time: '22:00',
  }),
  room({ id: 'r3', code: 'R-501', name: 'Ruang 501', is_active: false }),
]

function serve() {
  api.defaults.adapter = mockAdapter((config) => {
    const url = String(config.url)
    if (url === '/room-types') return { status: 200, data: { data: POLICIES } }
    if (url === '/rooms') {
      const per = Number((config.params as { per_page?: number }).per_page)
      return {
        status: 200,
        data: {
          data: per === 1 ? ROOMS.slice(0, 1) : ROOMS,
          meta: { page: 1, per_page: per, total: ROOMS.length },
        },
      }
    }
    if (url === '/rooms/r1/qr') {
      return {
        status: 200,
        data: {
          url: 'https://classlab.test/check-in?room=R-301&t=abc',
          qr_token: 'abc',
          rotated_at: '2026-10-22T03:00:00Z',
        },
      }
    }
    return { status: 404 }
  })
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <RoomManager />
    </QueryClientProvider>
  )
}

describe('RoomManager', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.search = {}
    useAuthStore.getState().auth.setSession('t', {
      ...sampleUser,
      role: 'TU',
    })
    serve()
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('lists rooms with their effective hours and status', async () => {
    const screen = await renderPage()

    await expect.element(screen.getByText('R-402')).toBeInTheDocument()
    // R-301 follows the type default; R-402 has its own hours.
    await expect.element(screen.getByText('07:00–21:00').first()).toBeVisible()
    await expect.element(screen.getByText('07:00–22:00 (khusus)')).toBeVisible()
    await expect
      .element(screen.getByText('Nonaktif', { exact: true }))
      .toBeVisible()
  })

  it('edits the first room beside the list and opens its QR from the card', async () => {
    const screen = await renderPage()

    await expect
      .element(screen.getByRole('complementary', { name: 'Edit ruang' }))
      .toBeInTheDocument()
    await expect.element(screen.getByLabelText('Kode')).toHaveValue('R-301')

    await userEvent.click(
      screen.getByRole('button', { name: /Cetak QR \/ Ganti Token/ })
    )
    await expect
      .element(screen.getByRole('img', { name: /QR check-in ruang R-301/ }))
      .toBeInTheDocument()
  })

  it('selects a room from the list through the URL', async () => {
    const screen = await renderPage()

    await userEvent.click(screen.getByText('R-402'))

    expect(mocks.navigate).toHaveBeenCalledTimes(1)
    const { search } = mocks.navigate.mock.calls[0][0] as {
      search: (prev: object) => object
    }
    expect(search({})).toEqual({ room: 'r2' })
  })
})
