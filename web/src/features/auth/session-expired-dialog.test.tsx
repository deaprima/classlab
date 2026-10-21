import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { useSessionExpiredStore } from '@/stores/session-expired-store'
import { SessionExpiredDialog } from './session-expired-dialog'

const navigate = vi.fn()

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return { ...actual, useNavigate: () => navigate }
})

function renderDialog(queryClient = new QueryClient()) {
  return render(
    <QueryClientProvider client={queryClient}>
      <SessionExpiredDialog />
    </QueryClientProvider>
  )
}

describe('SessionExpiredDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore.getState().auth.setSession('token', { ...sampleUser })
  })

  afterEach(() => {
    useAuthStore.getState().auth.reset()
    useSessionExpiredStore.getState().close()
  })

  it('is hidden until a session expires', async () => {
    const screen = await renderDialog()
    await expect
      .element(screen.getByRole('alertdialog'))
      .not.toBeInTheDocument()
  })

  it('keeps the session until the user chooses to sign in again', async () => {
    const screen = await renderDialog()
    useSessionExpiredStore.getState().show('/bookings?status=MENUNGGU')

    await expect
      .element(screen.getByRole('heading', { name: 'Sesi berakhir' }))
      .toBeVisible()
    expect(useAuthStore.getState().auth.isAuthenticated).toBe(true)
  })

  it('signs in again: drops the session and returns to the page afterwards', async () => {
    const queryClient = new QueryClient()
    queryClient.setQueryData(['x'], 1)
    const screen = await renderDialog(queryClient)
    useSessionExpiredStore.getState().show('/bookings?status=MENUNGGU')

    await userEvent.click(screen.getByRole('button', { name: 'Masuk Lagi' }))

    expect(navigate).toHaveBeenCalledWith({
      to: '/sign-in',
      search: { redirect: '/bookings?status=MENUNGGU' },
      replace: true,
    })
    expect(useAuthStore.getState().auth.user).toBeNull()
    expect(queryClient.getQueryData(['x'])).toBeUndefined()
    expect(useSessionExpiredStore.getState().open).toBe(false)
  })

  it('remembers the first page when several requests fail together', () => {
    useSessionExpiredStore.getState().show('/a')
    useSessionExpiredStore.getState().show('/b')
    expect(useSessionExpiredStore.getState().redirect).toBe('/a')
  })
})
