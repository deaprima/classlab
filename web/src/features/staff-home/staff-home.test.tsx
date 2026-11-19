import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { type Booking, type UserRole } from '@/lib/api-types'
import { POLICIES, makeBooking, slotOn } from '@/features/my-bookings/fixtures'
import { Home } from '.'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
  }
})
vi.mock('@/components/layout/page', () => ({
  Page: ({
    title,
    description,
    aside,
    children,
  }: {
    title: string
    description?: React.ReactNode
    aside?: React.ReactNode
    children: React.ReactNode
  }) => (
    <div>
      <h1>{title}</h1>
      <p>{description}</p>
      {aside}
      {children}
    </div>
  ),
}))
vi.mock('@/features/dashboard', () => ({ Dashboard: () => <p>Dashboard</p> }))

const originalAdapter = api.defaults.adapter

const pending = (over: Partial<Booking>) =>
  makeBooking({ status: 'MENUNGGU', manual_reason: 'ROOM_POLICY', ...over })
const first = pending({
  id: 'b1',
  purpose: 'Praktikum Jaringan',
  ...slotOn(2, 10, 12),
})
const second = pending({
  id: 'b2',
  purpose: 'Seminar Nasional',
  ...slotOn(3, 13, 16),
})

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
const hours = { open_time: '07:00', close_time: '21:00', custom: false }
const AVAILABILITY = [
  { room: room('R-1', 'KELAS'), operating_hours: hours, status: 'DIGUNAKAN' },
  { room: room('R-2', 'KELAS'), operating_hours: hours, status: 'TERSEDIA' },
  { room: room('LAB-1', 'LAB'), operating_hours: hours, status: 'DIAJUKAN' },
].map((r) => ({
  ...r,
  conflicts:
    r.status === 'DIGUNAKAN'
      ? [
          {
            kind: 'SCHEDULE',
            title: 'Kuliah Basis Data',
            start_at: new Date(Date.now() - 3_600_000).toISOString(),
            end_at: new Date(Date.now() + 3_600_000).toISOString(),
          },
        ]
      : [],
}))

const page = <T,>(data: T[], total = data.length) => ({
  data,
  meta: { page: 1, per_page: 20, total },
})

const CHECKOUTS = [
  {
    booking_id: 'c1',
    room_id: 'R-1',
    room_code: 'R-1',
    room_name: 'Ruang 1',
    room_type: 'KELAS',
    checked_out_at: '2026-10-22T03:00:00Z',
    condition: 'ADA_MASALAH',
    note: 'Proyektor mati',
    attachment_id: 'a1',
  },
  {
    booking_id: 'c2',
    room_id: 'R-2',
    room_code: 'R-2',
    room_name: 'Ruang 2',
    room_type: 'KELAS',
    checked_out_at: '2026-10-22T02:00:00Z',
    condition: 'NORMAL',
    note: null,
    attachment_id: 'a2',
  },
]

function serve(calls: string[] = []) {
  api.defaults.adapter = mockAdapter((config) => {
    const url = String(config.url)
    const q = (config.params ?? {}) as Record<string, string>
    calls.push(`${config.method} ${url}`)
    if (config.method === 'post') return { status: 200, data: first }
    if (url === '/room-types') return { status: 200, data: { data: POLICIES } }
    if (url === '/availability') {
      return { status: 200, data: { data: AVAILABILITY } }
    }
    if (url === '/bookings') {
      if (q.status === 'BERLANGSUNG') return { status: 200, data: page([], 3) }
      return { status: 200, data: page([first, second], 7) }
    }
    if (url === '/reports') return { status: 200, data: page([], 2) }
    if (url === '/users') return { status: 200, data: page([], 1248) }
    if (url === '/audit-logs') return { status: 200, data: page([], 86) }
    if (url === '/checkouts') return { status: 200, data: page(CHECKOUTS) }
    return { status: 404 }
  })
  return calls
}

function renderAs(role: UserRole) {
  useAuthStore.getState().auth.setSession('t', {
    ...sampleUser,
    name: 'Hendra Wijaya',
    role,
  })
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Home />
    </QueryClientProvider>
  )
}

describe('Beranda per role', () => {
  beforeEach(() => {
    serve()
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('shows the requesters their Dashboard', async () => {
    const screen = await renderAs('MAHASISWA')
    await expect.element(screen.getByText('Dashboard')).toBeInTheDocument()
  })

  it('gives TU the counts, the pending queue and the room status', async () => {
    const screen = await renderAs('TU')

    await expect
      .element(screen.getByRole('heading', { name: 'Hai, Hendra' }))
      .toBeVisible()
    await expect.element(screen.getByText('Pengajuan Menunggu')).toBeVisible()
    // The clock sits in the page header and shows seconds in WIB.
    await expect
      .element(screen.getByLabelText('Waktu sekarang'))
      .toHaveTextContent(/\d{2}:\d{2}:\d{2}\s*WIB/)
    // TU approves classrooms only: 1 of its 2 rooms is in use.
    await expect.element(screen.getByText('1 / 2')).toBeVisible()
    await expect.element(screen.getByText('50% sedang terpakai')).toBeVisible()
    await expect.element(screen.getByText('Praktikum Jaringan')).toBeVisible()
    await expect.element(screen.getByText('Seminar Nasional')).toBeVisible()
    await expect
      .element(screen.getByRole('img', { name: /2 ruang: 1 dipakai/ }))
      .toBeInTheDocument()
    // Who is in the room now.
    await expect.element(screen.getByText('Kuliah Basis Data')).toBeVisible()
    await expect
      .element(screen.getByText('Sedang Dipakai Sekarang'))
      .toBeVisible()
  })

  it('approves from the queue after confirming', async () => {
    const calls: string[] = []
    serve(calls)
    const screen = await renderAs('TU')

    await userEvent.click(
      screen.getByRole('button', { name: /^Setujui pengajuan/ }).first()
    )
    await userEvent.click(
      screen.getByRole('alertdialog').getByRole('button', { name: 'Setujui' })
    )

    await vi.waitFor(() => expect(calls).toContain('post /bookings/b1/approve'))
  })

  it('shows Kalab the lab wording and only the lab rooms', async () => {
    const screen = await renderAs('KALAB')

    await expect.element(screen.getByText('Lab Menunggu')).toBeVisible()
    await expect
      .element(screen.getByText('Perlu Persetujuan Lab'))
      .toBeVisible()
    await expect.element(screen.getByText('0 / 1')).toBeVisible()
    await expect.element(screen.getByText('Total Lab')).toBeVisible()
  })

  it('shows Admin the system numbers', async () => {
    const screen = await renderAs('ADMIN')

    await expect.element(screen.getByText('1.248')).toBeVisible()
    await expect.element(screen.getByText('86', { exact: true })).toBeVisible()
    await expect.element(screen.getByText('Tipe Ruang')).toBeVisible()
    // Admin sees every room in the ring: 3 rooms, 1 in use.
    await expect
      .element(screen.getByRole('img', { name: /3 ruang: 1 dipakai/ }))
      .toBeInTheDocument()
  })

  it('shows OB the latest check-outs and no approval buttons', async () => {
    const screen = await renderAs('OB')

    await expect.element(screen.getByText('Check-out Terbaru')).toBeVisible()
    await expect.element(screen.getByText('Ada masalah')).toBeVisible()
    await expect.element(screen.getByText(/Proyektor mati/)).toBeVisible()
    await expect.element(screen.getByText('Normal')).toBeVisible()
    expect(
      screen.getByRole('button', { name: /Setujui/ }).elements()
    ).toHaveLength(0)
  })
})
