import { useEffect, useRef, useState } from 'react'

/** Whether the user asked for less motion; then nothing animates and the final state shows at once. */
export function prefersReducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  )
}

/**
 * True once the element has scrolled into view (and stays true). Without IntersectionObserver, or
 * with reduced motion, it is true from the start, so the content is never hidden for good.
 */
export function useInView<T extends Element>(threshold = 0.2) {
  const ref = useRef<T>(null)
  const [seen, setSeen] = useState(
    () => typeof IntersectionObserver === 'undefined' || prefersReducedMotion()
  )

  useEffect(() => {
    const node = ref.current
    if (seen || !node) return
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) {
          setSeen(true)
          observer.disconnect()
        }
      },
      { threshold }
    )
    observer.observe(node)
    return () => observer.disconnect()
  }, [seen, threshold])

  return [ref, seen] as const
}
