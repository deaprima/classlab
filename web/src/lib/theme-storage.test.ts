import { afterEach, describe, expect, it, vi } from 'vitest'
import { readTheme, THEME_STORAGE_KEY, writeTheme } from './theme-storage'

afterEach(() => {
  vi.restoreAllMocks()
  localStorage.clear()
})

describe('theme storage', () => {
  it('defaults to system when nothing is saved', () => {
    expect(readTheme()).toBe('system')
  })

  it('round-trips a saved theme', () => {
    writeTheme('dark')
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
    expect(readTheme()).toBe('dark')
  })

  it('ignores an unknown stored value', () => {
    localStorage.setItem(THEME_STORAGE_KEY, 'sepia')
    expect(readTheme()).toBe('system')
  })

  it('clears the saved theme', () => {
    writeTheme('light')
    writeTheme(null)
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBeNull()
  })

  it('survives storage that throws', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new DOMException('blocked', 'SecurityError')
    })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('blocked', 'SecurityError')
    })
    expect(readTheme()).toBe('system')
    expect(() => writeTheme('dark')).not.toThrow()
  })
})
