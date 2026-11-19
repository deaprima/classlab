import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { Students } from '.'

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
vi.mock('@/hooks/use-mobile', () => ({ useIsMobile: () => false }))
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
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
vi.mock('sonner', () => ({ toast }))

const originalAdapter = api.defaults.adapter

const student = (
  id: string,
  name: string,
  over: Record<string, unknown> = {}
) => ({
  ...sampleUser,
  id,
  name,
  identifier: `2023${id}`,
  email: `${id}@mhs.test`,
  role: 'MAHASISWA',
  is_active: true,
  ...over,
})
const STUDENTS = [
  student('01', 'Dea Prima'),
  student('02', 'Putri A.', { is_active: false }),
]

type Call = {
  method: string
  url: string
  params?: unknown
  body?: Record<string, unknown>
}

function serve() {
  const calls: Call[] = []
  api.defaults.adapter = mockAdapter((config) => {
    const url = String(config.url)
    const method = String(config.method)
    calls.push({
      method,
      url,
      params: config.params,
      body: config.data ? JSON.parse(String(config.data)) : undefined,
    })
    if (url === '/users' && method === 'get') {
      return {
        status: 200,
        data: {
          data: STUDENTS,
          meta: { page: 1, per_page: 20, total: 112 },
        },
      }
    }
    if (url === '/users' && method === 'post') {
      return { status: 201, data: student('99', 'Rina Hartono') }
    }
    if (url.startsWith('/users/') && method === 'patch') {
      return { status: 200, data: STUDENTS[0] }
    }
    return { status: 404 }
  })
  return calls
}

function renderPage() {
  useAuthStore.getState().auth.setSession('t', { ...sampleUser, role: 'TU' })
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Students />
    </QueryClientProvider>
  )
}

describe('Mahasiswa (TU)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.search = {}
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('lists students only, with their status and the total', async () => {
    const calls = serve()
    const screen = await renderPage()

    await expect.element(screen.getByText('Dea Prima')).toBeVisible()
    await expect
      .element(screen.getByRole('table').getByText('Nonaktif'))
      .toBeVisible()
    await expect.element(screen.getByText('112', { exact: true })).toBeVisible()
    expect(
      calls.every(
        (c) => (c.params as { role?: string } | undefined)?.role === 'MAHASISWA'
      )
    ).toBe(true)
  })

  it('adds a student without a role to choose', async () => {
    const calls = serve()
    const screen = await renderPage()

    await userEvent.click(
      screen.getByRole('button', { name: 'Tambah Mahasiswa' })
    )
    await expect.element(screen.getByRole('dialog')).toBeVisible()
    expect(screen.getByLabelText('Peran').elements()).toEqual([])

    await userEvent.fill(screen.getByLabelText('NIM / NIP'), '2321001099')
    await userEvent.fill(screen.getByLabelText('Nama'), 'Rina Hartono')
    await userEvent.fill(
      screen.getByLabelText('Kata sandi sementara'),
      'sementara-123'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Buat akun' }))

    await vi.waitFor(() =>
      expect(calls.some((c) => c.method === 'post')).toBe(true)
    )
    const post = calls.find((c) => c.method === 'post')
    expect(post?.body).toMatchObject({
      identifier: '2321001099',
      name: 'Rina Hartono',
      role: 'MAHASISWA',
    })
  })

  it('edits name, email and status but never sends a role', async () => {
    const calls = serve()
    const screen = await renderPage()

    await userEvent.click(
      screen.getByRole('button', { name: 'Ubah Dea Prima' })
    )
    await userEvent.fill(screen.getByLabelText('Nama'), 'Dea Prima Sari')
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))

    await vi.waitFor(() =>
      expect(calls.some((c) => c.method === 'patch')).toBe(true)
    )
    const patch = calls.find((c) => c.method === 'patch')
    expect(patch?.body).toMatchObject({ name: 'Dea Prima Sari' })
    expect(patch?.body).not.toHaveProperty('role')
  })

  it('narrows by status through the URL', async () => {
    serve()
    const screen = await renderPage()

    await userEvent.click(screen.getByRole('radio', { name: 'Nonaktif' }))

    const { search } = mocks.navigate.mock.calls[0][0] as {
      search: (prev: object) => object
    }
    expect(search({})).toEqual({ page: undefined, status: 'inactive' })
  })
})
