import { SearchIcon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useSearch } from '@/context/search-provider'

/** Figma `Search Input` in the Top Bar; opens the command menu. */
export function Search({
  className = '',
  placeholder = 'Cari halaman',
  ...props
}: React.ComponentProps<'button'> & { placeholder?: string }) {
  const { setOpen } = useSearch()
  return (
    <button
      type='button'
      {...props}
      className={cn(
        'group relative flex h-10 items-center gap-2 rounded-full border bg-subtle px-3 type-body text-fg-tertiary outline-none hover:border-line-hover focus-visible:ring-2 focus-visible:ring-ring',
        'max-md:size-10 max-md:justify-center max-md:px-0 md:w-56 lg:w-72',
        className
      )}
      aria-label='Cari halaman atau perintah'
      aria-keyshortcuts='Meta+K Control+K'
      onClick={() => setOpen(true)}
    >
      <SearchIcon aria-hidden className='size-4 shrink-0' />
      <span className='max-md:hidden'>{placeholder}</span>
      <kbd className='pointer-events-none ms-auto hidden rounded-full border bg-surface px-2 py-0.5 type-label-sm select-none lg:block'>
        Ctrl K
      </kbd>
    </button>
  )
}
