import * as React from 'react'
import { cn } from '@/lib/utils'

function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      type={type}
      data-slot='input'
      className={cn(
        'flex h-10 w-full min-w-0 rounded-md border border-input bg-surface px-3 py-1 text-base transition-[color,box-shadow] outline-none selection:bg-primary selection:text-primary-foreground file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-fg-tertiary hover:border-line-hover disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-fg-tertiary md:text-sm',
        'focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/30 focus-visible:outline-none',
        'aria-invalid:border-line-error aria-invalid:ring-2 aria-invalid:ring-line-error/20',
        className
      )}
      {...props}
    />
  )
}

export { Input }
