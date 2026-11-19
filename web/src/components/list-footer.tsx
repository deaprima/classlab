import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'

/** "1–10 dari 23" with previous/next icon buttons; hidden when everything fits on one page. */
export function ListFooter({
  page,
  perPage,
  total,
  onPageChange,
}: {
  page: number
  perPage: number
  total: number
  onPageChange: (page: number) => void
}) {
  // One page needs no controls.
  if (total <= perPage) return null
  const pages = Math.ceil(total / perPage)
  const from = (page - 1) * perPage + 1
  const to = Math.min(total, page * perPage)
  return (
    <nav aria-label='Halaman' className='flex items-center justify-end gap-3'>
      <p className='type-body-sm text-fg-secondary'>
        {from}–{to} dari {total}
      </p>
      <Button
        variant='secondary'
        size='icon'
        aria-label='Halaman sebelumnya'
        disabled={page <= 1}
        onClick={() => onPageChange(page - 1)}
      >
        <ChevronLeft />
      </Button>
      <Button
        variant='secondary'
        size='icon'
        aria-label='Halaman berikutnya'
        disabled={page >= pages}
        onClick={() => onPageChange(page + 1)}
      >
        <ChevronRight />
      </Button>
    </nav>
  )
}
