import { Link } from '@tanstack/react-router'
import { Logo } from '@/assets/logo'
import { Button } from '@/components/ui/button'
import { NAV_LINKS } from './constants'
import { Reveal } from './motion'

/** Footer: the inverse block with the brand, link columns and the oversized wordmark cut off at the bottom edge. */
export function LandingFooter() {
  return (
    <footer className='overflow-hidden rounded-t-[32px] bg-inverse text-fg-inverse'>
      <div className='mx-auto w-full max-w-[1440px] px-5 pt-16 sm:px-10 xl:px-[120px]'>
        <Reveal>
          <div className='grid gap-12 md:grid-cols-[minmax(0,1.4fr)_repeat(3,minmax(0,1fr))]'>
            <div className='flex flex-col items-start gap-4'>
              <p className='flex items-center gap-2 type-section'>
                <Logo className='size-5' />
                ClassLab.io
              </p>
              <p className='max-w-xs type-body text-fg-inverse/70'>
                Ruang kosong, jam pasti. Booking ruang kampus yang jelas dari
                pengajuan sampai check-out.
              </p>
              <Button variant='accent' asChild>
                <Link to='/sign-in'>Masuk</Link>
              </Button>
            </div>
            <nav aria-label='Platform'>
              <p className='mb-4 type-overline text-fg-inverse/60'>Platform</p>
              <ul className='flex flex-col gap-2 type-body'>
                {NAV_LINKS.map((l) => (
                  <li key={l.href}>
                    <a
                      href={l.href}
                      className='text-fg-inverse/80 transition-colors hover:text-fg-inverse'
                    >
                      {l.label}
                    </a>
                  </li>
                ))}
              </ul>
            </nav>
            <nav aria-label='Pengguna'>
              <p className='mb-4 type-overline text-fg-inverse/60'>Pengguna</p>
              <ul className='flex flex-col gap-2 type-body'>
                <li>
                  <Link
                    to='/sign-in'
                    className='text-fg-inverse/80 transition-colors hover:text-fg-inverse'
                  >
                    Masuk
                  </Link>
                </li>
                <li className='text-fg-inverse/60'>Mahasiswa dan Organisasi</li>
                <li className='text-fg-inverse/60'>TU, Kalab, OB, Admin</li>
              </ul>
            </nav>
            <div>
              <p className='mb-4 type-overline text-fg-inverse/60'>Informasi</p>
              <ul className='flex flex-col gap-2 type-body text-fg-inverse/60'>
                <li>Akun dibuat oleh TU kampus</li>
                <li>Tidak ada pendaftaran mandiri</li>
              </ul>
            </div>
          </div>
        </Reveal>
        <div className='mt-16 border-t border-fg-inverse/15 pt-6 type-body-sm text-fg-inverse/60'>
          <span>© {new Date().getFullYear()} ClassLab.io</span>
        </div>
      </div>
      <p
        aria-hidden
        className='mx-auto mt-6 -mb-[0.2em] w-full max-w-[1440px] px-5 text-center text-[clamp(64px,16vw,232px)] leading-[0.9] font-semibold tracking-[-0.04em] whitespace-nowrap text-fg-inverse/15 select-none'
      >
        ClassLab.io
      </p>
    </footer>
  )
}
