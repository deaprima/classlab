import * as React from 'react'
import { cn } from '@/lib/utils'

function Textarea({ className, ...props }: React.ComponentProps<'textarea'>) {
  return (
    <textarea
      data-slot='textarea'
      className={cn(
        'flex field-sizing-content min-h-16 w-full rounded-md border border-input bg-surface px-3 py-2 text-base transition-[color,box-shadow] outline-none placeholder:text-fg-tertiary hover:border-line-hover focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/30 disabled:cursor-not-allowed disabled:bg-disabled disabled:text-fg-tertiary aria-invalid:border-line-error aria-invalid:ring-2 aria-invalid:ring-line-error/20 md:text-sm',
        className
      )}
      {...props}
    />
  )
}

export { Textarea }
