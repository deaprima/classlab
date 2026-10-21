import { describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { Guide } from '.'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    Link: ({ to, children }: { to: string; children: React.ReactNode }) => (
      <a href={to}>{children}</a>
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

describe('Guide (Panduan)', () => {
  it('walks through using the app in order', async () => {
    const screen = await render(<Guide />)
    await expect
      .element(screen.getByRole('heading', { name: 'Cara memakai ClassLab' }))
      .toBeVisible()
    await expect
      .element(screen.getByText('Check-in dengan QR di pintu'))
      .toBeVisible()
    await expect
      .element(screen.getByText('Check-out dengan foto'))
      .toBeVisible()
  })

  it('explains each kind of violation point and the block', async () => {
    const screen = await render(<Guide />)
    await expect.element(screen.getByText('Tidak hadir')).toBeVisible()
    await expect.element(screen.getByText('2 poin')).toBeVisible()
    await expect
      .element(screen.getByText(/mencapai 6, kamu tidak bisa mengajukan/))
      .toBeVisible()
    await expect
      .element(screen.getByRole('link', { name: /Lihat poinmu/ }))
      .toHaveAttribute('href', '/account')
  })
})
