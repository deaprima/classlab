import { ChevronLeft, ChevronRight } from 'lucide-react'
import { type PaginationMeta } from '@/lib/api-types'
import { Button } from '@/components/ui/button'

/** Previous/next controls for the API's `{ page, per_page, total }` lists. */
export function Pager({
  meta,
  onPageChange,
}: {
  meta: PaginationMeta | undefined
  onPageChange: (page: number) => void
}) {
  const page = meta?.page ?? 1
  const perPage = meta?.per_page ?? 20
  const total = meta?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / perPage))
  if (total <= perPage && page === 1) return null

  return (
    <nav
      aria-label='Halaman'
      className='flex items-center justify-between gap-4 pt-4'
    >
      <p className='text-sm text-muted-foreground'>
        Halaman {page} dari {pages} · {total} data
      </p>
      <div className='flex gap-2'>
        <Button
          variant='outline'
          size='sm'
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
        >
          <ChevronLeft />
          Sebelumnya
        </Button>
        <Button
          variant='outline'
          size='sm'
          disabled={page >= pages}
          onClick={() => onPageChange(page + 1)}
        >
          Berikutnya
          <ChevronRight />
        </Button>
      </div>
    </nav>
  )
}
