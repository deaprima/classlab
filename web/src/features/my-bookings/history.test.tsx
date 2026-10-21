import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { type Booking } from '@/lib/api-types'
import { makeBooking, POLICIES } from './fixtures'
import { History } from './history-page'

const mocks = vi.hoisted(() => ({
  search: {} as Record<string, unknown>,
  navigate: vi.fn(),
  wide: false,
}))

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    getRouteApi: () => ({
      useSearch: () => mocks.search,
      useNavigate: () => mocks.navigate,
    }),
    Link: ({
      to,
      children,
      ...props
    }: {
      to: string
      children?: React.ReactNode
    }) => (
      <a href={to} {...props}>
        {children}
      </a>
    ),
  }
})

vi.mock('@/hooks/use-media-query', () => ({ useMediaQuery: () => mocks.wide }))

// A table on every width, so rows can be counted.
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

// 2 hours each: three finished and two refused ones, out of five.
const finished = (n: number) =>
  makeBooking({
    id: `f${n}`,
    status: 'SELESAI',
    purpose: `Selesai ${n}`,
    start_at: `2026-09-1${n}T10:00:00+07:00`,
    end_at: `2026-09-1${n}T12:00:00+07:00`,
  })
const refused = (n: number) =>
  makeBooking({ id: `r${n}`, status: 'DITOLAK', purpose: `Ditolak ${n}` })
const ALL: Booking[] = [
  finished(1),
  finished(2),
  finished(3),
  refused(1),
  refused(2),
]

type Seen = {
  status?: string
  from?: string
  to?: string
  page?: string
  per_page?: string
  type?: string
}

function serve(all: Booking[] = ALL) {
  const seen: Seen[] = []
  api.defaults.adapter = mockAdapter((config) => {
    if (config.url === '/room-types') {
      return { status: 200, data: { data: POLICIES } }
    }
    if (config.url === '/bookings') {
      const q = (config.params ?? {}) as Seen
      seen.push(q)
      let list = all
      if (q.status) list = list.filter((b) => b.status === q.status)
      const per = Number(q.per_page ?? 20)
      const page = Number(q.page ?? 1)
      return {
        status: 200,
        data: {
          data: list.slice((page - 1) * per, page * per),
          meta: { page, per_page: per, total: list.length },
        },
      }
    }
    return { status: 404 }
  })
  return seen
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <History />
    </QueryClientProvider>
  )
}

