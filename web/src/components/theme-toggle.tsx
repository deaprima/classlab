import { Moon, Sun } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useTheme } from '@/context/theme-provider'

const OPTIONS = [
  { value: 'light', label: 'Tema terang', icon: Sun },
  { value: 'dark', label: 'Tema gelap', icon: Moon },
] as const

/** The sun/moon pill of the Figma sidebar and landing nav. The active side is the theme in effect. */
export function ThemeToggle({ className }: { className?: string }) {
  const { resolvedTheme, setTheme } = useTheme()

  return (
    <div
      role='radiogroup'
      aria-label='Tema'
      data-slot='theme-toggle'
      className={cn(
        'inline-flex w-fit items-center gap-1 rounded-full border bg-surface p-1',
        className
      )}
    >
      {OPTIONS.map(({ value, label, icon: Icon }) => {
        const active = resolvedTheme === value
        return (
          <button
            key={value}
            type='button'
            role='radio'
            aria-checked={active}
            aria-label={label}
            onClick={() => setTheme(value)}
            className={cn(
              'flex size-8 items-center justify-center rounded-full transition-colors',
              active
                ? 'bg-primary text-primary-foreground'
                : 'text-fg-secondary hover:text-fg'
            )}
          >
            <Icon aria-hidden className='size-4' />
          </button>
        )
      })}
    </div>
  )
}
