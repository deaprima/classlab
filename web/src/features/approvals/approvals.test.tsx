import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { type Booking } from '@/lib/api-types'
import { POLICIES, makeBooking, slotOn } from '@/features/my-bookings/fixtures'
import { Approvals } from '.'

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

const base = (over: Partial<Booking>) =>
  makeBooking({
    status: 'MENUNGGU',
    manual_reason: 'ROOM_POLICY',
    expires_at: new Date(
      Date.now() + 5 * 3_600_000 + 20 * 60_000
    ).toISOString(),
    ...over,
  })

const lab = base({
  id: 'b1',
  purpose: 'Praktikum Jaringan',
  ...slotOn(2, 10, 12),
})
const hall = base({
  id: 'b2',
  purpose: 'Seminar Nasional',
  manual_reason: 'ORGANIZATION',
  group_id: 'g1',
  occurrence_no: 1,
  occurrence_count: 2,
  letter: { attachment_id: 'att1', letter_number: '012/HMIF/X' },
  requester_summary: { active_points: 1, courses: [] },
  ...slotOn(3, 13, 16),
})
const hall2 = { ...hall, id: 'b3', occurrence_no: 2, ...slotOn(4, 9, 12) }

type Calls = { url: string; method: string; body?: unknown }[]

function serve(rows: Booking[]) {
  const calls: Calls = []
  api.defaults.adapter = mockAdapter((config) => {
    const url = String(config.url)
    const method = String(config.method)
    if (url === '/room-types') return { status: 200, data: { data: POLICIES } }
    if (method === 'post') {
      calls.push({
        url,
        method,
        body: config.data ? JSON.parse(String(config.data)) : undefined,
      })
      return { status: 200, data: lab }
    }
    if (url === '/bookings') {
      const q = (config.params ?? {}) as Record<string, string>
      const list = q.group_id
        ? rows.filter((r) => r.group_id === q.group_id)
        : rows
      return {
        status: 200,
        data: {
          data: list,
          meta: {
            page: 1,
            per_page: Number(q.per_page ?? 10),
            total: list.length,
          },
        },
      }
    }
    const found = rows.find((r) => url === `/bookings/${r.id}`)
    return found ? { status: 200, data: found } : { status: 404 }
  })
  return calls
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Approvals />
    </QueryClientProvider>
  )
}

describe('Approvals (Approval Pengajuan)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.search = {}
    useAuthStore
      .getState()
      .auth.setSession('token', { ...sampleUser, role: 'KALAB' })
  })

  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('lists the queue with the manual reason and folds a multi-date request into one line', async () => {
    serve([lab, hall, hall2])
    const screen = await renderPage()
    await expect
      .element(screen.getByText('Kebijakan ruang').first())
      .toBeVisible()
    await expect.element(screen.getByText('Organisasi').first()).toBeVisible()
    await expect.element(screen.getByText(/\+1 tanggal/)).toBeVisible()
    // Two requests, not three rows: the dates of the hall request share one line.
    expect(document.querySelectorAll('tbody tr').length).toBe(2)
  })

  it('shows the selected request: time left, points, letter and the two decisions', async () => {
    serve([hall, hall2, lab])
    const screen = await renderPage()
    const detail = screen.getByRole('complementary', {
      name: 'Detail pengajuan',
    })
    await expect.element(detail.getByText(/Kedaluwarsa dalam/)).toBeVisible()
    await expect.element(detail.getByText('2 tanggal (grup)')).toBeVisible()
    await expect.element(detail.getByText('1 / 6')).toBeVisible()
    await expect.element(detail.getByText('Surat 012/HMIF/X')).toBeVisible()
    await expect
      .element(detail.getByRole('button', { name: 'Unduh' }))
      .toBeVisible()
    await expect
      .element(detail.getByRole('button', { name: 'Tolak' }))
      .toBeVisible()
    await expect
      .element(detail.getByRole('button', { name: 'Setujui' }))
      .toBeVisible()
  })

  it('approves after a confirmation', async () => {
    const calls = serve([lab])
    const screen = await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Setujui' }))
    await userEvent.click(
      screen.getByRole('alertdialog').getByRole('button', { name: 'Setujui' })
    )
    await vi.waitFor(() =>
      expect(calls.map((c) => c.url)).toContain('/bookings/b1/approve')
    )
  })

  it('rejects only with a reason of at least five characters', async () => {
    const calls = serve([lab])
    const screen = await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Tolak' }))
    await expect
      .element(screen.getByRole('heading', { name: 'Tolak pengajuan?' }))
      .toBeVisible()
    await userEvent.click(
      screen.getByRole('dialog').getByRole('button', { name: 'Tolak' })
    )
    await expect.element(screen.getByText(/minimal 5 karakter/i)).toBeVisible()
    expect(calls).toHaveLength(0)

    await userEvent.fill(
      screen.getByLabelText('Alasan penolakan'),
      'Bentrok dengan kegiatan jurusan.'
    )
    await userEvent.click(
      screen.getByRole('dialog').getByRole('button', { name: 'Tolak' })
    )
    await vi.waitFor(() => expect(calls).toHaveLength(1))
    expect(calls[0]).toMatchObject({
      url: '/bookings/b1/reject',
      body: { reason: 'Bentrok dengan kegiatan jurusan.' },
    })
  })

  it('switches the status tab through the URL and returns to page 1', async () => {
    serve([lab])
    const screen = await renderPage()
    await userEvent.click(screen.getByRole('radio', { name: /Disetujui/ }))
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({ page: 3, booking: 'x' })).toMatchObject({
      status: 'DISETUJUI',
      page: undefined,
      booking: undefined,
    })
  })

  it('filters by room type through the URL', async () => {
    serve([lab])
    const screen = await renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Laboratorium' }))
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({})).toMatchObject({ type: 'LAB' })
  })

  it('narrows the queue by the search text of the URL', async () => {
    mocks.search = { q: 'jaringan' }
    serve([lab, hall, hall2])
    const screen = await renderPage()
    await expect.element(screen.getByText('LAB-RPL-02').first()).toBeVisible()
    expect(document.querySelectorAll('tbody tr').length).toBe(1)
  })

  it('says so when the search matches nothing', async () => {
    mocks.search = { q: 'zzz' }
    serve([lab])
    const screen = await renderPage()
    await expect.element(screen.getByText('Tidak ada yang cocok')).toBeVisible()
  })

  it('commits a search through the URL and returns to page 1', async () => {
    serve([lab])
    const screen = await renderPage()
    const box = screen.getByRole('searchbox', { name: 'Cari pengajuan' })
    await userEvent.fill(box, 'fajar')
    await userEvent.keyboard('{Enter}')
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({ page: 3, booking: 'x' })).toMatchObject({
      q: 'fajar',
      page: undefined,
      booking: undefined,
    })
  })

  it('pages the queue ten at a time', async () => {
    const many = Array.from({ length: 12 }, (_, i) =>
      base({ id: `m${i}`, purpose: `Acara ${i}` })
    )
    serve(many)
    const screen = await renderPage()
    await expect.element(screen.getByText('1–10 dari 12')).toBeVisible()
    expect(document.querySelectorAll('tbody tr').length).toBe(10)
    await userEvent.click(
      screen.getByRole('button', { name: 'Halaman berikutnya' })
    )
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({})).toMatchObject({ page: 2 })
  })

  it('shows an empty state when nothing is waiting', async () => {
    serve([])
    const screen = await renderPage()
    await expect
      .element(screen.getByText('Tidak ada yang menunggu'))
      .toBeVisible()
  })
})
