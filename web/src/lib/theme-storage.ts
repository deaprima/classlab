// The theme choice is a per-browser convenience, so storage can fail (private window, blocked site
// data) without breaking the page: every access is guarded and falls back to "system".

export type Theme = 'dark' | 'light' | 'system'

export const THEME_STORAGE_KEY = 'classlab-theme'

const THEMES: readonly Theme[] = ['dark', 'light', 'system']

export function readTheme(): Theme {
  try {
    const value = localStorage.getItem(THEME_STORAGE_KEY)
    return THEMES.includes(value as Theme) ? (value as Theme) : 'system'
  } catch {
    return 'system'
  }
}

export function writeTheme(theme: Theme | null): void {
  try {
    if (theme === null) localStorage.removeItem(THEME_STORAGE_KEY)
    else localStorage.setItem(THEME_STORAGE_KEY, theme)
  } catch {
    // Not persisted; the choice still applies until the page is closed.
  }
}
