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
import { EditDialog } from './edit-dialog'
import { makeBooking, POLICIES, slotOn } from './fixtures'

const originalAdapter = api.defaults.adapter

type Handler = (body: Record<string, unknown>) => {
  status: number
  data?: unknown
}

function serve(onPatch: Handler = () => ({ status: 500 })) {
  const calls: Record<string, unknown>[] = []
  api.defaults.adapter = mockAdapter((config) => {
    if (config.url === '/room-types') {
      return { status: 200, data: { data: POLICIES } }
    }
    if (config.method === 'patch') {
      const body = JSON.parse(String(config.data)) as Record<string, unknown>
      calls.push(body)
      return onPatch(body)
    }
    return { status: 404 }
  })
  return calls
}

async function renderDialog(booking: Booking) {
  const onOpenChange = vi.fn()
  const screen = await render(
    <QueryClientProvider client={new QueryClient()}>
      <EditDialog booking={booking} onOpenChange={onOpenChange} />
    </QueryClientProvider>
  )
  await expect.element(screen.getByRole('dialog')).toBeVisible()
  return { screen, onOpenChange }
}

describe('EditDialog', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
  })

  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('starts from the booking and says the room cannot change', async () => {
    serve()
    const { screen } = await renderDialog(makeBooking())
    await expect
      .element(screen.getByText(/Ruang tidak bisa diganti/))
      .toBeVisible()
    await expect
      .element(screen.getByLabelText('Nama kegiatan'))
      .toHaveValue('Praktikum Basis Data')
    await expect
      .element(screen.getByRole('combobox', { name: 'Mulai' }))
      .toHaveTextContent('10:00')
  })

  it('warns that an approved booking may need approval again, but not a waiting one', async () => {
    serve()
    const approved = await renderDialog(makeBooking({ status: 'DISETUJUI' }))
    await expect
      .element(approved.screen.getByText('Perlu persetujuan ulang'))
      .toBeVisible()
  })

  it('shows no re-approval warning for a waiting request', async () => {
    serve()
    const { screen } = await renderDialog(makeBooking({ status: 'MENUNGGU' }))
    await expect
      .element(screen.getByText('Perlu persetujuan ulang'))
      .not.toBeInTheDocument()
  })

  it('closes without calling the API when nothing changed', async () => {
    const calls = serve()
    const { screen, onOpenChange } = await renderDialog(makeBooking())
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))
    await vi.waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(calls).toHaveLength(0)
  })

  it('sends only the changed field', async () => {
    const calls = serve(() => ({ status: 200, data: makeBooking() }))
    const { screen, onOpenChange } = await renderDialog(makeBooking())
    await userEvent.fill(
      screen.getByLabelText('Nama kegiatan'),
      'Praktikum Jaringan'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))
    await vi.waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(calls).toEqual([{ purpose: 'Praktikum Jaringan' }])
  })

  it('sends both times when one of them changes', async () => {
    const booking = makeBooking()
    const calls = serve(() => ({ status: 200, data: booking }))
    const { screen } = await renderDialog(booking)
    await userEvent.click(screen.getByRole('combobox', { name: 'Selesai' }))
    await userEvent.click(screen.getByRole('option', { name: '13:00' }))
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))
    const { start_at, end_at } = slotOn(2, 10, 13)
    await vi.waitFor(() => expect(calls).toHaveLength(1))
    expect(calls[0]).toEqual({ start_at, end_at })
  })

  it('does not send an invalid change', async () => {
    const calls = serve()
    const { screen } = await renderDialog(makeBooking())
    await userEvent.fill(screen.getByLabelText('Jumlah peserta'), '99')
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))
    await expect
      .element(screen.getByText('Melebihi kapasitas ruang (32 orang).'))
      .toBeVisible()
    expect(calls).toHaveLength(0)
  })

  it('explains a clash with the clashing time', async () => {
    serve(() => ({
      status: 409,
      data: apiErrorBody(
        'BOOKING_CONFLICT',
        'Bentrok: LAB-RPL-02 sudah terisi.',
        [
          {
            kind: 'SCHEDULE',
            title: 'Kuliah Kalkulus',
            ...slotOn(2, 12, 13),
          },
        ]
      ),
    }))
    const { screen, onOpenChange } = await renderDialog(makeBooking())
    await userEvent.fill(screen.getByLabelText('Nama kegiatan'), 'Praktikum')
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))
    await expect
      .element(screen.getByText('Bentrok: LAB-RPL-02 sudah terisi.'))
      .toBeVisible()
    await expect.element(screen.getByText(/12:00 – 13:00/)).toBeVisible()
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  it('shows the server message when the edit window has closed', async () => {
    serve(() => ({
      status: 409,
      data: apiErrorBody('EDIT_WINDOW_CLOSED', 'Batas ubah sudah lewat.'),
    }))
    const { screen } = await renderDialog(makeBooking())
    await userEvent.fill(screen.getByLabelText('Nama kegiatan'), 'Praktikum')
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))
    await expect
      .element(screen.getByText('Batas ubah sudah lewat.'))
      .toBeVisible()
  })

  it('puts a field error from the server on its field', async () => {
    serve(() => ({
      status: 422,
      data: apiErrorBody('VALIDATION_ERROR', 'Data tidak valid.', [
        { field: 'purpose', message: 'Nama kegiatan terlalu pendek.' },
      ]),
    }))
    const { screen } = await renderDialog(makeBooking())
    await userEvent.fill(screen.getByLabelText('Nama kegiatan'), 'Pr')
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))
    await expect
      .element(screen.getByText('Nama kegiatan terlalu pendek.'))
      .toBeVisible()
  })
})
