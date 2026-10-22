import { useEffect, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { Logo } from '@/assets/logo'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { ThemeToggle } from '@/components/theme-toggle'
import { NAV_LINKS, SECTION_X } from './constants'
import { DoorPlate } from './door-plate'
import { Reveal } from './motion'
import { ScheduleStrip } from './schedule-strip'

/** The capsule nav: it gets a soft shadow once the page scrolls, so it reads as floating above the content. */
export function Nav() {
  const [scrolled, setScrolled] = useState(false)
  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  return (
    <header className='sticky top-4 z-40 mt-4 px-4'>
      <nav
        aria-label='Utama'
        className={cn(
          'mx-auto flex max-w-[1200px] items-center justify-between gap-4 rounded-full border bg-surface/90 py-2 ps-5 pe-2 backdrop-blur transition-shadow duration-300',
          scrolled && 'shadow-lg'
        )}
      >
        <Link to='/' className='flex items-center gap-2 type-section text-fg'>
          <Logo className='size-5' />
          ClassLab.io
        </Link>
        <ul className='hidden items-center gap-6 md:flex'>
          {NAV_LINKS.map((l) => (
            <li key={l.href}>
              <a
                href={l.href}
                className='type-label text-fg-secondary transition-colors hover:text-fg'
              >
                {l.label}
              </a>
            </li>
          ))}
        </ul>
        <div className='flex items-center gap-2'>
          <ThemeToggle />
          <Button asChild>
            <Link to='/sign-in'>Masuk</Link>
          </Button>
        </div>
      </nav>
    </header>
  )
}

/** Hero: headline and buttons on the left, the door sign on the right, the day's schedule strip under both. */
export function Hero() {
  return (
    <section aria-labelledby='hero-title'>
      <div
        className={cn(
          SECTION_X,
          'grid items-center gap-12 pt-16 pb-12 lg:grid-cols-[minmax(0,1fr)_420px] lg:pt-24'
        )}
      >
        <div>
          <p className='mb-6 inline-flex rounded-full border bg-surface px-3 py-1 type-label text-fg-secondary motion-safe:animate-landing-rise'>
            Booking ruang kampus
          </p>
          <h1
            id='hero-title'
            className='text-[clamp(52px,9vw,104px)] leading-[0.96] font-semibold tracking-[-0.03em] text-fg motion-safe:animate-landing-rise'
            style={{ animationDelay: '80ms' }}
          >
            Ruang kosong.
            <br />
            Jam pasti<span className='text-fg-accent'>.</span>
          </h1>
          <p
            className='mt-6 max-w-[540px] type-body-lg text-fg-secondary motion-safe:animate-landing-rise'
            style={{ animationDelay: '180ms' }}
          >
            Ajukan ruang kelas, lab, auditorium, ruang ujian, atau ruang sidang.
            Slot yang kosong langsung terlihat, bentrok dicegah sistem, dan
            check-in cukup memindai QR di pintu.
          </p>
          <div
            className='mt-8 flex flex-wrap items-center gap-3 motion-safe:animate-landing-rise'
            style={{ animationDelay: '280ms' }}
          >
            <Button variant='accent' size='lg' className='h-12 px-6' asChild>
              <Link to='/sign-in'>Masuk dengan NIM / NIP</Link>
            </Button>
            <Button variant='ghost' size='lg' className='h-12 px-5' asChild>
              <a href='#cara-kerja'>Lihat cara kerja</a>
            </Button>
          </div>
          <p
            className='mt-4 type-body-sm text-fg-tertiary motion-safe:animate-landing-rise'
            style={{ animationDelay: '360ms' }}
          >
            Akun dibuat oleh TU kampus.
          </p>
        </div>
        <DoorPlate />
      </div>
      <div className={cn(SECTION_X, 'pb-8')}>
        <Reveal>
          <ScheduleStrip />
        </Reveal>
      </div>
    </section>
  )
}
