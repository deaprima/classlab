import { useEffect } from 'react'
import { LandingFooter } from './footer'
import { Hero, Nav } from './hero'
import { Cta, Features, HowItWorks, Roles, Rules } from './sections'

/** The page a visitor without a session sees at "/" (docs/DESIGN.md 3.3): a door sign, not a SaaS template. */
export function Landing() {
  // The nav links jump to sections; glide there unless the user asked for less motion.
  useEffect(() => {
    const root = document.documentElement
    root.classList.add('motion-safe:scroll-smooth')
    return () => root.classList.remove('motion-safe:scroll-smooth')
  }, [])

  return (
    <div className='min-h-svh overflow-x-clip bg-background text-fg'>
      <Nav />
      <main id='content'>
        <Hero />
        <HowItWorks />
        <Features />
        <Roles />
        <Rules />
        <Cta />
      </main>
      <LandingFooter />
    </div>
  )
}
