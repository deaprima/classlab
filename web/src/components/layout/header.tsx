import { useEffect, useState } from 'react'
import { cn } from '@/lib/utils'
import { SidebarTrigger } from '@/components/ui/sidebar'

type HeaderProps = React.HTMLAttributes<HTMLElement> & {
  fixed?: boolean
  ref?: React.Ref<HTMLElement>
}

/**
 * Top Bar (docs/DESIGN.md 3.1), 72 px high. On desktop it is a separate bordered card inset from the
 * sidebar (20 px above, 24 px at the sides); on mobile it is a full-width bar flush with the screen
 * edges. When fixed it sticks to the top and, once the page has scrolled, casts a shadow so it does
 * not blend into the content passing under it. The sidebar trigger only shows on mobile.
 */
export function Header({ className, fixed, children, ...props }: HeaderProps) {
  const [scrolled, setScrolled] = useState(false)

  useEffect(() => {
    const onScroll = () =>
      setScrolled(
        (document.body.scrollTop || document.documentElement.scrollTop) > 4
      )
    onScroll()
    document.addEventListener('scroll', onScroll, { passive: true })
    return () => document.removeEventListener('scroll', onScroll)
  }, [])

  return (
    <header
      className={cn(
        'z-50 md:px-6 md:pt-5',
        fixed &&
          'header-fixed peer/header sticky top-0 w-[inherit] bg-background',
        className
      )}
      {...props}
    >
      <div
        className={cn(
          'flex h-18 items-center gap-3 border-b bg-surface px-4 transition-shadow sm:gap-4 md:border md:px-6',
          scrolled &&
            'shadow-[0_6px_14px_-6px_rgb(0_0_0/0.18)] dark:shadow-[0_6px_14px_-6px_rgb(0_0_0/0.7)]'
        )}
      >
        <SidebarTrigger variant='outline' className='md:hidden' />
        {children}
      </div>
    </header>
  )
}
