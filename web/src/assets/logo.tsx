import { cn } from '@/lib/utils'
import logoForDarkTheme from './classlab-logo-dark-500.svg'
import logoForLightTheme from './classlab-logo-light-500.svg'

/**
 * The ClassLab mark. It is a rounded tile that has to stand out from its surroundings: a dark tile
 * on the light theme, a white one on the dark theme. `inverse` is for surfaces that are themselves
 * the inverse of the page (the landing footer, the sign-in brand panel), where it is the other way
 * round. The size comes from `className`; the file is square.
 */
export function Logo({
  className,
  inverse = false,
}: {
  className?: string
  inverse?: boolean
}) {
  const onLight = inverse ? logoForDarkTheme : logoForLightTheme
  const onDark = inverse ? logoForLightTheme : logoForDarkTheme
  return (
    <>
      <img
        src={onLight}
        alt=''
        className={cn('size-6 dark:hidden', className)}
      />
      <img
        src={onDark}
        alt=''
        className={cn('hidden size-6 dark:block', className)}
      />
    </>
  )
}
