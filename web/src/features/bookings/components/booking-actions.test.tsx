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
import { type Booking } from '@/lib/api-types'
import { BookingActions } from './booking-actions'

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
vi.mock('sonner', () => ({ toast }))

const originalAdapter = api.defaults.adapter
const TU = {
  ...sampleUser,
  id: 'tu-id',
  role: 'TU_KALAB' as const,
  name: 'Siti',
}

const booking: Booking = {
  id: '11111111-1111-4111-8111-111111111111',
  room_id: 'room-id',
  room: {
    id: 'room-id',
    org_unit_id: sampleUser.org_unit_id,
    code: 'LAB-2',
    name: 'Lab RPL',
    type: 'LAB',
    capacity: 40,
    facilities: [],
    is_active: true,
  },
  requester_id: sampleUser.id,
  requester: { id: sampleUser.id, identifier: '2021101001', name: 'Andi' },
  start_at: '2099-10-21T13:00:00+07:00',
  end_at: '2099-10-21T14:30:00+07:00',
  purpose: 'Latihan kelompok',
  status: 'MENUNGGU',
  created_at: '2026-10-14T09:00:00+07:00',
}

function renderActions(b: Booking = booking) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <BookingActions booking={b} />
    </QueryClientProvider>
  )
}

describe('BookingActions', () => {
  beforeEach(() => vi.clearAllMocks())
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('requires a reason of at least 5 characters to reject (BR-04)', async () => {
    useAuthStore.getState().auth.setSession('t', TU)
    const bodies: unknown[] = []
    api.defaults.adapter = mockAdapter((config) => {
      bodies.push(JSON.parse(String(config.data)))
      return { status: 200, data: { ...booking, status: 'DITOLAK' } }
    })
    const screen = await renderActions()

    await userEvent.click(screen.getByRole('button', { name: 'Tolak' }))
    await userEvent.fill(screen.getByLabelText('Alasan'), 'abc')
    await userEvent.click(screen.getByRole('button', { name: 'Tolak' }).last())
    await expect
      .element(screen.getByText('Alasan wajib diisi, minimal 5 karakter.'))
      .toBeInTheDocument()
    expect(bodies).toHaveLength(0)

    await userEvent.fill(
      screen.getByLabelText('Alasan'),
      'Ruang dipakai ujian.'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Tolak' }).last())

    await vi.waitFor(() =>
      expect(bodies).toEqual([{ reason: 'Ruang dipakai ujian.' }])
    )
    expect(toast.success).toHaveBeenCalledWith('Pengajuan Andi ditolak.')
  })

  it('shows what now occupies the slot when approval conflicts', async () => {
    useAuthStore.getState().auth.setSession('t', TU)
    api.defaults.adapter = mockAdapter(() => ({
      status: 409,
      data: apiErrorBody('BOOKING_CONFLICT', 'Ruang sudah terpakai.', [
        {
          kind: 'SCHEDULE',
          start_at: '2099-10-21T13:00:00+07:00',
          end_at: '2099-10-21T15:00:00+07:00',
        },
      ]),
    }))
    const screen = await renderActions()

    await userEvent.click(screen.getByRole('button', { name: 'Setujui' }))
    await userEvent.click(
      screen.getByRole('button', { name: 'Setujui' }).last()
    )

    await vi.waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith('Ruang sudah terpakai.', {
        description: expect.stringContaining('13:00–15:00'),
      })
    )
  })

  it('lets the owner cancel a pending request without a reason', async () => {
    useAuthStore.getState().auth.setSession('t', { ...sampleUser })
    const calls: { url?: string; body: unknown }[] = []
    api.defaults.adapter = mockAdapter((config) => {
      calls.push({ url: config.url, body: JSON.parse(String(config.data)) })
      return { status: 200, data: { ...booking, status: 'DIBATALKAN' } }
    })
    const screen = await renderActions()

    await expect
      .element(screen.getByRole('button', { name: 'Setujui' }))
      .not.toBeInTheDocument()
    await userEvent.click(
      screen.getByRole('button', { name: 'Batalkan pengajuan' })
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Batalkan', exact: true })
    )

    await vi.waitFor(() =>
      expect(calls).toEqual([
        { url: `/bookings/${booking.id}/cancel`, body: {} },
      ])
    )
  })

  it('renders nothing for Kaprodi', async () => {
    useAuthStore
      .getState()
      .auth.setSession('t', { ...sampleUser, id: 'k', role: 'KAPRODI' })
    const screen = await renderActions()

    expect(screen.container.querySelectorAll('button')).toHaveLength(0)
  })
})
