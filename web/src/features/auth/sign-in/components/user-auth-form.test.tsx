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
import { UserAuthForm } from './user-auth-form'

const replace = vi.fn()

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    useRouter: () => ({ history: { replace } }),
  }
})

const originalAdapter = api.defaults.adapter

function renderForm(redirectTo?: string) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <UserAuthForm redirectTo={redirectTo} />
    </QueryClientProvider>
  )
}

async function fillAndSubmit(
  screen: Awaited<ReturnType<typeof renderForm>>,
  identifier: string,
  password: string
) {
  await userEvent.fill(screen.getByLabelText('NIM / NIP'), identifier)
  await userEvent.fill(screen.getByLabelText('Kata Sandi'), password)
  await userEvent.click(screen.getByRole('button', { name: 'Masuk' }))
}

describe('UserAuthForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('requires both fields before calling the API', async () => {
    const request = vi.fn(() => ({ status: 500 }))
    api.defaults.adapter = mockAdapter(request)
    const screen = await renderForm()

    await userEvent.click(screen.getByRole('button', { name: 'Masuk' }))

    await expect
      .element(screen.getByText('NIM atau NIP wajib diisi.'))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText('Kata sandi wajib diisi.'))
      .toBeInTheDocument()
    expect(request).not.toHaveBeenCalled()
  })

  it('stores the session in memory and follows the redirect', async () => {
    api.defaults.adapter = mockAdapter((config) => {
      expect(JSON.parse(String(config.data))).toEqual({
        identifier: '220101001',
        password: 'rahasia123',
      })
      return {
        status: 200,
        data: { access_token: 'jwt', expires_in: 900, user: sampleUser },
      }
    })
    const screen = await renderForm('/bookings?status=MENUNGGU')

    await fillAndSubmit(screen, '220101001', 'rahasia123')

    await vi.waitFor(() =>
      expect(replace).toHaveBeenCalledWith('/bookings?status=MENUNGGU')
    )
    expect(useAuthStore.getState().auth.accessToken).toBe('jwt')
    expect(useAuthStore.getState().auth.user?.name).toBe(sampleUser.name)
  })

  it('sends an account with a temporary password to the change page first', async () => {
    api.defaults.adapter = mockAdapter(() => ({
      status: 200,
      data: {
        access_token: 'jwt',
        expires_in: 900,
        user: { ...sampleUser, must_change_password: true },
      },
    }))
    const screen = await renderForm('/bookings?status=MENUNGGU')

    await fillAndSubmit(screen, '220101001', 'sementara-123')

    await vi.waitFor(() =>
      expect(replace).toHaveBeenCalledWith(
        '/change-password?redirect=%2Fbookings%3Fstatus%3DMENUNGGU'
      )
    )
  })

  it('ignores a redirect to another site', async () => {
    api.defaults.adapter = mockAdapter(() => ({
      status: 200,
      data: { access_token: 'jwt', expires_in: 900, user: sampleUser },
    }))
    const screen = await renderForm('https://evil.example/')

    await fillAndSubmit(screen, '220101001', 'rahasia123')

    await vi.waitFor(() => expect(replace).toHaveBeenCalledWith('/'))
  })

  it('shows the server message for wrong credentials', async () => {
    api.defaults.adapter = mockAdapter(() => ({
      status: 401,
      data: apiErrorBody('UNAUTHENTICATED', 'NIM/NIP atau kata sandi salah.'),
    }))
    const screen = await renderForm()

    await fillAndSubmit(screen, '220101001', 'salah')

    await expect
      .element(screen.getByText('NIM/NIP atau kata sandi salah.'))
      .toBeInTheDocument()
    expect(useAuthStore.getState().auth.isAuthenticated).toBe(false)
    expect(replace).not.toHaveBeenCalled()
  })

  it('shows the rate-limit message', async () => {
    api.defaults.adapter = mockAdapter(() => ({
      status: 429,
      data: apiErrorBody(
        'RATE_LIMITED',
        'Terlalu banyak percobaan masuk. Coba lagi nanti.'
      ),
    }))
    const screen = await renderForm()

    await fillAndSubmit(screen, '220101001', 'salah')

    await expect
      .element(
        screen.getByText('Terlalu banyak percobaan masuk. Coba lagi nanti.')
      )
      .toBeInTheDocument()
  })
})
