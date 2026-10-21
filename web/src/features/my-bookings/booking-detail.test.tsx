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
import { type Booking, type BookingEvent } from '@/lib/api-types'
import { BookingDetailPage } from './booking-detail-page'
import { makeBooking, POLICIES, slotOn } from './fixtures'

const mocks = vi.hoisted(() => ({ id: 'b1' }))

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    getRouteApi: () => ({ useParams: () => ({ id: mocks.id }) }),
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

vi.mock('@/components/layout/page', () => ({
  Page: ({ title, children }: { title: string; children: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}))

const originalAdapter = api.defaults.adapter

function serve(
  booking: Booking | 'missing',
  others: Booking[] = [],
  events: BookingEvent[] = []
) {
  api.defaults.adapter = mockAdapter((config) => {
    if (config.url === '/room-types') {
      return { status: 200, data: { data: POLICIES } }
    }
    if (config.url === `/bookings/${mocks.id}/events`) {
      return { status: 200, data: { data: events } }
    }
    if (config.url === `/bookings/${mocks.id}`) {
      return booking === 'missing'
        ? { status: 404, data: apiErrorBody('NOT_FOUND', 'Tidak ada.') }
        : { status: 200, data: booking }
    }
    if (config.url === '/bookings') {
      return {
        status: 200,
        data: {
          data: others,
          meta: { page: 1, per_page: 100, total: others.length },
        },
      }
    }
    return { status: 404 }
  })
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <BookingDetailPage />
    </QueryClientProvider>
  )
}

describe('BookingDetailPage', () => {
  beforeEach(() => {
    mocks.id = 'b1'
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
  })

  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('shows a waiting request with who validates it and the steps', async () => {
    serve(
      makeBooking({
        id: 'b1',
        expires_at: new Date(Date.now() + 23 * 3600_000).toISOString(),
      })
    )
    const screen = await renderPage()
    await expect
      .element(screen.getByRole('heading', { name: 'Status Pengajuan' }))
      .toBeVisible()
    await expect.element(screen.getByText('LAB-RPL-02').first()).toBeVisible()
    await expect
      .element(screen.getByText('Menunggu validasi Kepala Lab'))
      .toBeVisible()
    await expect
      .element(screen.getByText(/Kedaluwarsa otomatis dalam 22 jam/))
      .toBeVisible()
    await expect.element(screen.getByText('Pengajuan dikirim')).toBeVisible()
    await expect.element(screen.getByText('Ditinjau Kepala Lab')).toBeVisible()
  })

  it('offers edit and cancel to the owner', async () => {
    serve(makeBooking({ id: 'b1' }))
    const screen = await renderPage()
    await expect
      .element(screen.getByRole('button', { name: 'Ubah Pengajuan' }))
      .toBeVisible()
    await userEvent.click(
      screen.getByRole('button', { name: 'Batalkan Pengajuan' })
    )
    await expect
      .element(screen.getByRole('heading', { name: 'Batalkan pengajuan?' }))
      .toBeVisible()
  })

  it('offers nothing to someone who does not own it', async () => {
    serve(makeBooking({ id: 'b1', requester_id: 'someone-else' }))
    const screen = await renderPage()
    await expect.element(screen.getByText('LAB-RPL-02').first()).toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Ubah Pengajuan' }))
      .not.toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: 'Batalkan Pengajuan' }))
      .not.toBeInTheDocument()
  })

  it('offers no change once the booking is over', async () => {
    serve(makeBooking({ id: 'b1', status: 'SELESAI' }))
    const screen = await renderPage()
    await expect.element(screen.getByText('Selesai').first()).toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Ubah Pengajuan' }))
      .not.toBeInTheDocument()
  })

  it('shows the reason of a rejection in the error tone', async () => {
    serve(
      makeBooking({
        id: 'b1',
        status: 'DITOLAK',
        decision_reason: 'Ruang dipakai sidang skripsi.',
      })
    )
    const screen = await renderPage()
    await expect
      .element(screen.getByText('Ruang dipakai sidang skripsi.').first())
      .toBeVisible()
  })

  it('lists every date of a multi-date request with its own edit and cancel', async () => {
    const dates = [2, 9, 16].map((d, i) =>
      makeBooking({
        id: i === 0 ? 'b1' : `b${i + 1}`,
        group_id: 'g1',
        occurrence_no: i + 1,
        occurrence_count: 3,
        ...slotOn(d, 10, 12),
      })
    )
    serve(dates[0], dates)
    const screen = await renderPage()
    await expect.element(screen.getByText('Kejadian (3 tanggal)')).toBeVisible()
    await expect
      .element(
        screen.getByText(/Approve atau tolak berlaku untuk seluruh grup/)
      )
      .toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: /^Batalkan [A-Z][a-z]{2},/ }))
      .toHaveLength(3)
  })

  it('lists edits in the steps', async () => {
    serve(
      makeBooking({ id: 'b1' }),
      [],
      [
        {
          id: 'e2',
          booking_id: 'b1',
          actor_id: sampleUser.id,
          kind: 'EDITED',
          to_status: 'MENUNGGU',
          created_at: '2026-10-20T08:00:00+07:00',
          changes: { participants: { from: 20, to: 32 } },
        } as BookingEvent,
      ]
    )
    const screen = await renderPage()
    await expect.element(screen.getByText('Pengajuan diubah')).toBeVisible()
    await expect.element(screen.getByText('Diubah: peserta')).toBeVisible()
  })

  it('says so when the booking does not exist', async () => {
    serve('missing')
    const screen = await renderPage()
    await expect
      .element(screen.getByText('Pengajuan tidak ditemukan.'))
      .toBeVisible()
  })
})
