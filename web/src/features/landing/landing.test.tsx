import { attendanceRules } from '@/config/attendance-rules'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { ThemeProvider } from '@/context/theme-provider'
import { Landing } from '.'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    Link: ({
      children,
      to,
      ...rest
    }: {
      children: React.ReactNode
      to: string
    }) => (
      <a href={to} {...rest}>
        {children}
      </a>
    ),
  }
})

function renderLanding() {
  return render(
    <ThemeProvider>
      <Landing />
    </ThemeProvider>
  )
}

describe('Landing', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })
  afterEach(() => {
    document.documentElement.classList.remove('motion-safe:scroll-smooth')
  })

  it('opens with the headline and sends visitors to sign in', async () => {
    const screen = await renderLanding()

    await expect
      .element(screen.getByRole('heading', { level: 1 }))
      .toHaveTextContent('Ruang kosong.')
    await expect
      .element(screen.getByRole('heading', { level: 1 }))
      .toHaveTextContent('Jam pasti.')
    const signIn = screen.getByRole('link', { name: 'Masuk dengan NIM / NIP' })
    await expect.element(signIn).toHaveAttribute('href', '/sign-in')
    await expect
      .element(screen.getByText('Akun dibuat oleh TU kampus.').first())
      .toBeVisible()
  })

  it('has the seven sections of the design, each reachable from the nav', async () => {
    const screen = await renderLanding()

    for (const [name, id] of [
      ['Cara kerja', '#cara-kerja'],
      ['Fitur', '#fitur'],
      ['Peran', '#peran'],
      ['Aturan', '#aturan'],
    ] as const) {
      await expect
        .element(
          screen
            .getByRole('navigation', { name: 'Utama' })
            .getByRole('link', { name })
        )
        .toHaveAttribute('href', id)
    }
    await expect
      .element(screen.getByRole('heading', { name: 'Satu hari di R-302.' }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('heading', { name: /Dibuat untuk ruang/ }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('heading', { name: 'Enam peran, satu platform.' }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('heading', { name: /Lima angka/ }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('heading', { name: /Ruang itu menunggu/ }))
      .toBeInTheDocument()
    await expect
      .element(screen.getByRole('contentinfo'))
      .not.toHaveTextContent('Kelompok 2')
  })

  it('takes the numbers of the rules from the app config, not from copy', async () => {
    const screen = await renderLanding()

    // CountUp keeps the final value in its label while it counts.
    await expect
      .element(
        screen
          .getByLabelText(String(attendanceRules.checkInEarlyMinutes))
          .first()
      )
      .toBeInTheDocument()
    await expect
      .element(
        screen
          .getByLabelText(String(attendanceRules.blockThresholdPoints))
          .first()
      )
      .toBeInTheDocument()
  })

  it('keeps the theme toggle in the nav', async () => {
    const screen = await renderLanding()

    await expect
      .element(screen.getByRole('radiogroup', { name: 'Tema' }))
      .toBeVisible()
  })

  it('lists six roles and only copy that is not a testimonial or a price', async () => {
    const screen = await renderLanding()

    for (const role of [
      'Mahasiswa',
      'Organisasi',
      'TU',
      'Pengurus Lab',
      'OB',
      'Admin',
    ]) {
      await expect
        .element(
          screen.getByRole('heading', { name: role, level: 3, exact: true })
        )
        .toBeInTheDocument()
    }
    expect(screen.getByText(/testimoni|harga|Rp\s?\d/i).elements()).toEqual([])
  })
})
