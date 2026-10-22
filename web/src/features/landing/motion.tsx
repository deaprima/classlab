import { useEffect, useState, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { prefersReducedMotion, useInView } from './use-in-view'

/** Fades and lifts its content in when it scrolls into view; `delay` (ms) staggers siblings. */
export function Reveal({
  children,
  delay = 0,
  className,
  as: Tag = 'div',
}: {
  children: ReactNode
  delay?: number
  className?: string
  as?: 'div' | 'li' | 'section'
}) {
  const [ref, seen] = useInView<HTMLElement>(0.15)
  return (
    <Tag
      ref={ref as never}
      style={{ transitionDelay: seen ? `${delay}ms` : '0ms' }}
      className={cn(
        'transition-[opacity,transform] duration-700 ease-out motion-reduce:transition-none',
        seen ? 'translate-y-0 opacity-100' : 'translate-y-5 opacity-0',
        className
      )}
    >
      {children}
    </Tag>
  )
}

/** A number that counts up from 0 when it scrolls into view; with reduced motion it is just the number. */
export function CountUp({
  to,
  duration = 1200,
  className,
}: {
  to: number
  duration?: number
  className?: string
}) {
  const [ref, seen] = useInView<HTMLSpanElement>(0.4)
  const [value, setValue] = useState(0)
  const reduced = prefersReducedMotion()

  useEffect(() => {
    if (!seen || reduced) return
    let frame = 0
    const start = performance.now()
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration)
      // Ease out: fast at first, settling on the number.
      setValue(Math.round(to * (1 - Math.pow(1 - t, 3))))
      if (t < 1) frame = requestAnimationFrame(tick)
    }
    frame = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(frame)
  }, [seen, reduced, to, duration])

  return (
    <span ref={ref} className={className} aria-label={String(to)}>
      <span aria-hidden>{reduced ? to : value}</span>
    </span>
  )
}
