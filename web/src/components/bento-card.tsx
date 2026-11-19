import { cn } from '@/lib/utils'

type BentoCardProps = {
  title?: React.ReactNode
  /** Right-aligned control next to the title (Figma `Show Action`). */
  action?: React.ReactNode
  /** `subtle` sits on a card and uses bg/subtle (Figma `Style`). */
  tone?: 'default' | 'subtle'
  className?: string
  children: React.ReactNode
}

/** Figma `Card`: radius 2xl, padding 20, gap 16, title row, then the `Content` slot. */
export function BentoCard({
  title,
  action,
  tone = 'default',
  className,
  children,
}: BentoCardProps) {
  return (
    <section
      data-slot='bento-card'
      className={cn(
        'flex flex-col gap-4 rounded-2xl border p-5',
        tone === 'default' ? 'bg-card' : 'bg-subtle',
        className
      )}
    >
      {(title || action) && (
        <header className='flex items-center justify-between gap-3'>
          {title && <h2 className='type-section text-fg'>{title}</h2>}
          {action}
        </header>
      )}
      {children}
    </section>
  )
}
