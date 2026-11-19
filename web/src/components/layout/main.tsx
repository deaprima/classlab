import { cn } from '@/lib/utils'

type MainProps = React.HTMLAttributes<HTMLElement> & {
  fixed?: boolean
  ref?: React.Ref<HTMLElement>
}

/** Page content: full width beside the sidebar with the Figma 24 px padding (16 px on mobile). */
export function Main({ fixed, className, ...props }: MainProps) {
  return (
    <main
      data-layout={fixed ? 'fixed' : 'auto'}
      className={cn(
        'w-full px-4 py-5 md:px-6',

        // If layout is fixed, make the main container flex and grow
        fixed && 'flex grow flex-col overflow-hidden',
        className
      )}
      {...props}
    />
  )
}