describe('History (Riwayat)', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-10-21T10:00:00+07:00'))
    vi.clearAllMocks()
    mocks.wide = false
    mocks.search = {}
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
  })

  afterEach(() => {
    vi.useRealTimers()
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('shows the totals: bookings, hours used and refusals', async () => {
    serve()
    const screen = await renderPage()
    const card = (title: string) =>
      screen
        .getByText(title, { exact: true })
        .first()
        .element()
        .closest('[data-slot="stat-card"]')!
    await vi.waitFor(() =>
      expect(card('Total Booking').textContent).toContain('5')
    )
    await vi.waitFor(() =>
      expect(card('Jam Terpakai').textContent).toContain('6 jam')
    )
    await vi.waitFor(() => expect(card('Ditolak').textContent).toContain('2'))
  })

  it('counts the tabs from the API totals', async () => {
    serve()
    const screen = await renderPage()
    await expect
      .element(screen.getByRole('radio', { name: /Selesai/ }))
      .toHaveTextContent('3')
    await expect
      .element(screen.getByRole('radio', { name: /Ditolak/ }))
      .toHaveTextContent('2')
    await expect
      .element(screen.getByRole('radio', { name: /Semua/ }))
      .toHaveTextContent('5')
  })

  it('lists the bookings with their status', async () => {
    serve()
    const screen = await renderPage()
    await expect.element(screen.getByText('Selesai 1')).toBeVisible()
    await expect.element(screen.getByText('Ditolak 2')).toBeVisible()
    await expect
      .element(screen.getByRole('columnheader', { name: 'Detail' }))
      .not.toBeInTheDocument()
  })

  it('shows the detail beside the list on a wide screen', async () => {
    mocks.wide = true
    serve()
    const screen = await renderPage()
    await expect
      .element(
        screen.getByRole('complementary', { name: 'Detail Riwayat Pengajuan' })
      )
      .toBeVisible()
    await userEvent.click(screen.getByText('Ditolak 1'))
    expect(mocks.navigate).toHaveBeenCalled()
  })

  it('opens the detail in a popup when a row is clicked', async () => {
    serve()
    const screen = await renderPage()
    await userEvent.click(screen.getByText('Selesai 1'))
    await expect
      .element(screen.getByRole('dialog', { name: 'Detail Pengajuan' }))
      .toBeVisible()
  })

  it('sends the room type filter to the API', async () => {
    mocks.search = { type: 'LAB' }
    const seen = serve()
    await renderPage()
    await vi.waitFor(() =>
      expect(
        seen.some((q) => q.type === 'LAB' && Number(q.per_page) === 10)
      ).toBe(true)
    )
  })

  it('asks the API for the tab of the URL', async () => {
    mocks.search = { status: 'DITOLAK' }
    const seen = serve()
    const screen = await renderPage()
    await expect.element(screen.getByText('Ditolak 1')).toBeVisible()
    await expect.element(screen.getByText('Selesai 1')).not.toBeInTheDocument()
    expect(
      seen.some((q) => q.status === 'DITOLAK' && Number(q.per_page) === 10)
    ).toBe(true)
  })

  it('changes the tab through the URL and returns to page 1', async () => {
    serve()
    const screen = await renderPage()
    await userEvent.click(screen.getByRole('radio', { name: /Selesai/ }))
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({ page: 2 })).toMatchObject({
      status: 'SELESAI',
      page: undefined,
    })
  })

  it('limits the list and the totals to the chosen month', async () => {
    mocks.search = { month: '2026-09' }
    const seen = serve()
    const screen = await renderPage()
    await expect.element(screen.getByText('Selesai 1')).toBeVisible()
    await vi.waitFor(() => expect(seen.length).toBeGreaterThan(0))
    expect(
      seen.every(
        (q) =>
          q.from === '2026-09-01T00:00:00+07:00' &&
          q.to === '2026-10-01T00:00:00+07:00'
      )
    ).toBe(true)
  })

  it('offers the last twelve months and all time in the month filter', async () => {
    serve()
    const screen = await renderPage()
    await userEvent.click(screen.getByRole('combobox', { name: 'Bulan' }))
    await expect
      .element(screen.getByRole('option', { name: 'Semua waktu' }))
      .toBeVisible()
    await expect
      .element(screen.getByRole('option', { name: 'Okt 2026' }))
      .toBeVisible()
    await expect
      .element(screen.getByRole('option', { name: 'Nov 2025' }))
      .toBeVisible()
    await userEvent.click(screen.getByRole('option', { name: 'Sep 2026' }))
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({})).toMatchObject({ month: '2026-09' })
  })

  it('hides the pager while everything fits on one page', async () => {
    serve()
    const screen = await renderPage()
    await expect.element(screen.getByText('Selesai 1')).toBeVisible()
    await expect
      .element(screen.getByRole('navigation', { name: 'Halaman' }))
      .not.toBeInTheDocument()
  })

  it('shows the range and pages through a longer list', async () => {
    serve(
      Array.from({ length: 12 }, (_, i) =>
        makeBooking({ id: `p${i}`, status: 'SELESAI', purpose: `Acara ${i}` })
      )
    )
    const screen = await renderPage()
    await expect.element(screen.getByText('1–10 dari 12')).toBeVisible()
    await userEvent.click(
      screen.getByRole('button', { name: 'Halaman berikutnya' })
    )
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({})).toMatchObject({ page: 2 })
  })
})
