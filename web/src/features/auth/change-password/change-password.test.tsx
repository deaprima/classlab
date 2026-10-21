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
import { ChangePassword } from '.'

const replace = vi.fn()

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    useRouter: () => ({ history: { replace } }),
    useNavigate: () => vi.fn(),
  }
})

const originalAdapter = api.defaults.adapter

function renderPage(redirectTo?: string) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <ChangePassword redirectTo={redirectTo} />
    </QueryClientProvider>
  )
}

async function fill(
  screen: Awaited<ReturnType<typeof renderPage>>,
  current: string,
  next: string,
  confirm: string
) {
  await userEvent.fill(screen.getByLabelText('Password lama'), current)
  await userEvent.fill(
    screen.getByLabelText('Password baru', { exact: true }),
    next
  )
  await userEvent.fill(screen.getByLabelText('Ulangi password baru'), confirm)
  await userEvent.click(screen.getByRole('button', { name: 'Simpan Password' }))
}

describe('ChangePassword', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore
      .getState()
      .auth.setSession('token', { ...sampleUser, must_change_password: true })
  })

  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('explains why a temporary password must be changed', async () => {
    const screen = await renderPage()
    await expect
      .element(screen.getByText(/masih memakai kata sandi sementara/))
      .toBeVisible()
  })

  it('does not call the API when the form is invalid', async () => {
    const request = vi.fn(() => ({ status: 500 }))
    api.defaults.adapter = mockAdapter(request)
    const screen = await renderPage()

    await fill(screen, 'sementara-123', 'pendek', 'pendek')

    await expect
      .element(screen.getByText('Kata sandi baru minimal 10 karakter.'))
      .toBeVisible()
    expect(request).not.toHaveBeenCalled()
  })

  it('rejects a confirmation that differs', async () => {
    const request = vi.fn(() => ({ status: 500 }))
    api.defaults.adapter = mockAdapter(request)
    const screen = await renderPage()

    await fill(screen, 'sementara-123', 'kata-sandi-baru', 'kata-sandi-lain')

    await expect
      .element(screen.getByText('Kata sandi tidak sama.'))
      .toBeVisible()
    expect(request).not.toHaveBeenCalled()
  })

  it('sends the change, clears the flag and follows the redirect', async () => {
    api.defaults.adapter = mockAdapter((config) => {
      expect(config.url).toBe('/auth/change-password')
      expect(JSON.parse(String(config.data))).toEqual({
        current_password: 'sementara-123',
        new_password: 'kata-sandi-baru',
      })
      return { status: 204 }
    })
    const screen = await renderPage('/bookings')

    await fill(screen, 'sementara-123', 'kata-sandi-baru', 'kata-sandi-baru')

    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith('/bookings'))
    expect(useAuthStore.getState().auth.user?.must_change_password).toBe(false)
  })

  it('shows a wrong current password on its field', async () => {
    api.defaults.adapter = mockAdapter(() => ({
      status: 422,
      data: apiErrorBody('VALIDATION_ERROR', 'Data tidak valid.', [
        { field: 'current_password', message: 'Kata sandi saat ini salah.' },
      ]),
    }))
    const screen = await renderPage()

    await fill(screen, 'salah-salah-1', 'kata-sandi-baru', 'kata-sandi-baru')

    await expect
      .element(screen.getByText('Kata sandi saat ini salah.'))
      .toBeVisible()
    expect(replace).not.toHaveBeenCalled()
    expect(useAuthStore.getState().auth.user?.must_change_password).toBe(true)
  })

  it('shows the server message when the attempts are rate limited', async () => {
    api.defaults.adapter = mockAdapter(() => ({
      status: 429,
      data: apiErrorBody(
        'RATE_LIMITED',
        'Terlalu banyak percobaan. Coba lagi nanti.'
      ),
    }))
    const screen = await renderPage()

    await fill(screen, 'sementara-123', 'kata-sandi-baru', 'kata-sandi-baru')

    await expect
      .element(screen.getByText('Terlalu banyak percobaan. Coba lagi nanti.'))
      .toBeVisible()
  })
})
