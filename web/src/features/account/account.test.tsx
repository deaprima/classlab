import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { Account } from '.'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    Link: ({
      to,
      children,
      ...props
    }: {
      to: string
      children: React.ReactNode
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

function serve(me: Record<string, unknown>) {
  api.defaults.adapter = mockAdapter(() => ({
    status: 200,
    data: { ...sampleUser, ...me },
  }))
}

function renderAccount(user: Record<string, unknown> = {}) {
  useAuthStore.getState().auth.setSession('token', { ...sampleUser, ...user })
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Account />
    </QueryClientProvider>
  )
}

describe('Account', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-10-21T10:00:00+07:00'))
  })

  afterEach(() => {
    vi.useRealTimers()
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('shows the profile with a link to change the password', async () => {
    serve({ active_points: 0 })
    const screen = await renderAccount()
    await expect.element(screen.getByText('Budi Santoso')).toBeVisible()
    await expect.element(screen.getByText('NIM')).toBeVisible()
    await expect.element(screen.getByText('220101001')).toBeVisible()
    await expect.element(screen.getByText('Mahasiswa')).toBeVisible()
    await expect.element(screen.getByText('Aktif')).toBeVisible()
    await expect
      .element(screen.getByRole('link', { name: 'Ganti Password' }))
      .toHaveAttribute('href', '/change-password')
  })

  it('shows the violation points against the blocking threshold', async () => {
    serve({ active_points: 2 })
    const screen = await renderAccount()
    await expect.element(screen.getByText('dari 6 poin')).toBeVisible()
    const bar = screen.getByRole('progressbar', { name: 'Poin pelanggaran' })
    await expect.element(bar).toHaveAttribute('aria-valuenow', '2')
    await expect.element(bar).toHaveAttribute('aria-valuemax', '6')
  })

  it('does not show points for a role that has none', async () => {
    serve({ role: 'TU', identifier: '198203152008012002' })
    const screen = await renderAccount({
      role: 'TU',
      identifier: '198203152008012002',
    })
    await expect.element(screen.getByText('NIP')).toBeVisible()
    await expect
      .element(screen.getByText('Poin Pelanggaran'))
      .not.toBeInTheDocument()
  })

  it('says until when booking is blocked', async () => {
    serve({
      active_points: 6,
      booking_blocked_until: '2026-11-04T08:00:00+07:00',
    })
    const screen = await renderAccount()
    await expect
      .element(screen.getByText(/Diblokir mengajukan sampai/))
      .toBeVisible()
  })

  it('shows the contact person of an organisation', async () => {
    const org = {
      role: 'ORGANISASI',
      name: 'HIMA Informatika',
      identifier: 'HIMA-TI',
      pic_name: 'Rizky Maulana',
      pic_phone: '081234567890',
      active_points: 1,
    }
    serve(org)
    const screen = await renderAccount(org)
    await expect.element(screen.getByText('Kode organisasi')).toBeVisible()
    await expect.element(screen.getByText('Rizky Maulana')).toBeVisible()
  })
})
