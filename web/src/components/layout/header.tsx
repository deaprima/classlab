import { cn } from '@/lib/utils'
import { SidebarTrigger } from '@/components/ui/sidebar'

type HeaderProps = React.HTMLAttributes<HTMLElement> & {
  fixed?: boolean
  ref?: React.Ref<HTMLElement>
}

/** Top Bar (DESIGN.md 3.1): 72 px, bg/surface, bottom border. The sidebar trigger only shows on mobile. */
export function Header({ className, fixed, children, ...props }: HeaderProps) {
  return (
    <header
      className={cn(
        'z-50 h-18 border-b bg-surface',
        fixed && 'header-fixed peer/header sticky top-0 w-[inherit]',
        className
      )}
      {...props}
    >
      <div className='flex h-full items-center gap-3 px-4 sm:gap-4 md:px-6'>
        <SidebarTrigger variant='outline' className='md:hidden' />
        {children}
      </div>
    </header>
  )
}
