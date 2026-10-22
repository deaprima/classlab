import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  apiErrorBody,
  mockAdapter,
  sampleUser,
} from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { type UserRole } from '@/lib/api-types'
import { POLICIES } from '@/features/my-bookings/fixtures'
import { Verify } from '.'

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
  }
})
vi.mock('@/components/layout/page', () => ({
  Page: ({ title, children }: { title: string; children: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}))

const originalAdapter = api.defaults.adapter

// A 1x1 PNG, so the <img> has something to decode.
const PNG = Uint8Array.from(
  atob(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGNgYGD4DwABBAEAHVQ9dQAAAABJRU5ErkJggg=='
  ),
  (c) => c.charCodeAt(0)
)

const now = Date.now()
const CHECKOUTS = [
  {
    booking_id: 'c1',
    room_id: 'r1',
    room_code: 'R-402',
    room_name: 'Ruang 402',
    room_type: 'KELAS',
    checked_out_at: new Date(now - 3_600_000).toISOString(),
    condition: 'NORMAL',
    note: null,
    attachment_id: 'a1',
  },
  {
    booking_id: 'c2',
    room_id: 'r2',
    room_code: 'LAB-1',
    room_name: 'Lab Jaringan',
    room_type: 'LAB',
    checked_out_at: new Date(now - 7_200_000).toISOString(),
    condition: 'ADA_MASALAH',
    note: 'Kabel LAN putus',
    attachment_id: 'a2',
  },
]

function serve(requests: Record<string, unknown>[] = []) {
  api.defaults.adapter = mockAdapter((config) => {
    const url = String(config.url)
    if (url === '/room-types') return { status: 200, data: { data: POLICIES } }
    if (url === '/rooms') {
      return {
        status: 200,
        data: { data: [], meta: { page: 1, per_page: 100, total: 0 } },
      }
    }
    if (url === '/checkouts') {
      requests.push(config.params as Record<string, unknown>)
      return {
        status: 200,
        data: {
          data: CHECKOUTS,
          meta: { page: 1, per_page: 100, total: CHECKOUTS.length },
        },
      }
    }
    if (url === '/attachments/a2') {
      return {
        status: 410,
        data: apiErrorBody('GONE', 'Berkas sudah dihapus.'),
      }
    }
    if (url.startsWith('/attachments/')) {
      return { status: 200, data: new Blob([PNG], { type: 'image/png' }) }
    }
    return { status: 404 }
  })
  return requests
}

function renderAs(role: UserRole) {
  useAuthStore.getState().auth.setSession('t', { ...sampleUser, role })
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Verify />
    </QueryClientProvider>
  )
}

describe('Verifikasi Check-out', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.search = {}
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('lists today’s check-outs with photo, condition and time', async () => {
    const requests = serve()
    const screen = await renderAs('OB')

    await expect.element(screen.getByText('R-402')).toBeVisible()
    await expect.element(screen.getByText('Normal')).toBeVisible()
    await expect.element(screen.getByText('Ada masalah')).toBeVisible()
    await expect.element(screen.getByText('Kabel LAN putus')).toBeVisible()
    await expect
      .element(screen.getByRole('img', { name: 'Foto kondisi ruang R-402' }))
      .toBeVisible()
    // Today only, as a [from, to) window.
    const { from, to } = requests[0] as { from: string; to: string }
    expect(Date.parse(to) - Date.parse(from)).toBe(24 * 3_600_000)
  })

  it('says so when the retention policy already deleted a photo', async () => {
    serve()
    const screen = await renderAs('OB')

    await expect
      .element(screen.getByText('Foto sudah dihapus sesuai kebijakan retensi.'))
      .toBeVisible()
  })

  it('narrows to a room type', async () => {
    mocks.search = { type: 'LAB' }
    serve()
    const screen = await renderAs('OB')

    await expect.element(screen.getByText('LAB-1')).toBeVisible()
    expect(screen.getByText('R-402').elements()).toEqual([])
  })

  it('offers TU only the types it approves', async () => {
    serve()
    const screen = await renderAs('TU')

    await expect
      .element(screen.getByRole('button', { name: 'Ruang Kelas' }))
      .toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Laboratorium' }).elements()
    ).toEqual([])
  })

  it('puts the chosen type in the URL and opens a photo larger', async () => {
    serve()
    const screen = await renderAs('OB')

    await userEvent.click(screen.getByRole('button', { name: 'Laboratorium' }))
    const { search } = mocks.navigate.mock.calls[0][0] as {
      search: (prev: object) => object
    }
    expect(search({})).toEqual({ page: undefined, type: 'LAB' })

    await userEvent.click(
      screen.getByRole('button', { name: 'Perbesar foto R-402' })
    )
    await expect
      .element(screen.getByRole('dialog').getByText('Ruang 402'))
      .toBeVisible()
  })
})
