import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { AuthenticatedGate } from './authenticated-gate'

const mocks = vi.hoisted(() => ({ context: { guest: true } }))

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    getRouteApi: () => ({ useRouteContext: () => mocks.context }),
  }
})
vi.mock('@/features/landing', () => ({ Landing: () => <p>Halaman landing</p> }))
vi.mock('./authenticated-layout', () => ({
  AuthenticatedLayout: () => <p>Shell aplikasi</p>,
}))

describe('AuthenticatedGate', () => {
  beforeEach(() => {
    mocks.context = { guest: true }
  })

  it('shows the landing page to a visitor without a session', async () => {
    const screen = await render(<AuthenticatedGate />)

    await expect.element(screen.getByText('Halaman landing')).toBeVisible()
    expect(screen.getByText('Shell aplikasi').elements()).toEqual([])
  })

  it('shows the app shell to a signed-in user', async () => {
    mocks.context = { guest: false }
    const screen = await render(<AuthenticatedGate />)

    await expect.element(screen.getByText('Shell aplikasi')).toBeVisible()
    expect(screen.getByText('Halaman landing').elements()).toEqual([])
  })
})
