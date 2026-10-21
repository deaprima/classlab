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
import { RoomDetail } from './room-detail'

const mocks = vi.hoisted(() => ({
  search: {} as Record<string, unknown>,
  navigate: vi.fn(),
}))

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    getRouteApi: () => ({
      useParams: () => ({ id: 'room-lab' }),
      useSearch: () => mocks.search,
      useNavigate: () => mocks.navigate,
    }),
    Link: ({
      to,
      search,
      children,
      ...props
    }: {
      to: string
      search?: Record<string, string>
      children?: React.ReactNode
    }) => {
      const query = search ? `?${new URLSearchParams(search).toString()}` : ''
      return (
        <a href={`${to}${query}`} {...props}>
          {children}
        </a>
      )
    },
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

const LAB = {
  id: 'room-lab',
  org_unit_id: sampleUser.org_unit_id,
  code: 'LAB-RPL-02',
  name: 'Lab RPL',
  type: 'LAB',
  capacity: 32,
  facilities: ['Proyektor', 'AC', 'WiFi'],
  is_active: true,
  open_time: '07:00',
  close_time: '12:00',
}

const POLICY = {
  type: 'LAB',
  label: 'Laboratorium',
  approver_roles: ['KALAB'],
  requester_roles: ['MAHASISWA'],
  approval_policy: 'ALWAYS_MANUAL',
  letter_required: false,
  open_time: '07:00',
  close_time: '21:00',
  min_duration_min: 30,
  max_duration_min: 480,
  max_advance_days: 7,
  updated_at: 'x',
}

const originalAdapter = api.defaults.adapter

function serve(room: Record<string, unknown> | 'missing' = LAB) {
  api.defaults.adapter = mockAdapter((config) => {
    if (config.url === '/auth/me') {
      return { status: 200, data: { ...sampleUser, active_points: 0 } }
    }
    if (config.url === '/availability') {
      return {
        status: 200,
        data: {
          data: [
            {
              room,
              operating_hours: {
                open_time: '07:00',
                close_time: '12:00',
                custom: true,
              },
              status: 'TERSEDIA',
              conflicts: [],
            },
          ],
        },
      }
    }
    if (config.url === '/room-types')
      return { status: 200, data: { data: [POLICY] } }
    if (config.url === '/rooms/room-lab') {
      return room === 'missing'
        ? { status: 404, data: apiErrorBody('NOT_FOUND', 'Ruang tidak ada.') }
        : { status: 200, data: room }
    }
    if (config.url === '/rooms/room-lab/schedule') {
      return {
        status: 200,
        data: {
          data: [
            {
              id: 'i1',
              kind: 'SCHEDULE',
              title: 'Basis Data',
              start_at: '2026-10-21T08:00:00+07:00',
              end_at: '2026-10-21T09:00:00+07:00',
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
      <RoomDetail />
    </QueryClientProvider>
  )
}

describe('RoomDetail', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-10-21T06:00:00+07:00'))
    vi.clearAllMocks()
    mocks.search = {}
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
  })

  afterEach(() => {
    vi.useRealTimers()
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('shows the room facts and its own operating hours', async () => {
    serve()
    const screen = await renderPage()
    await expect
      .element(screen.getByRole('heading', { name: 'LAB-RPL-02' }))
      .toBeVisible()
    await expect.element(screen.getByText('32 orang')).toBeVisible()
    await expect.element(screen.getByText('07:00 – 12:00')).toBeVisible()
    await expect
      .element(screen.getByText('Proyektor', { exact: true }))
      .toBeVisible()
  })

  it('lists the slots of the day inside the hours with the lecture marked used', async () => {
    serve()
    const screen = await renderPage()
    await expect
      .element(
        screen.getByRole('button', { name: '08:00, terpakai: Basis Data' })
      )
      .toBeDisabled()
    await expect
      .element(screen.getByRole('button', { name: '09:00, tersedia' }))
      .toBeEnabled()
    // The room closes at 12:00: no slot after 11:30.
    await expect
      .element(screen.getByRole('button', { name: /^12:00/ }))
      .not.toBeInTheDocument()
  })

  it('picks a range from the slot chips and opens the popup with it', async () => {
    serve()
    const screen = await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: '09:00, tersedia' })
    )
    await userEvent.click(
      screen.getByRole('button', { name: '10:30, tersedia' })
    )
    await expect
      .element(screen.getByText('Pengajuan dibuka maksimal 7 hari ke depan.'))
      .toBeVisible()

    await userEvent.click(screen.getByRole('button', { name: 'Ajukan Ruang' }))

    const dialog = screen.getByRole('dialog')
    await expect.element(dialog).toBeVisible()
    await expect
      .element(dialog.getByRole('combobox', { name: 'Mulai' }))
      .toHaveTextContent('09:00')
    await expect
      .element(dialog.getByRole('combobox', { name: 'Selesai' }))
      .toHaveTextContent('11:00')
    await expect.element(dialog.getByLabelText('Nama kegiatan')).toBeVisible()
  })

  it('moves to another day', async () => {
    serve()
    const screen = await renderPage()
    await userEvent.click(screen.getByRole('radio', { name: 'Kam 22' }))
    expect(mocks.navigate).toHaveBeenCalledWith({
      search: { date: '2026-10-22' },
      replace: true,
    })
  })

  it('has no request panel for a role that cannot book', async () => {
    serve()
    useAuthStore.getState().auth.reset()
    useAuthStore
      .getState()
      .auth.setSession('token', { ...sampleUser, role: 'OB' })
    const screen = await renderPage()
    await expect
      .element(screen.getByRole('heading', { name: 'LAB-RPL-02' }))
      .toBeVisible()
    await expect
      .element(screen.getByText('Ajukan Ruang Ini'))
      .not.toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: '09:00, tersedia' }))
      .toBeDisabled()
  })

  it('says so when the room does not exist', async () => {
    serve('missing')
    const screen = await renderPage()
    await expect
      .element(screen.getByText('Ruang tidak ditemukan atau sudah dihapus.'))
      .toBeVisible()
  })

  it('marks an inactive room and offers no request', async () => {
    serve({ ...LAB, is_active: false })
    const screen = await renderPage()
    await expect.element(screen.getByText('Nonaktif')).toBeVisible()
    await expect
      .element(screen.getByText('Ajukan Ruang Ini'))
      .not.toBeInTheDocument()
  })
})
