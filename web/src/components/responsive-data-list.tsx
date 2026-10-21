import { cn } from '@/lib/utils'
import { useIsMobile } from '@/hooks/use-mobile'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

export type DataColumn<T> = {
  key: string
  header: string
  cell: (row: T) => React.ReactNode
  /** Shown as the card title on mobile instead of a labelled line (one column at most). */
  primary?: boolean
  /** Not shown as a labelled line on mobile (e.g. an actions column rendered at the card's end). */
  actions?: boolean
  className?: string
}

type ResponsiveDataListProps<T> = {
  rows: readonly T[]
  columns: readonly DataColumn<T>[]
  rowKey: (row: T) => string
  /** Caption read by screen readers; hidden visually. */
  caption: string
  /** Forces a layout; by default a table from 768 px up and cards below. */
  layout?: 'table' | 'cards'
  className?: string
}

/** A table on desktop that turns into a stack of cards on mobile, so no page scrolls sideways. */
export function ResponsiveDataList<T>({
  rows,
  columns,
  rowKey,
  caption,
  layout,
  className,
}: ResponsiveDataListProps<T>) {
  const isMobile = useIsMobile()
  const mode = layout ?? (isMobile ? 'cards' : 'table')

  if (mode === 'table') {
    return (
      <div
        className={cn('overflow-hidden rounded-2xl border bg-card', className)}
      >
        <Table>
          <caption className='sr-only'>{caption}</caption>
          <TableHeader>
            <TableRow className='hover:bg-transparent'>
              {columns.map((col) => (
                <TableHead
                  scope='col'
                  key={col.key}
                  className={cn(
                    'h-12 px-5 type-table-header text-fg-secondary',
                    col.className
                  )}
                >
                  {col.actions ? (
                    <span className='sr-only'>{col.header}</span>
                  ) : (
                    col.header
                  )}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={rowKey(row)}>
                {columns.map((col) => (
                  <TableCell
                    key={col.key}
                    className={cn('px-5 py-3', col.className)}
                  >
                    {col.cell(row)}
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    )
  }

  const primary = columns.find((col) => col.primary)
  const actions = columns.filter((col) => col.actions)
  const details = columns.filter((col) => col !== primary && !col.actions)

  return (
    <ul aria-label={caption} className={cn('flex flex-col gap-3', className)}>
      {rows.map((row) => (
        <li
          key={rowKey(row)}
          className='flex flex-col gap-3 rounded-2xl border bg-card p-4'
        >
          {primary && (
            <div className='type-card-title text-fg'>{primary.cell(row)}</div>
          )}
          <dl className='flex flex-col gap-2'>
            {details.map((col) => (
              <div
                key={col.key}
                className='flex items-center justify-between gap-4'
              >
                <dt className='shrink-0 type-label text-fg-secondary'>
                  {col.header}
                </dt>
                <dd className='min-w-0 text-end type-body text-fg'>
                  {col.cell(row)}
                </dd>
              </div>
            ))}
          </dl>
          {actions.length > 0 && (
            <div className='flex flex-wrap justify-end gap-2'>
              {actions.map((col) => (
                <div key={col.key}>{col.cell(row)}</div>
              ))}
            </div>
          )}
        </li>
      ))}
    </ul>
  )
}
