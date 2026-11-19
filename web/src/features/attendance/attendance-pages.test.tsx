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
import { makeBooking } from '@/features/my-bookings/fixtures'
import { CheckIn } from './check-in-page'
import { CheckOut } from './check-out-page'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    Link: ({ to, children }: { to: string; children: React.ReactNode }) => (
      <a href={to}>{children}</a>
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

// The camera itself has its own tests; here it just yields a photo.
vi.mock('@/components/camera-capture', () => ({
  CameraCapture: ({ onCapture }: { onCapture: (b: Blob) => void }) => (
    <button
      type='button'
      onClick={() => onCapture(new Blob(['x'], { type: 'image/jpeg' }))}
    >
      Ambil foto uji
    </button>
  ),
}))

const originalAdapter = api.defaults.adapter

function wrap(ui: React.ReactNode) {
  return render(
    <QueryClientProvider client={new QueryClient()}>{ui}</QueryClientProvider>
  )
}

beforeEach(() => {
  useAuthStore.getState().auth.setSession('token', { ...sampleUser })
})

afterEach(() => {
  api.defaults.adapter = originalAdapter
  useAuthStore.getState().auth.reset()
})

describe('CheckIn', () => {
  it('checks in once with the QR values and shows the previous check-out', async () => {
    const bodies: unknown[] = []
    api.defaults.adapter = mockAdapter((config) => {
      if (config.url === '/check-in') {
        bodies.push(JSON.parse(String(config.data)))
        return {
          status: 200,
          data: {
            booking: makeBooking({ status: 'BERLANGSUNG' }),
            previous_checkout: {
              checked_out_at: '2026-10-21T09:00:00+07:00',
              condition: 'ADA_MASALAH',
              attachment_id: 'att-1',
            },
          },
        }
      }
      return { status: 404 }
    })
    const screen = await wrap(<CheckIn room='R-401' token='tok' />)
    await expect.element(screen.getByText('Check-in berhasil')).toBeVisible()
    await expect.element(screen.getByText(/Ada kerusakan/)).toBeVisible()
    expect(bodies).toEqual([{ room_code: 'R-401', qr_token: 'tok' }])
    await expect
      .element(screen.getByRole('link', { name: 'Laporkan masalah' }))
      .toBeVisible()
  })

  it('explains a QR the server rejects', async () => {
    api.defaults.adapter = mockAdapter(() => ({
      status: 422,
      data: apiErrorBody('QR_INVALID', 'QR tidak valid'),
    }))
    const screen = await wrap(<CheckIn room='R-401' token='bad' />)
    await expect
      .element(screen.getByText('QR tidak valid').first())
      .toBeVisible()
  })

  it('tells when the next window opens', async () => {
    api.defaults.adapter = mockAdapter(() => ({
      status: 409,
      data: apiErrorBody('NO_ELIGIBLE_BOOKING', 'x', [
        { field: 'next_window.opens_at', message: '2026-10-21T12:45:00+07:00' },
        {
          field: 'next_window.closes_at',
          message: '2026-10-21T13:15:00+07:00',
        },
      ]),
    }))
    const screen = await wrap(<CheckIn room='R-401' token='tok' />)
    await expect
      .element(screen.getByText('Belum waktunya check-in'))
      .toBeVisible()
  })

  it('shows the not-owner state when nothing is eligible', async () => {
    api.defaults.adapter = mockAdapter(() => ({
      status: 409,
      data: apiErrorBody('NO_ELIGIBLE_BOOKING', 'x'),
    }))
    const screen = await wrap(<CheckIn room='R-401' token='tok' />)
    await expect
      .element(screen.getByText('Tidak ada booking untukmu di sini'))
      .toBeVisible()
  })

  it('does not call the API when the link has no QR values', async () => {
    const adapter = vi.fn(() => ({ status: 500 }))
    api.defaults.adapter = mockAdapter(adapter)
    const screen = await wrap(<CheckIn />)
    await expect
      .element(screen.getByText('QR tidak valid').first())
      .toBeVisible()
    expect(adapter).not.toHaveBeenCalled()
  })
})

describe('CheckOut', () => {
  function serve(booking: Booking) {
    const uploads: FormData[] = []
    api.defaults.adapter = mockAdapter((config) => {
      if (config.method === 'get') return { status: 200, data: booking }
      if (String(config.url).endsWith('/check-out')) {
        uploads.push(config.data as FormData)
        return {
          status: 200,
          data: { booking: { ...booking, status: 'SELESAI' }, report_id: null },
        }
      }
      return { status: 404 }
    })
    return uploads
  }

  it('refuses a booking that is not running', async () => {
    serve(makeBooking({ status: 'DISETUJUI' }))
    const screen = await wrap(<CheckOut bookingId='b1' />)
    await expect.element(screen.getByText('Belum bisa check-out')).toBeVisible()
  })

  it('asks for the condition and a photo before uploading', async () => {
    const uploads = serve(makeBooking({ status: 'BERLANGSUNG' }))
    const screen = await wrap(<CheckOut bookingId='b1' />)
    await userEvent.click(screen.getByRole('button', { name: 'Check-out' }))
    await expect.element(screen.getByText('Pilih kondisi ruang.')).toBeVisible()
    await expect
      .element(screen.getByText('Ambil foto kondisi ruang dulu.'))
      .toBeVisible()
    expect(uploads).toHaveLength(0)
  })

  it('requires a note when something is damaged', async () => {
    const uploads = serve(makeBooking({ status: 'BERLANGSUNG' }))
    const screen = await wrap(<CheckOut bookingId='b1' />)
    await userEvent.click(screen.getByRole('radio', { name: 'Ada kerusakan' }))
    await userEvent.click(
      screen.getByRole('button', { name: 'Ambil foto uji' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Check-out' }))
    await expect
      .element(screen.getByText('Jelaskan kerusakan yang kamu temukan.'))
      .toBeVisible()
    expect(uploads).toHaveLength(0)
  })

  it('uploads condition and photo as multipart', async () => {
    const uploads = serve(makeBooking({ status: 'BERLANGSUNG' }))
    const screen = await wrap(<CheckOut bookingId='b1' />)
    await userEvent.click(
      screen.getByRole('radio', { name: 'Bersih dan kondisi baik' })
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Ambil foto uji' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Check-out' }))
    await expect.element(screen.getByText('Check-out berhasil')).toBeVisible()
    expect(uploads).toHaveLength(1)
    expect(uploads[0].get('condition')).toBe('NORMAL')
    expect(uploads[0].get('photo')).toBeInstanceOf(Blob)
  })
})
