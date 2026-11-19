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
  Page: ({
    title,
    actions,
    children,
  }: {
    title: string
    actions?: React.ReactNode
    children: React.ReactNode
  }) => (
    <div>
      <h1>{title}</h1>
      {actions}
      {children}
    </div>
  ),
}))

const originalAdapter = api.defaults.adapter

const VIOLATIONS = [
  {
    id: 'v1',
    booking_id: 'b1',
    kind: 'BATAL_TERLAMBAT',
    points: 1,
    active: true,
    created_at: '2026-10-12T08:00:00+07:00',
    room_code: 'R-302',
    booking_start_at: '2026-10-12T10:00:00+07:00',
  },
  {
    id: 'v2',
    booking_id: 'b2',
    kind: 'TANPA_CHECKOUT',
    points: 1,
    active: false,
    created_at: '2026-08-03T08:00:00+07:00',
    room_code: 'LAB-JK-01',
    booking_start_at: '2026-08-03T10:00:00+07:00',
  },
]

function serve(me: Record<string, unknown>, violations = VIOLATIONS) {
  api.defaults.adapter = mockAdapter((config) =>
    String(config.url).endsWith('/violations')
      ? {
          status: 200,
          data: {
            data: violations,
            meta: { page: 1, per_page: 5, total: violations.length },
            active_points: 1,
            booking_blocked_until: null,
          },
        }
      : { status: 200, data: { ...sampleUser, ...me } }
  )
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

  it('links to the guide from the top of the page for requesters only', async () => {
    serve({ active_points: 0 })
    const screen = await renderAccount()
    await expect
      .element(screen.getByRole('link', { name: 'Panduan Penggunaan' }))
      .toHaveAttribute('href', '/guide')
  })

  it('hides the guide link from staff', async () => {
    serve({ role: 'TU' })
    const screen = await renderAccount({ role: 'TU' })
    await expect.element(screen.getByText('NIP')).toBeVisible()
    await expect
      .element(screen.getByRole('link', { name: 'Panduan Penggunaan' }))
      .not.toBeInTheDocument()
  })

  it('shows the program of study from the profile', async () => {
    serve({ active_points: 0, org_unit_name: 'Informatika' })
    const screen = await renderAccount()
    await expect.element(screen.getByText('Program studi')).toBeVisible()
    await expect.element(screen.getByText('Informatika')).toBeVisible()
  })

  it('lists the latest violations, dimming the ones that no longer count', async () => {
    serve({ active_points: 1 })
    const screen = await renderAccount()
    await expect.element(screen.getByText('Pembatalan terlambat')).toBeVisible()
    await expect.element(screen.getByText(/R-302/)).toBeVisible()
    await expect.element(screen.getByText(/tidak dihitung/)).toBeVisible()
  })

  it('shows no history when there are no violations', async () => {
    serve({ active_points: 0 }, [])
    const screen = await renderAccount()
    await expect.element(screen.getByText('dari 6 poin')).toBeVisible()
    await expect
      .element(screen.getByText('Riwayat', { exact: true }))
      .not.toBeInTheDocument()
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
