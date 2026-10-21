import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { type Booking } from '@/lib/api-types'
import { CancelDialog } from './cancel-dialog'
import { makeBooking, slotOn } from './fixtures'

const originalAdapter = api.defaults.adapter

function serve() {
  const calls: { url: string; body: Record<string, unknown> }[] = []
  api.defaults.adapter = mockAdapter((config) => {
    if (config.method === 'post' && String(config.url).endsWith('/cancel')) {
      calls.push({
        url: String(config.url),
        body: JSON.parse(String(config.data ?? '{}')),
      })
      return { status: 200, data: makeBooking({ status: 'DIBATALKAN' }) }
    }
    return { status: 404 }
  })
  return calls
}

async function renderDialog(booking: Booking) {
  const onOpenChange = vi.fn()
  const screen = await render(
    <QueryClientProvider client={new QueryClient()}>
      <CancelDialog booking={booking} onOpenChange={onOpenChange} />
    </QueryClientProvider>
  )
  await expect.element(screen.getByRole('dialog')).toBeVisible()
  return { screen, onOpenChange }
}

const group = (over: Partial<Booking> = {}) =>
  makeBooking({
    group_id: 'g1',
    occurrence_no: 1,
    occurrence_count: 3,
    ...over,
  })

describe('CancelDialog', () => {
  beforeEach(() => {
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
  })

  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('cancels a single booking without asking for a scope', async () => {
    const calls = serve()
    const { screen, onOpenChange } = await renderDialog(makeBooking())
    await expect.element(screen.getByRole('radiogroup')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Batalkan' }))
    await vi.waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(calls).toHaveLength(1)
    expect(calls[0].body).toEqual({})
  })

  it('offers this date or the whole group, this date by default', async () => {
    const calls = serve()
    const { screen } = await renderDialog(group())
    await expect
      .element(screen.getByRole('radio', { name: /Kejadian ini saja/ }))
      .toBeChecked()
    await expect
      .element(screen.getByRole('radio', { name: 'Seluruh grup (3 tanggal)' }))
      .not.toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: 'Batalkan' }))
    await vi.waitFor(() => expect(calls).toHaveLength(1))
    expect(calls[0].body).toEqual({ scope: 'OCCURRENCE' })
  })

  it('cancels every date when the whole group is chosen', async () => {
    const calls = serve()
    const { screen } = await renderDialog(group())
    await userEvent.click(
      screen.getByRole('radio', { name: 'Seluruh grup (3 tanggal)' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Batalkan' }))
    await vi.waitFor(() => expect(calls).toHaveLength(1))
    expect(calls[0].body).toEqual({ scope: 'GROUP' })
  })

  it('warns that a late cancel of an approved booking costs a point', async () => {
    serve()
    const soon = new Date(Date.now() + 60 * 60_000)
    const { screen } = await renderDialog(
      makeBooking({
        status: 'DISETUJUI',
        start_at: soon.toISOString(),
        end_at: new Date(soon.getTime() + 2 * 3600_000).toISOString(),
      })
    )
    await expect
      .element(screen.getByText('Batal terlambat kena poin'))
      .toBeVisible()
    await expect.element(screen.getByText(/menambah 1 poin/)).toBeVisible()
  })

  it('gives no warning when there is time, or for a waiting request', async () => {
    serve()
    const early = await renderDialog(
      makeBooking({ status: 'DISETUJUI', ...slotOn(3, 10, 12) })
    )
    await expect
      .element(early.screen.getByText('Batal terlambat kena poin'))
      .not.toBeInTheDocument()
  })

  it('closes with Kembali without calling the API', async () => {
    const calls = serve()
    const { screen, onOpenChange } = await renderDialog(makeBooking())
    await userEvent.click(screen.getByRole('button', { name: 'Kembali' }))
    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(calls).toHaveLength(0)
  })
})
