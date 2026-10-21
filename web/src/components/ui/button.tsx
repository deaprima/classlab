import * as React from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-full text-sm font-medium transition-all disabled:pointer-events-none [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4 shrink-0 [&_svg]:shrink-0 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
  {
    variants: {
      variant: {
        default:
          'bg-primary text-primary-foreground hover:bg-primary-hover disabled:bg-disabled disabled:text-fg-tertiary disabled:opacity-100',
        accent:
          'bg-brand text-on-brand hover:bg-brand-hover active:bg-brand-pressed disabled:bg-disabled disabled:text-fg-tertiary disabled:opacity-100',
        // No red buttons (DESIGN.md section 9): a destructive action is a Primary button with a clear label.
        destructive:
          'bg-primary text-primary-foreground hover:bg-primary-hover disabled:bg-disabled disabled:text-fg-tertiary disabled:opacity-100',
        outline:
          'border border-line-strong bg-surface text-fg hover:border-line-hover disabled:border-line disabled:bg-disabled disabled:text-fg-tertiary disabled:opacity-100',
        secondary:
          'border border-line-strong bg-surface text-fg hover:border-line-hover disabled:border-line disabled:bg-disabled disabled:text-fg-tertiary disabled:opacity-100',
        ghost: 'text-fg hover:bg-muted disabled:text-fg-tertiary',
        link: 'text-primary underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-10 px-4 py-2 has-[>svg]:px-3',
        sm: 'h-8 gap-1.5 px-3 has-[>svg]:px-2.5',
        lg: 'h-10 px-6 has-[>svg]:px-4',
        icon: 'size-10',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  }
)

function Button({
  className,
  variant,
  size,
  asChild = false,
  ...props
}: React.ComponentProps<'button'> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot : 'button'

  return (
    <Comp
      data-slot='button'
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
