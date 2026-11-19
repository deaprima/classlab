import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { api } from '@/lib/api-client'
import { type Room } from '@/lib/api-types'
import { RoomQrDialog } from './qr-dialog'

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
vi.mock('sonner', () => ({ toast }))

const originalAdapter = api.defaults.adapter
const ROOM: Room = {
  id: '0b6a3a8e-1d2c-4c7e-9f0a-1234567890ab',
  org_unit_id: sampleUser.org_unit_id,
  code: 'R-301',
  name: 'Ruang 301',
  type: 'KELAS',
  capacity: 40,
  facilities: [],
  is_active: true,
  open_time: null,
  close_time: null,
}
const QR = (token: string) => ({
  url: `https://classlab.test/check-in?room=R-301&t=${token}`,
  qr_token: token,
  rotated_at: '2026-10-22T03:00:00Z',
})

describe('RoomQrDialog', () => {
  afterEach(() => {
    api.defaults.adapter = originalAdapter
  })

  it('draws the QR of the room, then replaces the token after confirming', async () => {
    const calls: string[] = []
    api.defaults.adapter = mockAdapter((config) => {
      calls.push(`${config.method} ${config.url}`)
      return {
        status: 200,
        data: QR(config.method === 'post' ? 'new-token' : 'old-token'),
      }
    })
    const screen = await render(
      <QueryClientProvider client={new QueryClient()}>
        <RoomQrDialog room={ROOM} onOpenChange={vi.fn()} />
      </QueryClientProvider>
    )

    await expect
      .element(screen.getByRole('img', { name: /QR check-in ruang R-301/ }))
      .toBeInTheDocument()
    expect(calls).toEqual([`get /rooms/${ROOM.id}/qr`])

    await userEvent.click(screen.getByRole('button', { name: 'Ganti token' }))
    await userEvent.click(
      screen
        .getByRole('alertdialog')
        .getByRole('button', { name: 'Ganti token' })
    )

    await vi.waitFor(() =>
      expect(calls).toContain(`post /rooms/${ROOM.id}/qr/rotate`)
    )
    await vi.waitFor(() => expect(toast.success).toHaveBeenCalled())
  })

  it('shows the server error when the QR cannot be read', async () => {
    api.defaults.adapter = mockAdapter(() => ({
      status: 403,
      data: {
        error: { code: 'FORBIDDEN', message: 'Tidak boleh.', details: [] },
      },
    }))
    const screen = await render(
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <RoomQrDialog room={ROOM} onOpenChange={vi.fn()} />
      </QueryClientProvider>
    )

    await expect.element(screen.getByText('Tidak boleh.')).toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: 'Cetak' }))
      .toBeDisabled()
  })
})
