import { afterEach, describe, expect, it } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { THEME_STORAGE_KEY } from '@/lib/theme-storage'
import { ThemeProvider } from '@/context/theme-provider'
import { ThemeToggle } from './theme-toggle'

afterEach(() => {
  localStorage.clear()
  document.documentElement.classList.remove('light', 'dark')
})

function renderToggle() {
  return render(
    <ThemeProvider>
      <ThemeToggle />
    </ThemeProvider>
  )
}

describe('ThemeToggle', () => {
  it('marks the theme in effect and switches to the other one', async () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'light')
    const screen = await renderToggle()
    await expect
      .element(screen.getByRole('radio', { name: 'Tema terang' }))
      .toBeChecked()

    await userEvent.click(screen.getByRole('radio', { name: 'Tema gelap' }))

    await expect
      .element(screen.getByRole('radio', { name: 'Tema gelap' }))
      .toBeChecked()
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
  })

  it('switches back to light', async () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'dark')
    const screen = await renderToggle()
    await userEvent.click(screen.getByRole('radio', { name: 'Tema terang' }))
    expect(document.documentElement.classList.contains('light')).toBe(true)
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light')
  })
})
