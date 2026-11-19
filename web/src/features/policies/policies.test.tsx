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
import { POLICIES } from '@/features/my-bookings/fixtures'
import { Policies } from '.'
import { policyFormSchema } from './schema'

vi.mock('@/components/layout/page', () => ({
  Page: ({ title, children }: { title: string; children: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}))
vi.mock('@/hooks/use-mobile', () => ({ useIsMobile: () => false }))
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
vi.mock('sonner', () => ({ toast }))

const originalAdapter = api.defaults.adapter

function serve(patch?: () => { status: number; data?: unknown }) {
  const bodies: Record<string, unknown>[] = []
  api.defaults.adapter = mockAdapter((config) => {
    const url = String(config.url)
    if (url === '/room-types') return { status: 200, data: { data: POLICIES } }
    if (url.startsWith('/room-types/') && config.method === 'patch') {
      bodies.push(JSON.parse(String(config.data)))
      return patch?.() ?? { status: 200, data: { ...POLICIES[1] } }
    }
    return { status: 404 }
  })
  return bodies
}

function renderPage() {
  useAuthStore.getState().auth.setSession('t', { ...sampleUser, role: 'ADMIN' })
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Policies />
    </QueryClientProvider>
  )
}

describe('Kebijakan Tipe Ruang', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('lists each room type with approvers, default hours and approval mode', async () => {
    serve()
    const screen = await renderPage()

    await expect.element(screen.getByText('Ruang Kelas').first()).toBeVisible()
    await expect.element(screen.getByText('Laboratorium').first()).toBeVisible()
    await expect
      .element(screen.getByText('Otomatis', { exact: true }))
      .toBeVisible()
    await expect
      .element(screen.getByText('Manual', { exact: true }))
      .toBeVisible()
    await expect.element(screen.getByText('07:00–21:00').first()).toBeVisible()
  })

  it('saves the edited policy of one type', async () => {
    const bodies = serve()
    const screen = await renderPage()

    await userEvent.click(
      screen.getByRole('button', { name: 'Ubah kebijakan Ruang Kelas' })
    )
    await expect.element(screen.getByRole('dialog')).toBeVisible()
    await userEvent.fill(screen.getByLabelText('Maks hari ke depan'), '14')
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))

    await vi.waitFor(() => expect(bodies).toHaveLength(1))
    expect(bodies[0]).toMatchObject({
      approver_roles: ['TU'],
      approval_policy: 'AUTO_IF_ELIGIBLE',
      max_advance_days: 14,
      open_time: '07:00',
      close_time: '21:00',
    })
    await vi.waitFor(() => expect(toast.success).toHaveBeenCalled())
  })

  it('refuses a policy without any approver before sending', async () => {
    const bodies = serve()
    const screen = await renderPage()

    await userEvent.click(
      screen.getByRole('button', { name: 'Ubah kebijakan Ruang Kelas' })
    )
    await userEvent.click(screen.getByRole('checkbox', { name: 'TU' }))
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))

    await expect
      .element(screen.getByText('Pilih minimal satu approver.'))
      .toBeVisible()
    expect(bodies).toEqual([])
  })

  it('shows a server validation error on its field', async () => {
    serve(() => ({
      status: 422,
      data: apiErrorBody('VALIDATION_ERROR', 'Data tidak valid.', [
        { field: 'max_advance_days', message: 'Terlalu besar.' },
      ]),
    }))
    const screen = await renderPage()

    await userEvent.click(
      screen.getByRole('button', { name: 'Ubah kebijakan Ruang Kelas' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))

    await expect.element(screen.getByText('Terlalu besar.')).toBeVisible()
  })
})

describe('policy form rules', () => {
  const base = {
    approver_roles: ['TU'],
    requester_roles: ['MAHASISWA', 'ORGANISASI'],
    approval_policy: 'AUTO_IF_ELIGIBLE',
    letter_required: false,
    open_time: '07:00',
    close_time: '21:00',
    min_duration_min: 30,
    max_duration_min: 480,
    max_advance_days: '7',
  }
  const ok = (over: object) =>
    policyFormSchema.safeParse({ ...base, ...over }).success

  it('accepts the saved shape and the midnight closing time', () => {
    expect(ok({})).toBe(true)
    expect(ok({ close_time: '24:00' })).toBe(true)
  })

  it('rejects hours off the half hour or in the wrong order', () => {
    expect(ok({ open_time: '07:15' })).toBe(false)
    expect(ok({ close_time: '07:00' })).toBe(false)
    expect(ok({ open_time: '24:00' })).toBe(false)
  })

  it('rejects a minimum above the maximum, and empty role lists', () => {
    expect(ok({ min_duration_min: 600 })).toBe(false)
    expect(ok({ approver_roles: [] })).toBe(false)
    expect(ok({ requester_roles: [] })).toBe(false)
  })

  it('takes advance days from 1 to 365', () => {
    expect(ok({ max_advance_days: '0' })).toBe(false)
    expect(ok({ max_advance_days: '366' })).toBe(false)
    expect(ok({ max_advance_days: '30' })).toBe(true)
  })
})
