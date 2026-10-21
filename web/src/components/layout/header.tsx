import { cn } from '@/lib/utils'
import { SidebarTrigger } from '@/components/ui/sidebar'

type HeaderProps = React.HTMLAttributes<HTMLElement> & {
  fixed?: boolean
  ref?: React.Ref<HTMLElement>
}

/**
 * Top Bar (DESIGN.md 3.1): a separate white card, 72 px high with a border, inset from the sidebar
 * by the page padding (20 px above, 24 px at the sides). When fixed, the canvas-coloured wrapper
 * keeps scrolled content from showing in those gaps. The sidebar trigger only shows on mobile.
 */
export function Header({ className, fixed, children, ...props }: HeaderProps) {
  return (
    <header
      className={cn(
        'z-50 px-4 pt-4 md:px-6 md:pt-5',
        fixed &&
          'header-fixed peer/header sticky top-0 w-[inherit] bg-background',
        className
      )}
      {...props}
    >
      <div className='flex h-18 items-center gap-3 border bg-surface px-4 sm:gap-4 md:px-6'>
        <SidebarTrigger variant='outline' className='md:hidden' />
        {children}
      </div>
    </header>
  )
}
