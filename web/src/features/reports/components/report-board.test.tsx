import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { POLICIES } from '@/features/my-bookings/fixtures'
import { Reports } from '..'

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
// The drawer and the form have their own tests.
vi.mock('./report-detail-sheet', () => ({ ReportDetailSheet: () => null }))
vi.mock('./report-form-dialog', () => ({ ReportFormDialog: () => null }))

const originalAdapter = api.defaults.adapter

const room = (code: string, type: string) => ({
  id: code,
  org_unit_id: 'u',
  code,
  name: code,
  type,
  capacity: 30,
  facilities: [],
  is_active: true,
  open_time: null,
  close_time: null,
})
const report = (
  id: string,
  status: string,
  code: string,
  type: string,
  description: string
) => ({
  id,
  room_id: code,
  room: room(code, type),
  reporter_id: 'p',
  reporter: { id: 'p', identifier: '1', name: 'Dea Prima' },
  description,
  attachment_id: null,
  status,
  created_at: new Date(Date.now() - 2 * 3_600_000).toISOString(),
})

const REPORTS = [
  report('r1', 'BARU', 'R-402', 'KELAS', 'Proyektor tidak menyala'),
  report('r2', 'BARU', 'LAB-1', 'LAB', 'Kabel LAN putus'),
  report('r3', 'DIPROSES', 'R-501', 'KELAS', 'Kursi patah'),
  report('r4', 'SELESAI', 'R-401', 'KELAS', 'Stop kontak longgar'),
]

function serve(requests: Record<string, unknown>[] = []) {
  api.defaults.adapter = mockAdapter((config) => {
    const url = String(config.url)
    if (url === '/room-types') return { status: 200, data: { data: POLICIES } }
    if (url === '/reports') {
      const q = (config.params ?? {}) as { status?: string }
      requests.push(q)
      const rows = REPORTS.filter((r) => r.status === q.status)
      return {
        status: 200,
        data: {
          data: rows,
          meta: { page: 1, per_page: 50, total: rows.length },
        },
      }
    }
    return { status: 404 }
  })
  return requests
}

function renderAs(role: 'TU' | 'KALAB' | 'MAHASISWA') {
  useAuthStore.getState().auth.setSession('t', { ...sampleUser, role })
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Reports />
    </QueryClientProvider>
  )
}

describe('Laporan Kerusakan board (managers)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.search = {}
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('puts every report in the column of its status, with the column count', async () => {
    const requests = serve()
    const screen = await renderAs('TU')

    await expect.element(screen.getByText('Baru · 2')).toBeVisible()
    await expect.element(screen.getByText('Diterima · 0')).toBeVisible()
    await expect.element(screen.getByText('Diproses · 1')).toBeVisible()
    await expect.element(screen.getByText('Selesai · 1')).toBeVisible()
    const baru = screen.getByRole('region', { name: 'Kolom Baru' })
    await expect
      .element(baru.getByText('Proyektor tidak menyala'))
      .toBeVisible()
    await expect.element(baru.getByText(/Dea Prima/).first()).toBeVisible()
    expect(new Set(requests.map((r) => r.status))).toEqual(
      new Set(['BARU', 'DITERIMA', 'DIPROSES', 'SELESAI'])
    )
  })

  it('opens the drawer of a card through the URL', async () => {
    serve()
    const screen = await renderAs('TU')

    await userEvent.click(screen.getByRole('button', { name: /Kursi patah/ }))

    const { search } = mocks.navigate.mock.calls[0][0] as {
      search: (prev: object) => object
    }
    expect(search({})).toEqual({ report: 'r3' })
  })

  it('narrows the board to a room type and recounts the columns', async () => {
    mocks.search = { type: 'LAB' }
    serve()
    const screen = await renderAs('KALAB')

    await expect.element(screen.getByText('Baru · 1')).toBeVisible()
    await expect.element(screen.getByText('Kabel LAN putus')).toBeVisible()
    expect(screen.getByText('Proyektor tidak menyala').elements()).toEqual([])
    await expect.element(screen.getByText('Diproses · 0')).toBeVisible()
  })

  it('offers TU only the types it approves, and puts the choice in the URL', async () => {
    serve()
    const screen = await renderAs('TU')

    expect(
      screen.getByRole('button', { name: 'Laboratorium' }).elements()
    ).toEqual([])
    await userEvent.click(screen.getByRole('button', { name: 'Ruang Kelas' }))
    const { search } = mocks.navigate.mock.calls[0][0] as {
      search: (prev: object) => object
    }
    expect(search({})).toEqual({ type: 'KELAS' })
  })

  it('keeps the reporter on their own list, without the board', async () => {
    api.defaults.adapter = mockAdapter((config) =>
      String(config.url) === '/reports'
        ? {
            status: 200,
            data: { data: [], meta: { page: 1, per_page: 10, total: 0 } },
          }
        : {
            status: 200,
            data: { data: [], meta: { page: 1, per_page: 100, total: 0 } },
          }
    )
    const screen = await renderAs('MAHASISWA')

    await expect
      .element(screen.getByText('Belum ada laporan kerusakan'))
      .toBeVisible()
    expect(screen.getByText('Baru · 0').elements()).toEqual([])
  })
})
