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
import { BookingDetailDialog } from './detail-dialog'
import { makeBooking, POLICIES, slotOn } from './fixtures'

const mocks = vi.hoisted(() => ({ id: 'b1' }))

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

const onEdit = vi.fn()
const onCancel = vi.fn()
const onOpenChange = vi.fn()

function renderPage(withChanges = true) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <BookingDetailDialog
        id={mocks.id}
        onOpenChange={onOpenChange}
        onEdit={withChanges ? onEdit : undefined}
        onCancel={withChanges ? onCancel : undefined}
      />
    </QueryClientProvider>
  )
}

describe('BookingDetailDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
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
      .element(screen.getByRole('heading', { name: 'Detail Pengajuan' }))
      .toBeVisible()
    await expect.element(screen.getByText('LAB-RPL-02').first()).toBeVisible()
    await expect
      .element(screen.getByText('Menunggu validasi Pengurus Lab'))
      .toBeVisible()
    await expect
      .element(screen.getByText(/Kedaluwarsa otomatis dalam 22 jam/))
      .toBeVisible()
    await expect.element(screen.getByText('Pengajuan dikirim')).toBeVisible()
    await expect
      .element(screen.getByText('Ditinjau Pengurus Lab'))
      .toBeVisible()
  })

  it('hands edit and cancel to the page and closes itself', async () => {
    serve(makeBooking({ id: 'b1' }))
    const screen = await renderPage()
    await userEvent.click(
      screen.getByRole('button', { name: 'Batalkan Pengajuan' })
    )
    expect(onCancel).toHaveBeenCalledWith(expect.objectContaining({ id: 'b1' }))
    expect(onOpenChange).toHaveBeenCalledWith(false)
    await userEvent.click(
      screen.getByRole('button', { name: 'Ubah Pengajuan' })
    )
    expect(onEdit).toHaveBeenCalledWith(expect.objectContaining({ id: 'b1' }))
  })

  it('only shows the booking when no actions are given', async () => {
    serve(makeBooking({ id: 'b1' }))
    const screen = await renderPage(false)
    await expect.element(screen.getByText('LAB-RPL-02').first()).toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Ubah Pengajuan' }))
      .not.toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: 'Tutup' }).first())
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
