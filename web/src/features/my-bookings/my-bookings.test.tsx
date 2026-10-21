import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { type Booking } from '@/lib/api-types'
import { KELAS, makeBooking, POLICIES, slotOn } from './fixtures'
import { MyBookings } from './my-bookings-page'

const mocks = vi.hoisted(() => ({
  search: {} as Record<string, unknown>,
  navigate: vi.fn(),
  wide: true,
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
      params,
      children,
      ...props
    }: {
      to: string
      params?: Record<string, string>
      children?: React.ReactNode
    }) => (
      <a href={params ? to.replace('$id', params.id) : to} {...props}>
        {children}
      </a>
    ),
  }
})

vi.mock('@/hooks/use-media-query', () => ({ useMediaQuery: () => mocks.wide }))
// A table on a wide screen, cards on a narrow one (the test browser is narrow by default).
vi.mock('@/hooks/use-mobile', () => ({ useIsMobile: () => !mocks.wide }))

vi.mock('@/components/layout/page', () => ({
  Page: ({ title, children }: { title: string; children: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}))

const originalAdapter = api.defaults.adapter

function serve(bookings: Booking[]) {
  api.defaults.adapter = mockAdapter((config) => {
    if (config.url === '/room-types') {
      return { status: 200, data: { data: POLICIES } }
    }
    if (config.url === '/bookings') {
      return {
        status: 200,
        data: {
          data: bookings,
          meta: { page: 1, per_page: 100, total: bookings.length },
        },
      }
    }
    const match = String(config.url).match(/^\/bookings\/([^/]+)(\/events)?$/)
    if (match) {
      const found = bookings.find((b) => b.id === match[1])
      if (!found) return { status: 404 }
      return match[2]
        ? { status: 200, data: { data: [] } }
        : { status: 200, data: found }
    }
    return { status: 404 }
  })
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MyBookings />
    </QueryClientProvider>
  )
}

const waiting = makeBooking({
  id: 'w1',
  purpose: 'Praktikum Basis Data',
  ...slotOn(3, 10, 12),
})
const approved = makeBooking({
  id: 'a1',
  room: KELAS,
  room_id: KELAS.id,
  status: 'DISETUJUI',
  approval_mode: 'AUTO',
  purpose: 'Kuliah Fisika',
  ...slotOn(2, 14, 16),
})
const refused = makeBooking({
  id: 'r1',
  status: 'DITOLAK',
  purpose: 'Rapat panitia',
  decision_reason: 'Ruang dipakai sidang.',
  ...slotOn(4, 13, 15),
})
const finished = makeBooking({
  id: 'f1',
  status: 'SELESAI',
  purpose: 'Sudah lewat',
})
const cancelled = makeBooking({
  id: 'c1',
  status: 'DIBATALKAN',
  purpose: 'Dibatalkan',
})

describe('MyBookings (Pengajuan Saya)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.search = {}
    mocks.wide = true
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
  })

  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('lists current requests by start, without finished or cancelled ones, and counts them per status', async () => {
    serve([refused, waiting, approved, finished, cancelled])
    const screen = await renderPage()
    await expect
      .element(screen.getByText('Kuliah Fisika').first())
      .toBeVisible()
    await expect
      .element(screen.getByText('Praktikum Basis Data').first())
      .toBeVisible()
    await expect.element(screen.getByText('Rapat panitia')).toBeVisible()
    await expect
      .element(screen.getByText('Sudah lewat'))
      .not.toBeInTheDocument()
    await expect.element(screen.getByText('Dibatalkan')).not.toBeInTheDocument()
    const all = screen.getByRole('radio', { name: /Semua/ })
    await expect.element(all).toHaveTextContent('3')
    await expect
      .element(screen.getByRole('radio', { name: /Menunggu/ }))
      .toHaveTextContent('1')
    await expect
      .element(screen.getByRole('radio', { name: /Disetujui/ }))
      .toHaveTextContent('1')
    await expect
      .element(screen.getByRole('radio', { name: /Ditolak/ }))
      .toHaveTextContent('1')
    // The earliest start (the approved one, in 2 days) comes first.
    await vi.waitFor(() => {
      const order = [
        ...screen.container.querySelectorAll('tbody tr td:first-child'),
      ].map((td) => td.textContent)
      expect(order).toEqual(['R-401', 'LAB-RPL-02', 'LAB-RPL-02'])
    })
  })

  it('filters by the status tab from the URL', async () => {
    mocks.search = { status: 'MENUNGGU' }
    serve([refused, waiting, approved])
    const screen = await renderPage()
    await expect
      .element(screen.getByText('Praktikum Basis Data').first())
      .toBeVisible()
    await expect
      .element(screen.getByText('Kuliah Fisika'))
      .not.toBeInTheDocument()
    await expect
      .element(screen.getByText('Rapat panitia'))
      .not.toBeInTheDocument()
  })

  it('changes the tab through the URL', async () => {
    serve([waiting, approved])
    const screen = await renderPage()
    await userEvent.click(screen.getByRole('radio', { name: /Disetujui/ }))
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({ booking: 'x' })).toMatchObject({
      status: 'DISETUJUI',
      booking: 'x',
    })
  })

  it('shows the first request in the detail panel, with its steps', async () => {
    serve([waiting, approved])
    const screen = await renderPage()
    const panel = screen.getByRole('complementary', {
      name: 'Detail pengajuan',
    })
    await expect.element(panel.getByText('Kuliah Fisika')).toBeVisible()
    await expect.element(panel.getByText('Pengajuan dikirim')).toBeVisible()
    await expect.element(panel.getByText('Disetujui otomatis')).toBeVisible()
  })

  it('selects another request on click', async () => {
    serve([waiting, approved])
    const screen = await renderPage()
    await userEvent.click(screen.getByText('Praktikum Basis Data').first())
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({})).toMatchObject({ booking: 'w1' })
  })

  it('opens the cancel dialog from the detail panel', async () => {
    mocks.search = { booking: 'w1' }
    serve([waiting, approved])
    const screen = await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Batalkan Pengajuan' })
    )
    await expect
      .element(screen.getByRole('heading', { name: 'Batalkan pengajuan?' }))
      .toBeVisible()
  })

  it('offers edit and cancel in the row menu of a changeable request, and no menu for the others', async () => {
    serve([waiting, refused])
    const screen = await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: /^Aksi LAB-RPL-02/ }).first()
    )
    await expect
      .element(screen.getByRole('menuitem', { name: 'Ubah' }))
      .toBeVisible()
    await expect
      .element(screen.getByRole('menuitem', { name: 'Batalkan' }))
      .toBeVisible()
    // The detail is beside the list, and a refused request has nothing to change.
    await expect
      .element(screen.getByRole('menuitem', { name: 'Lihat detail' }))
      .not.toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: /^Aksi LAB-RPL-02, .*/ }))
      .toHaveLength(1)
  })

  it('opens the edit dialog from the row menu', async () => {
    serve([waiting])
    const screen = await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: /^Aksi LAB-RPL-02/ }).first()
    )
    await userEvent.click(screen.getByRole('menuitem', { name: 'Ubah' }))
    await expect
      .element(screen.getByRole('heading', { name: 'Ubah pengajuan' }))
      .toBeVisible()
  })

  it('shows an empty state with a way to request a room', async () => {
    serve([finished, cancelled])
    const screen = await renderPage()
    await expect.element(screen.getByText('Belum ada pengajuan')).toBeVisible()
    await expect
      .element(screen.getByRole('link', { name: 'Ajukan Ruang' }))
      .toHaveAttribute('href', '/schedule')
  })

  it('opens a popup instead of a side panel on a narrow screen', async () => {
    mocks.wide = false
    serve([waiting])
    const screen = await renderPage()
    await expect
      .element(screen.getByRole('complementary', { name: 'Detail pengajuan' }))
      .not.toBeInTheDocument()
    await userEvent.click(screen.getByText('Praktikum Basis Data').first())
    await expect
      .element(screen.getByRole('heading', { name: 'Detail Pengajuan' }))
      .toBeVisible()
  })

  it('opens the detail popup from the row menu on a narrow screen', async () => {
    mocks.wide = false
    serve([waiting])
    const screen = await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: /^Aksi LAB-RPL-02/ }).first()
    )
    await userEvent.click(
      screen.getByRole('menuitem', { name: 'Lihat detail' })
    )
    await expect
      .element(screen.getByRole('dialog', { name: 'Detail Pengajuan' }))
      .toBeVisible()
  })
})
