import { Logo } from '@/assets/logo'
import { ThemeToggle } from '@/components/theme-toggle'

/** Figma `Logo / Full`: the orange-dot mark and the product name. */
function BrandLogo() {
  return (
    <div className='flex items-center gap-2'>
      <Logo className='size-8' />
      <span className='type-section text-fg'>ClassLab</span>
    </div>
  )
}

/**
 * Change-password (Figma "Ganti Password"): one card centered on the canvas, the logo on top.
 * The theme toggle sits top right.
 */
export function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className='flex min-h-svh flex-col px-4'>
      <div className='flex justify-end py-4'>
        <ThemeToggle />
      </div>
      <div className='mx-auto flex w-full max-w-110 flex-1 flex-col justify-center pb-16'>
        <div className='flex flex-col gap-4 rounded-[20px] bg-surface p-6'>
          <BrandLogo />
          {children}
        </div>
      </div>
    </div>
  )
}

/**
 * Sign-in (Figma "Login"): full screen. On wide screens the canvas holds the brand panel and the rounded form card; below `lg` only the form remains, full screen. The brand panel is the inverse of the page (dark in light theme, light in dark theme); the card follows the theme.
 */
export function SplitAuthLayout({
  aside,
  children,
}: {
  aside?: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <div className='flex min-h-svh bg-surface lg:h-svh lg:bg-canvas lg:p-2'>
      <div className='flex w-full flex-1 overflow-hidden lg:gap-2'>
        <BrandPanel />
        <div className='flex min-w-0 flex-1 flex-col bg-surface p-6 lg:rounded-3xl lg:px-10 lg:py-8'>
          <div className='flex items-center justify-between gap-4'>
            <BrandLogo />
            <div className='flex items-center gap-4'>
              {aside}
              <ThemeToggle />
            </div>
          </div>
          <div className='flex flex-1 flex-col items-center justify-center py-10'>
            <div className='flex w-full max-w-90 flex-col gap-4'>
              {children}
            </div>
          </div>
          <div className='flex items-center justify-between gap-4 type-body-sm text-fg-secondary'>
            <span>© 2026 ClassLab</span>
            <span>Kebijakan Privasi</span>
          </div>
        </div>
      </div>
    </div>
  )
}

function BrandPanel() {
  return (
    <div className='hidden w-125 shrink-0 flex-col items-center justify-end gap-4 rounded-3xl bg-inverse px-8 py-10 lg:flex'>
      {/* The panel is the inverse of the page, so the tile is too. */}
      <Logo inverse className='size-18' />
      <p className='flex flex-col items-center text-center text-[32px] leading-10 font-semibold tracking-[-0.64px] whitespace-nowrap'>
        <span className='text-fg-inverse'>Satu platform untuk</span>
        <span className='text-fg-inverse/60'>booking semua ruang kampus</span>
      </p>
      <p className='w-85 text-center type-body text-fg-inverse/50'>
        Lihat ruang yang kosong, ajukan, dan check-in dalam satu tempat.
      </p>
      <div aria-hidden className='flex gap-2'>
        <span className='size-1.5 rounded-full bg-fg-inverse' />
        <span className='size-1.5 rounded-full bg-fg-inverse/30' />
        <span className='size-1.5 rounded-full bg-fg-inverse/30' />
      </div>
    </div>
  )
}
