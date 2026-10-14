import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { SignOutDialog } from './sign-out-dialog'

const navigate = vi.fn()
const toastError = vi.hoisted(() => vi.fn())

vi.mock('sonner', () => ({ toast: { error: toastError } }))

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    useNavigate: () => navigate,
  }
})

const originalAdapter = api.defaults.adapter

function renderDialog() {
  const queryClient = new QueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <SignOutDialog open onOpenChange={vi.fn()} />
    </QueryClientProvider>
  )
}

describe('SignOutDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
  })

  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('revokes the session on the server, clears it locally and goes to sign-in', async () => {
    const logout = vi.fn(() => ({ status: 200 }))
    api.defaults.adapter = mockAdapter((config) =>
      config.url === '/auth/logout' && config.method === 'post'
        ? logout()
        : { status: 404 }
    )
    const { getByRole } = await renderDialog()

    await userEvent.click(getByRole('button', { name: /^Keluar$/ }))

    await vi.waitFor(() => expect(navigate).toHaveBeenCalled())
    expect(logout).toHaveBeenCalledOnce()
    expect(useAuthStore.getState().auth.isAuthenticated).toBe(false)
    expect(navigate).toHaveBeenCalledWith({ to: '/sign-in', replace: true })
  })

  it('keeps the user signed in when the server cannot be reached', async () => {
    api.defaults.adapter = mockAdapter(() => ({ status: 503 }))
    const { getByRole } = await renderDialog()

    await userEvent.click(getByRole('button', { name: /^Keluar$/ }))

    await vi.waitFor(() => expect(toastError).toHaveBeenCalled())
    expect(useAuthStore.getState().auth.isAuthenticated).toBe(true)
    expect(navigate).not.toHaveBeenCalled()
  })

  it('does nothing when Batal is clicked', async () => {
    const { getByRole } = await renderDialog()

    await userEvent.click(getByRole('button', { name: /^Batal$/ }))

    expect(navigate).not.toHaveBeenCalled()
    expect(useAuthStore.getState().auth.isAuthenticated).toBe(true)
  })
})
