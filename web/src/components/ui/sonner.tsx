import { Toaster as Sonner, ToasterProps } from 'sonner'
import { useTheme } from '@/context/theme-provider'

export function Toaster({ ...props }: ToasterProps) {
  const { resolvedTheme } = useTheme()

  return (
    <Sonner
      theme={resolvedTheme}
      className='toaster group [&_div[data-content]]:w-full'
      toastOptions={{ classNames: { toast: '!rounded-2xl !px-5' } }}
      style={
        {
          '--normal-bg': 'var(--bg-inverse)',
          '--normal-text': 'var(--text-inverse)',
          '--normal-border': 'var(--bg-inverse)',
        } as React.CSSProperties
      }
      {...props}
    />
  )
}
