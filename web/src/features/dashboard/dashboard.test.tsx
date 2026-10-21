import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { type Booking } from '@/lib/api-types'
import { Dashboard } from '.'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    Link: ({
      to,
      children,
      ...props
    }: {
      to: string
      children: React.ReactNode
    }) => (
      <a href={to} {...props}>
        {children}
      </a>
    ),
  }
})

// The page chrome (Top Bar, search, notifications) has its own tests and needs app providers.
vi.mock('@/components/layout/page', () => ({
  Page: ({
    title,
    description,
    children,
  }: {
    title: string
    description?: string
    children: React.ReactNode
  }) => (
    <div>
      <h1>{title}</h1>
      {description && <p>{description}</p>}
      {children}
    </div>
  ),
}))

const originalAdapter = api.defaults.adapter

const room: Booking['room'] = {
  id: 'room-1',
  org_unit_id: sampleUser.org_unit_id,
  code: 'R-401',
  name: 'Ruang 401',
  type: 'KELAS',
  capacity: 60,
  facilities: [],
  is_active: true,
  open_time: '07:00',
  close_time: '21:00',
}

function booking(over: Partial<Booking>): Booking {
  return {
    id: 'b-1',
    room_id: room.id,
    room,
    requester_id: sampleUser.id,
    start_at: '2026-10-21T14:00:00+07:00',
    end_at: '2026-10-21T15:30:00+07:00',
    purpose: 'Belajar',
    details: {},
    status: 'DISETUJUI',
    approval_mode: 'AUTO',
    manual_reason: null,
    group_id: null,
    occurrence_no: null,
    occurrence_count: 1,
    expires_at: null,
    edited_at: null,
    letter: null,
    created_at: '2026-10-20T09:00:00+07:00',
    ...over,
  } as Booking
}

function serve(bookings: Booking[], me: Record<string, unknown> = {}) {
  api.defaults.adapter = mockAdapter((config) => {
    if (config.url === '/bookings') {
      return {
        status: 200,
        data: {
          data: bookings,
          meta: { page: 1, per_page: 100, total: bookings.length },
        },
      }
    }
    if (config.url === '/auth/me') {
      return { status: 200, data: { ...sampleUser, active_points: 0, ...me } }
    }
    return { status: 404 }
  })
}

function renderDashboard() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Dashboard />
    </QueryClientProvider>
  )
}

function signIn(user: Record<string, unknown> = {}) {
  useAuthStore.getState().auth.setSession('token', {
    ...sampleUser,
    name: 'Nisrina Nur Fatin',
    ...user,
  })
}

describe('Dashboard', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    // 13:50 WIB: inside the check-in window of a 14:00 booking.
    vi.setSystemTime(new Date('2026-10-21T13:50:00+07:00'))
    signIn()
  })

  afterEach(() => {
    vi.useRealTimers()
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('greets a student by first name and counts active bookings', async () => {
    serve([booking({})])
    const screen = await renderDashboard()
    await expect
      .element(screen.getByRole('heading', { name: 'Hai, Nisrina' }))
      .toBeVisible()
    await expect
      .element(screen.getByText('Kamu punya 1 booking aktif.'))
      .toBeVisible()
  })

  it('shows the next booking and enables check-in inside the window', async () => {
    serve([booking({})])
    const screen = await renderDashboard()
    await expect.element(screen.getByText('Mulai dalam 10 menit')).toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Check-in' }))
      .toBeEnabled()
    await expect
      .element(screen.getByRole('button', { name: 'Check-out' }))
      .toBeDisabled()
  })

  it('keeps check-in closed before the window opens', async () => {
    serve([
      booking({
        start_at: '2026-10-21T16:00:00+07:00',
        end_at: '2026-10-21T17:00:00+07:00',
      }),
    ])
    const screen = await renderDashboard()
    await expect
      .element(screen.getByRole('button', { name: 'Check-in' }))
      .toBeDisabled()
  })

  it('explains that check-in happens by scanning the room QR', async () => {
    serve([booking({})])
    const screen = await renderDashboard()
    await userEvent.click(screen.getByRole('button', { name: 'Check-in' }))
    await expect
      .element(screen.getByRole('dialog'))
      .toHaveTextContent(/Pindai QR yang menempel di pintu R-401/)
  })

  it('offers check-out while the booking is running', async () => {
    serve([booking({ status: 'BERLANGSUNG' })])
    const screen = await renderDashboard()
    await expect.element(screen.getByText('Sedang berlangsung')).toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Check-out' }))
      .toBeEnabled()
    await expect
      .element(screen.getByRole('button', { name: 'Check-in' }))
      .toBeDisabled()
  })

  it('lists waiting and rejected requests with the time left', async () => {
    serve([
      booking({
        id: 'w',
        status: 'MENUNGGU',
        expires_at: '2026-10-22T13:50:00+07:00',
        room: { ...room, code: 'LAB-RPL-02', type: 'LAB' },
      }),
      booking({ id: 'r', status: 'DITOLAK', room: { ...room, code: 'R-402' } }),
    ])
    const screen = await renderDashboard()
    await expect.element(screen.getByText('LAB-RPL-02')).toBeVisible()
    await expect.element(screen.getByText(/sisa 24 j/)).toBeVisible()
    await expect.element(screen.getByText('Ditolak')).toBeVisible()
  })

  it('shows an empty state when nothing is approved yet', async () => {
    serve([])
    const screen = await renderDashboard()
    await expect
      .element(screen.getByText('Belum ada booking disetujui'))
      .toBeVisible()
    await expect
      .element(screen.getByText('Belum ada booking aktif.'))
      .toBeVisible()
  })

  it('warns while booking is blocked', async () => {
    serve([], {
      booking_blocked_until: '2026-11-04T08:00:00+07:00',
      active_points: 6,
    })
    const screen = await renderDashboard()
    await expect
      .element(screen.getByText('Pengajuan diblokir sementara'))
      .toBeVisible()
  })

  it('does not warn once the block has passed', async () => {
    serve([], { booking_blocked_until: '2026-10-01T08:00:00+07:00' })
    const screen = await renderDashboard()
    await expect
      .element(screen.getByText('Belum ada booking aktif.'))
      .toBeVisible()
    await expect
      .element(screen.getByText('Pengajuan diblokir sementara'))
      .not.toBeInTheDocument()
  })

  it('gives an organisation its own greeting and primary action', async () => {
    useAuthStore.getState().auth.reset()
    signIn({ role: 'ORGANISASI', name: 'HIMA Informatika' })
    serve([])
    const screen = await renderDashboard()
    await expect
      .element(screen.getByRole('heading', { name: 'Hai, HIMA Informatika' }))
      .toBeVisible()
    await expect
      .element(screen.getByRole('link', { name: 'Ajukan Auditorium' }))
      .toBeVisible()
  })
})
