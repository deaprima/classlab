import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { api } from '@/lib/api-client'
import { type User } from '@/lib/api-types'
import { ViolationsDialog } from './violations-dialog'

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
vi.mock('sonner', () => ({ toast }))

const originalAdapter = api.defaults.adapter

const USER = {
  ...sampleUser,
  id: 'u1',
  name: 'Fajar R.',
  identifier: '2023010045',
  role: 'MAHASISWA',
} as User

const violation = (id: string, over: Record<string, unknown> = {}) => ({
  id,
  booking_id: `b-${id}`,
  kind: 'TIDAK_HADIR',
  points: 2,
  active: true,
  annulled_at: null,
  annulled_by: null,
  annul_reason: null,
  created_at: '2026-10-12T03:00:00Z',
  room_code: 'R-402',
  ...over,
})

type Call = { method: string; url: string; body?: Record<string, unknown> }

function serve(blockedUntil: string | null) {
  const calls: Call[] = []
  api.defaults.adapter = mockAdapter((config) => {
    const url = String(config.url)
    const method = String(config.method)
    calls.push({
      method,
      url,
      body: config.data ? JSON.parse(String(config.data)) : undefined,
    })
    if (url === '/users/u1/violations') {
      return {
        status: 200,
        data: {
          data: [
            violation('v1'),
            violation('v2', {
              kind: 'TANPA_CHECKOUT',
              points: 1,
              active: false,
              annulled_at: '2026-10-13T03:00:00Z',
              annul_reason: 'Salah catat',
              room_code: 'R-401',
            }),
          ],
          meta: { page: 1, per_page: 50, total: 2 },
          active_points: 6,
          booking_blocked_until: blockedUntil,
        },
      }
    }
    if (url === '/violations/v1/annul') {
      return { status: 200, data: violation('v1', { active: false }) }
    }
    if (url === '/users/u1' && method === 'patch') {
      return { status: 200, data: USER }
    }
    return { status: 404 }
  })
  return calls
}

const future = () => new Date(Date.now() + 5 * 86_400_000).toISOString()

function renderDialog() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <ViolationsDialog user={USER} onClose={vi.fn()} />
    </QueryClientProvider>
  )
}

describe('ViolationsDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
  })

  it('shows the block, the points against the threshold and each violation', async () => {
    serve(future())
    const screen = await renderDialog()

    await expect.element(screen.getByText(/Diblokir sampai/)).toBeVisible()
    await expect.element(screen.getByText('6 / 6')).toBeVisible()
    await expect.element(screen.getByText('Tidak hadir · R-402')).toBeVisible()
    await expect
      .element(screen.getByText(/di-annul: Salah catat/))
      .toBeVisible()
    // Only the active violation can be annulled.
    expect(
      screen.getByRole('button', { name: /^Annul/ }).elements()
    ).toHaveLength(1)
  })

  it('annuls a violation only with a reason of at least 5 characters', async () => {
    const calls = serve(null)
    const screen = await renderDialog()

    await userEvent.click(screen.getByRole('button', { name: /^Annul/ }))
    const dialog = screen.getByRole('dialog').last()
    await userEvent.fill(dialog.getByLabelText(/Alasan annul/), 'tes')
    await userEvent.click(dialog.getByRole('button', { name: 'Annul' }))
    await expect
      .element(screen.getByText(/minimal 5 karakter/i).first())
      .toBeVisible()
    expect(calls.some((c) => c.url === '/violations/v1/annul')).toBe(false)

    await userEvent.fill(
      dialog.getByLabelText(/Alasan annul/),
      'Kendala jaringan kampus'
    )
    await userEvent.click(dialog.getByRole('button', { name: 'Annul' }))

    await vi.waitFor(() =>
      expect(calls.find((c) => c.url === '/violations/v1/annul')?.body).toEqual(
        {
          reason: 'Kendala jaringan kampus',
        }
      )
    )
  })

  it('lifts the block with clear_booking_block after confirming', async () => {
    const calls = serve(future())
    const screen = await renderDialog()

    await userEvent.click(screen.getByRole('button', { name: 'Buka Blokir' }))
    await userEvent.click(
      screen
        .getByRole('alertdialog')
        .getByRole('button', { name: 'Buka Blokir' })
    )

    await vi.waitFor(() =>
      expect(
        calls.find((c) => c.method === 'patch' && c.url === '/users/u1')?.body
      ).toEqual({ clear_booking_block: true })
    )
  })

  it('offers no Buka Blokir when nothing is blocked', async () => {
    serve(null)
    const screen = await renderDialog()

    await expect.element(screen.getByText('Tidak hadir · R-402')).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Buka Blokir' }).elements()
    ).toEqual([])
  })
})
