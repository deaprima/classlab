import { createContext, useContext, useEffect, useState, useMemo } from 'react'
import { readTheme, writeTheme, type Theme } from '@/lib/theme-storage'

type ResolvedTheme = Exclude<Theme, 'system'>

const DEFAULT_THEME: Theme = 'system'
// theme-color of the browser chrome: bg/canvas of each mode.
const THEME_COLORS: Record<ResolvedTheme, string> = {
  light: '#ededec',
  dark: '#0a0a0a',
}

type ThemeProviderProps = {
  children: React.ReactNode
}

type ThemeProviderState = {
  defaultTheme: Theme
  resolvedTheme: ResolvedTheme
  theme: Theme
  setTheme: (theme: Theme) => void
  resetTheme: () => void
}

const initialState: ThemeProviderState = {
  defaultTheme: DEFAULT_THEME,
  resolvedTheme: 'light',
  theme: DEFAULT_THEME,
  setTheme: () => null,
  resetTheme: () => null,
}

const ThemeContext = createContext<ThemeProviderState>(initialState)

const systemTheme = (): ResolvedTheme =>
  window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'

export function ThemeProvider({ children }: ThemeProviderProps) {
  const [theme, _setTheme] = useState<Theme>(readTheme)
  // Follows the OS while the choice is "system".
  const [system, setSystem] = useState<ResolvedTheme>(systemTheme)

  const resolvedTheme: ResolvedTheme = theme === 'system' ? system : theme

  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    const handleChange = () => setSystem(mediaQuery.matches ? 'dark' : 'light')
    mediaQuery.addEventListener('change', handleChange)
    return () => mediaQuery.removeEventListener('change', handleChange)
  }, [])

  useEffect(() => {
    const root = window.document.documentElement
    root.classList.remove('light', 'dark')
    root.classList.add(resolvedTheme)
    document
      .querySelector("meta[name='theme-color']")
      ?.setAttribute('content', THEME_COLORS[resolvedTheme])
  }, [resolvedTheme])

  const contextValue = useMemo(
    () => ({
      defaultTheme: DEFAULT_THEME,
      resolvedTheme,
      theme,
      setTheme: (next: Theme) => {
        writeTheme(next)
        _setTheme(next)
      },
      resetTheme: () => {
        writeTheme(null)
        _setTheme(DEFAULT_THEME)
      },
    }),
    [resolvedTheme, theme]
  )

  return <ThemeContext value={contextValue}>{children}</ThemeContext>
}

// eslint-disable-next-line react-refresh/only-export-components
export const useTheme = () => {
  const context = useContext(ThemeContext)

  if (!context) throw new Error('useTheme must be used within a ThemeProvider')

  return context
}
