import { type InternalAxiosRequestConfig } from 'axios'
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
import { addDays, todayWib } from '@/lib/datetime'
import { BookingForm } from './booking-form'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    Link: ({ children }: { children: React.ReactNode }) => (
      <a href='/'>{children}</a>
    ),
  }
})

const ROOM = {
  id: '0b6a3a8e-1d2c-4c7e-9f0a-1234567890ab',
  org_unit_id: sampleUser.org_unit_id,
  code: 'LAB-1',
  name: 'Lab Jaringan',
  type: 'LAB',
  capacity: 30,
  facilities: [],
  is_active: true,
}
const tomorrow = addDays(todayWib(), 1)
const originalAdapter = api.defaults.adapter

type BookingReply = (config: InternalAxiosRequestConfig) => {
  status: number
  data?: unknown
}

function serve(onBooking: BookingReply) {
  api.defaults.adapter = mockAdapter((config) => {
    if (config.url === '/rooms') {
      return {
        status: 200,
        data: { data: [ROOM], meta: { page: 1, per_page: 100, total: 1 } },
      }
    }
    if (config.url === '/availability') {
      return {
        status: 200,
        data: { data: [{ room: ROOM, status: 'TERSEDIA', conflicts: [] }] },
      }
    }
    if (config.url === '/bookings' && config.method === 'post') {
      return onBooking(config)
    }
    return { status: 404 }
  })
}

async function renderForm(onCreated = vi.fn()) {
  const screen = await render(
    <QueryClientProvider client={new QueryClient()}>
      <BookingForm
        initial={{
          room_id: ROOM.id,
          date: tomorrow,
          start_time: '08:00',
          end_time: '10:00',
        }}
        onCreated={onCreated}
      />
    </QueryClientProvider>
  )
  return { screen, onCreated }
}

describe('BookingForm', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
  })

  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('shows the live availability of the chosen slot', async () => {
    serve(() => ({ status: 500 }))
    const { screen } = await renderForm()

    await expect
      .element(screen.getByText('Ruang tersedia pada jam ini.'))
      .toBeInTheDocument()
  })

  it('submits WIB timestamps with an Idempotency-Key and reuses it on retry', async () => {
    const keys: string[] = []
    const bodies: unknown[] = []
    serve((config) => {
      keys.push(String(config.headers.get('Idempotency-Key')))
      bodies.push(JSON.parse(String(config.data)))
      return {
        status: 503,
        data: apiErrorBody('SERVICE_BUSY', 'Server sibuk, coba lagi.'),
      }
    })
    const { screen } = await renderForm()

    await userEvent.fill(screen.getByLabelText('Keperluan'), 'Latihan kelompok')
    await userEvent.click(
      screen.getByRole('button', { name: 'Kirim Pengajuan' })
    )
    await expect
      .element(screen.getByText('Server sibuk, coba lagi.'))
      .toBeInTheDocument()
    await userEvent.click(
      screen.getByRole('button', { name: 'Kirim Pengajuan' })
    )
    await vi.waitFor(() => expect(keys).toHaveLength(2))

    expect(bodies[0]).toEqual({
      room_id: ROOM.id,
      start_at: `${tomorrow}T08:00:00+07:00`,
      end_at: `${tomorrow}T10:00:00+07:00`,
      purpose: 'Latihan kelompok',
    })
    expect(keys[0]).toMatch(/^[0-9a-f-]{36}$/)
    expect(keys[1]).toBe(keys[0])
  })

  it('explains a 409 conflict with the clashing time', async () => {
    serve(() => ({
      status: 409,
      data: apiErrorBody(
        'BOOKING_CONFLICT',
        'Ruang sudah terpakai pada rentang waktu tersebut.',
        [
          {
            kind: 'BOOKING',
            status: 'DISETUJUI',
            start_at: `${tomorrow}T09:00:00+07:00`,
            end_at: `${tomorrow}T11:00:00+07:00`,
          },
        ]
      ),
    }))
    const { screen, onCreated } = await renderForm()

    await userEvent.fill(screen.getByLabelText('Keperluan'), 'Latihan kelompok')
    await userEvent.click(
      screen.getByRole('button', { name: 'Kirim Pengajuan' })
    )

    await expect.element(screen.getByText('Jadwal bentrok')).toBeInTheDocument()
    await expect
      .element(
        screen.getByText('Ruang sudah terpakai pada rentang waktu tersebut.')
      )
      .toBeInTheDocument()
    await expect.element(screen.getByText(/09:00–11:00/)).toBeInTheDocument()
    await expect
      .element(screen.getByText(/Peminjaman disetujui/))
      .toBeInTheDocument()
    expect(onCreated).not.toHaveBeenCalled()
  })

  it('puts server field errors on the matching fields', async () => {
    serve(() => ({
      status: 422,
      data: apiErrorBody('VALIDATION_ERROR', 'Data tidak valid.', [
        { field: 'start_at', message: 'start_at tidak boleh di masa lalu.' },
      ]),
    }))
    const { screen } = await renderForm()

    await userEvent.fill(screen.getByLabelText('Keperluan'), 'Latihan kelompok')
    await userEvent.click(
      screen.getByRole('button', { name: 'Kirim Pengajuan' })
    )

    await expect
      .element(screen.getByText('start_at tidak boleh di masa lalu.'))
      .toBeInTheDocument()
  })

  it('shows the quota error, which has no field', async () => {
    serve(() => ({
      status: 422,
      data: apiErrorBody('VALIDATION_ERROR', 'Data tidak valid.', [
        { message: 'Anda sudah memiliki 5 pengajuan aktif.' },
      ]),
    }))
    const { screen } = await renderForm()

    await userEvent.fill(screen.getByLabelText('Keperluan'), 'Latihan kelompok')
    await userEvent.click(
      screen.getByRole('button', { name: 'Kirim Pengajuan' })
    )

    await expect
      .element(screen.getByText('Anda sudah memiliki 5 pengajuan aktif.'))
      .toBeInTheDocument()
  })

  it('validates locally before calling the API', async () => {
    const post = vi.fn(() => ({ status: 201 }))
    serve(post)
    const { screen } = await renderForm()

    await userEvent.click(
      screen.getByRole('button', { name: 'Kirim Pengajuan' })
    )

    await expect
      .element(screen.getByText('Keperluan wajib diisi.'))
      .toBeInTheDocument()
    expect(post).not.toHaveBeenCalled()
  })

  it('reports the created booking', async () => {
    serve((config) => ({
      status: 201,
      data: {
        id: 'b1',
        status: 'MENUNGGU',
        room: ROOM,
        ...JSON.parse(String(config.data)),
      },
    }))
    const { screen, onCreated } = await renderForm()

    await userEvent.fill(screen.getByLabelText('Keperluan'), 'Latihan kelompok')
    await userEvent.click(
      screen.getByRole('button', { name: 'Kirim Pengajuan' })
    )

    await vi.waitFor(() => expect(onCreated).toHaveBeenCalledOnce())
    expect(onCreated.mock.calls[0][0]).toMatchObject({
      id: 'b1',
      status: 'MENUNGGU',
    })
  })
})
