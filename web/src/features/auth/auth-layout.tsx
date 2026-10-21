import { Logo } from '@/assets/logo'
import { ThemeToggle } from '@/components/theme-toggle'

type AuthLayoutProps = {
  children: React.ReactNode
}

/** Centered card on the canvas (Login, change password); the theme toggle sits top right. */
export function AuthLayout({ children }: AuthLayoutProps) {
  return (
    <div className='flex min-h-svh flex-col px-4'>
      <div className='flex justify-end py-4'>
        <ThemeToggle />
      </div>
      <div className='mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-6 pb-16'>
        <div className='flex items-center justify-center gap-2'>
          <span className='flex size-9 items-center justify-center rounded-full bg-primary text-primary-foreground'>
            <Logo className='size-5' />
          </span>
          <span className='type-section text-fg'>ClassLab.io</span>
        </div>
        {children}
      </div>
    </div>
  )
}
