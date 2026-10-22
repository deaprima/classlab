import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { AdminSchedules } from '.'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    getRouteApi: () => ({
      useSearch: () => ({}),
      useNavigate: () => vi.fn(),
    }),
    Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
  }
})
vi.mock('@/components/layout/page', () => ({
  Page: ({
    title,
    actions,
    children,
  }: {
    title: string
    actions?: React.ReactNode
    children: React.ReactNode
  }) => (
    <div>
      <h1>{title}</h1>
      {actions}
      {children}
    </div>
  ),
}))

const originalAdapter = api.defaults.adapter

const ROOM = {
  id: 'r1',
  org_unit_id: 'u',
  code: 'R-301',
  name: 'Ruang 301',
  type: 'KELAS',
  capacity: 40,
  facilities: [],
  is_active: true,
  open_time: null,
  close_time: null,
}

function renderAs(role: 'TU' | 'KALAB') {
  useAuthStore.getState().auth.setSession('t', { ...sampleUser, role })
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <AdminSchedules />
    </QueryClientProvider>
  )
}

describe('Jadwal (its own page)', () => {
  beforeEach(() => {
    api.defaults.adapter = mockAdapter((config) => {
      const url = String(config.url)
      if (url === '/rooms') {
        return {
          status: 200,
          data: { data: [ROOM], meta: { page: 1, per_page: 100, total: 1 } },
        }
      }
      if (url === '/rooms/r1/schedule')
        return { status: 200, data: { data: [] } }
      return { status: 404 }
    })
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('offers adding a schedule and importing the semester CSV to TU', async () => {
    const screen = await renderAs('TU')

    await expect
      .element(screen.getByRole('heading', { name: 'Jadwal' }))
      .toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Tambah Jadwal' }))
      .toBeVisible()
    await expect.element(screen.getByText('Impor Jadwal CSV')).toBeVisible()
  })

  it('leaves the import to TU and Admin; Kalab only edits schedules', async () => {
    const screen = await renderAs('KALAB')

    await expect
      .element(screen.getByRole('button', { name: 'Tambah Jadwal' }))
      .toBeVisible()
    expect(screen.getByText('Impor Jadwal CSV').elements()).toEqual([])
  })
})
