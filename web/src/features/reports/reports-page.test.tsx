import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { Reports } from '.'

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

// The detail sheet and the form have their own tests.
vi.mock('./components/report-detail-sheet', () => ({
  ReportDetailSheet: () => null,
}))
vi.mock('./components/report-form-dialog', () => ({
  ReportFormDialog: () => null,
}))

const originalAdapter = api.defaults.adapter

const ROOM = {
  id: 'room-1',
  org_unit_id: sampleUser.org_unit_id,
  code: 'LAB-1',
  name: 'Lab Jaringan',
  type: 'LAB',
  capacity: 30,
  facilities: [],
  is_active: true,
}

const report = (n: number, over: Record<string, unknown> = {}) => ({
  id: `r${n}`,
  room_id: ROOM.id,
  room: ROOM,
  reporter_id: sampleUser.id,
  reporter: { id: sampleUser.id, identifier: '2021101001', name: 'Andi' },
  description: `Kerusakan ${n}`,
  status: 'BARU',
  created_at: '2026-10-14T09:00:00+07:00',
  events: [],
  ...over,
})

function serve(list: ReturnType<typeof report>[], total = list.length) {
  const seen: Record<string, unknown>[] = []
  api.defaults.adapter = mockAdapter((config) => {
    if (config.url === '/rooms') {
      return {
        status: 200,
        data: { data: [ROOM], meta: { page: 1, per_page: 100, total: 1 } },
      }
    }
    if (config.url === '/reports') {
      seen.push(config.params ?? {})
      return {
        status: 200,
        data: { data: list, meta: { page: 1, per_page: 10, total } },
      }
    }
    return { status: 404 }
  })
  return seen
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Reports />
    </QueryClientProvider>
  )
}

describe('Reports (Laporan Kerusakan)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.search = {}
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
  })

  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('lists the reports with their status', async () => {
    serve([report(1), report(2, { status: 'DIPROSES' })])
    const screen = await renderPage()
    await expect.element(screen.getByText('Kerusakan 1').first()).toBeVisible()
    await expect.element(screen.getByText('Kerusakan 2').first()).toBeVisible()
  })

  it('shows an empty state when there are no reports', async () => {
    serve([])
    const screen = await renderPage()
    await expect
      .element(screen.getByText('Belum ada laporan kerusakan'))
      .toBeVisible()
  })

  it('asks the API for the status of the URL and offers to clear it', async () => {
    mocks.search = { status: 'DIPROSES' }
    const seen = serve([])
    const screen = await renderPage()
    await expect
      .element(screen.getByText('Tidak ada laporan yang cocok'))
      .toBeVisible()
    expect(seen.some((q) => q.status === 'DIPROSES')).toBe(true)
    await userEvent.click(screen.getByRole('button', { name: 'Hapus filter' }))
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({ status: 'DIPROSES' })).toMatchObject({
      status: undefined,
      page: undefined,
    })
  })

  it('changes the status filter through the URL and returns to page 1', async () => {
    serve([report(1)])
    const screen = await renderPage()
    await userEvent.click(screen.getByRole('radio', { name: 'Selesai' }))
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({ page: 3 })).toMatchObject({
      status: 'SELESAI',
      page: undefined,
    })
  })

  it('pages through a longer list', async () => {
    serve([report(1)], 12)
    const screen = await renderPage()
    await expect.element(screen.getByText('1–10 dari 12')).toBeVisible()
    await userEvent.click(
      screen.getByRole('button', { name: 'Halaman berikutnya' })
    )
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({})).toMatchObject({ page: 2 })
  })
})
