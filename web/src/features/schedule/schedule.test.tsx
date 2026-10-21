import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { Schedule } from '.'

const mocks = vi.hoisted(() => ({
  search: {} as Record<string, unknown>,
  navigate: vi.fn(),
  mobile: false,
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
      search,
      params,
      children,
      ...props
    }: {
      to: string
      search?: Record<string, string>
      params?: Record<string, string>
      children?: React.ReactNode
    }) => {
      const path = params ? to.replace('$id', params.id) : to
      const query = search ? `?${new URLSearchParams(search).toString()}` : ''
      return (
        <a href={`${path}${query}`} {...props}>
          {children}
        </a>
      )
    },
  }
})

vi.mock('@/hooks/use-mobile', () => ({ useIsMobile: () => mocks.mobile }))

vi.mock('@/components/layout/page', () => ({
  Page: ({ title, children }: { title: string; children: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}))

const base = {
  org_unit_id: sampleUser.org_unit_id,
  facilities: ['Proyektor', 'AC'],
  is_active: true,
  open_time: null,
  close_time: null,
}
const R302 = {
  ...base,
  id: 'room-302',
  code: 'R-302',
  name: 'R-302',
  type: 'KELAS',
  capacity: 50,
}
const R402 = {
  ...base,
  id: 'room-402',
  code: 'R-402',
  name: 'R-402',
  type: 'KELAS',
  capacity: 60,
}
const R301 = {
  ...base,
  id: 'room-301',
  code: 'R-301',
  name: 'R-301',
  type: 'KELAS',
  capacity: 50,
}
const LAB = {
  ...base,
  id: 'room-lab',
  code: 'LAB-RPL-02',
  name: 'Lab',
  type: 'LAB',
  capacity: 32,
}

const policy = (over: Record<string, unknown>) => ({
  label: 'x',
  approver_roles: ['TU'],
  requester_roles: ['MAHASISWA'],
  approval_policy: 'AUTO_IF_ELIGIBLE',
  letter_required: false,
  open_time: '07:00',
  close_time: '21:00',
  min_duration_min: 30,
  max_duration_min: 480,
  max_advance_days: 7,
  updated_at: 'x',
  ...over,
})
const POLICIES = [
  policy({ type: 'KELAS', label: 'Ruang Kelas' }),
  policy({
    type: 'LAB',
    label: 'Laboratorium',
    approver_roles: ['KALAB'],
    approval_policy: 'ALWAYS_MANUAL',
  }),
]

const originalAdapter = api.defaults.adapter

function serve() {
  const entry = (room: typeof R302, status: 'TERSEDIA' | 'DIGUNAKAN') => ({
    room,
    operating_hours: { open_time: '07:00', close_time: '21:00', custom: false },
    status,
    conflicts: [],
  })
  api.defaults.adapter = mockAdapter((config) => {
    if (config.url === '/auth/me') {
      return { status: 200, data: { ...sampleUser, active_points: 0 } }
    }
    if (config.url === '/bookings' && config.method === 'post') {
      return {
        status: 201,
        data: {
          id: 'b1',
          status: 'DISETUJUI',
          room: R302,
          ...JSON.parse(String(config.data)),
        },
      }
    }
    if (config.url === '/room-types')
      return { status: 200, data: { data: POLICIES } }
    if (config.url === '/availability') {
      return {
        status: 200,
        data: {
          data: [
            entry(R302, 'TERSEDIA'),
            entry(R402, 'TERSEDIA'),
            entry(R301, 'DIGUNAKAN'),
            entry(LAB, 'TERSEDIA'),
          ],
        },
      }
    }
    if (String(config.url).endsWith('/schedule')) {
      return {
        status: 200,
        data: {
          data: [
            {
              id: 'i1',
              kind: 'SCHEDULE',
              title: 'Statistika',
              start_at: '2026-10-21T08:00:00+07:00',
              end_at: '2026-10-21T10:00:00+07:00',
            },
          ],
        },
      }
    }
    return { status: 404 }
  })
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Schedule />
    </QueryClientProvider>
  )
}

describe('Schedule (Jadwal Ruang)', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-10-21T06:00:00+07:00'))
    vi.clearAllMocks()
    mocks.search = { date: '2026-10-21', start: '08:00', end: '10:00' }
    mocks.mobile = false
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
    serve()
  })

  afterEach(() => {
    vi.useRealTimers()
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('counts the free rooms of the chosen group and lists them with their status', async () => {
    const screen = await renderPage()
    await expect
      .element(screen.getByText('2 dari 3 ruang kelas kosong'))
      .toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: /R-402/ }))
      .toBeVisible()
    await expect.element(screen.getByText('Digunakan').first()).toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: /LAB-RPL-02/ }))
      .not.toBeInTheDocument()
  })

  it('switches to the lab group', async () => {
    const screen = await renderPage()
    await expect
      .element(screen.getByText('2 dari 3 ruang kelas kosong'))
      .toBeVisible()
    await userEvent.click(screen.getByRole('radio', { name: 'Lab' }))
    expect(mocks.navigate).toHaveBeenCalledOnce()
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({ date: 'd' })).toMatchObject({
      group: 'LAB',
      room: undefined,
    })
  })

  it('shows the week of the first free room and picks another room on click', async () => {
    const screen = await renderPage()
    await expect
      .element(screen.getByRole('heading', { name: 'R-302' }))
      .toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: /R-402/ }))
    const { search } = mocks.navigate.mock.calls[0][0]
    expect(search({})).toMatchObject({ room: 'room-402' })
  })

  it('draws the week grid on desktop with the booked lecture', async () => {
    const screen = await renderPage()
    await expect.element(screen.getByText('Statistika').first()).toBeVisible()
    await expect
      .element(screen.getByRole('grid', { name: 'Jadwal mingguan ruang' }))
      .toBeVisible()
  })

  it('keeps the request bar on screen, disabled until a slot is picked', async () => {
    const screen = await renderPage()
    const bar = screen.getByRole('region', { name: 'Pengajuan ruang' })
    await expect.element(bar).toBeVisible()
    await expect
      .element(
        screen.getByText('Pilih jam kosong di jadwal R-302 untuk mengajukan.')
      )
      .toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Ajukan Ruang' }))
      .toBeDisabled()
  })

  it('picks a start and an end on the grid and shows them in the bar', async () => {
    const screen = await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Pilih 2026-10-21 12:00' })
    )
    await expect
      .element(screen.getByText(/Rab 21 · 12:00 – 12:30/))
      .toBeVisible()
    await userEvent.click(
      screen.getByRole('button', { name: 'Pilih 2026-10-21 13:30' })
    )
    await expect
      .element(screen.getByText(/Rab 21 · 12:00 – 14:00/))
      .toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Ajukan Ruang' }))
      .toBeEnabled()
  })

  it('picks slots on several days and removes one', async () => {
    const screen = await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Pilih 2026-10-21 12:00' })
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Pilih 2026-10-22 09:00' })
    )
    await expect.element(screen.getByText(/Rab 21 · 12:00/)).toBeVisible()
    await expect.element(screen.getByText(/Kam 22 · 09:00/)).toBeVisible()
    await userEvent.click(
      screen.getByRole('button', { name: 'Hapus pilihan Rab 21' })
    )
    await expect
      .element(screen.getByText(/Rab 21 · 12:00/))
      .not.toBeInTheDocument()
    await expect.element(screen.getByText(/Kam 22 · 09:00/)).toBeVisible()
  })

  it('clicking the picked slot again clears it', async () => {
    const screen = await renderPage()
    const cell = screen.getByRole('button', { name: 'Pilih 2026-10-21 12:00' })
    await userEvent.click(cell)
    await userEvent.click(cell)
    await expect
      .element(
        screen.getByText('Pilih jam kosong di jadwal R-302 untuk mengajukan.')
      )
      .toBeVisible()
  })

  it('opens the request form as a popup with the picked slots', async () => {
    const screen = await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Pilih 2026-10-21 12:00' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Ajukan Ruang' }))
    const dialog = screen.getByRole('dialog')
    await expect.element(dialog).toBeVisible()
    await expect.element(dialog.getByLabelText('Nama kegiatan')).toBeVisible()
    await expect.element(dialog.getByText('12:00 – 12:30')).toBeVisible()
  })

  it('sends the request from the popup and clears the picked slots', async () => {
    const screen = await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Pilih 2026-10-21 12:00' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Ajukan Ruang' }))
    const dialog = screen.getByRole('dialog')
    await userEvent.fill(
      dialog.getByLabelText('Nama kegiatan'),
      'Belajar bareng'
    )
    await userEvent.click(
      dialog.getByRole('button', { name: 'Kirim Pengajuan' })
    )
    await expect.element(screen.getByRole('dialog')).not.toBeInTheDocument()
    await expect
      .element(
        screen.getByText('Pilih jam kosong di jadwal R-302 untuk mengajukan.')
      )
      .toBeVisible()
  })

  it('starts over when another room is chosen', async () => {
    const screen = await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Pilih 2026-10-21 12:00' })
    )
    mocks.search = { ...mocks.search, room: 'room-402' }
    await screen.rerender(
      <QueryClientProvider client={new QueryClient()}>
        <Schedule />
      </QueryClientProvider>
    )
    await expect
      .element(
        screen.getByText('Pilih jam kosong di jadwal R-402 untuk mengajukan.')
      )
      .toBeVisible()
  })

  it('lists the day as slot chips on mobile, with the lecture slots unavailable', async () => {
    mocks.mobile = true
    const screen = await renderPage()
    await expect
      .element(screen.getByRole('radio', { name: 'Rab 21' }))
      .toBeChecked()
    await expect
      .element(
        screen.getByRole('button', { name: '08:00, terpakai: Statistika' })
      )
      .toBeDisabled()
    await userEvent.click(
      screen.getByRole('button', { name: '12:00, tersedia' })
    )
    await expect
      .element(screen.getByText(/Rab 21 · 12:00 – 12:30/))
      .toBeVisible()
    await expect
      .element(screen.getByRole('grid', { name: 'Jadwal mingguan ruang' }))
      .not.toBeInTheDocument()
  })

  it('hides the request actions from a role that cannot book', async () => {
    useAuthStore.getState().auth.reset()
    useAuthStore
      .getState()
      .auth.setSession('token', { ...sampleUser, role: 'TU' })
    const screen = await renderPage()
    await expect
      .element(screen.getByRole('heading', { name: 'R-302' }))
      .toBeVisible()
    await expect
      .element(screen.getByRole('region', { name: 'Pengajuan ruang' }))
      .not.toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: 'Pilih 2026-10-21 12:00' }))
      .not.toBeInTheDocument()
  })
})
