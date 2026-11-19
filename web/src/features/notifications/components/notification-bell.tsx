import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { Bell, CheckCheck } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Skeleton } from '@/components/ui/skeleton'
import { useMarkAllRead, useNotifications, useUnreadCount } from '../api'
import { NotificationItem } from './notification-item'

const PREVIEW = 6

/** Header badge (brief.md 7.2) with the latest notifications in a popover. */
export function NotificationBell() {
  const [open, setOpen] = useState(false)
  const unread = useUnreadCount()
  const count = unread.data ?? 0

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant='secondary'
          size='icon'
          className='relative'
          aria-label={
            count > 0 ? `Notifikasi, ${count} belum dibaca` : 'Notifikasi'
          }
        >
          <Bell className='size-[1.2rem]' />
          {count > 0 && (
            <span
              className='absolute end-2 top-2 size-2 rounded-full bg-indicator'
              aria-hidden
            />
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align='end'
        collisionPadding={16}
        className='w-[min(22rem,calc(100vw-2rem))] p-0'
      >
        {open && <BellContent count={count} close={() => setOpen(false)} />}
      </PopoverContent>
    </Popover>
  )
}

function BellContent({ count, close }: { count: number; close: () => void }) {
  const latest = useNotifications({ page: 1, per_page: PREVIEW })
  const markAll = useMarkAllRead()

  return (
    <>
      <div className='flex items-center justify-between border-b px-3 py-2'>
        <p className='text-sm font-semibold'>Notifikasi</p>
        {count > 0 && (
          <Button
            variant='ghost'
            size='sm'
            disabled={markAll.isPending}
            onClick={() => markAll.mutate()}
          >
            <CheckCheck />
            Tandai semua dibaca
          </Button>
        )}
      </div>
      <div className='max-h-96 overflow-y-auto p-1'>
        {latest.isPending && <Skeleton className='m-2 h-24' />}
        {latest.isError && (
          <p className='p-3 text-sm text-destructive'>
            {getErrorMessage(latest.error)}
          </p>
        )}
        {latest.data?.data.length === 0 && (
          <p className='p-6 text-center text-sm text-muted-foreground'>
            Belum ada notifikasi.
          </p>
        )}
        {latest.data?.data.map((n) => (
          <NotificationItem
            key={n.id}
            notification={n}
            compact
            onOpen={close}
          />
        ))}
      </div>
      <div className='border-t p-1'>
        <Button variant='ghost' size='sm' className='w-full' asChild>
          <Link to='/notifications' onClick={close}>
            Lihat semua notifikasi
          </Link>
        </Button>
      </div>
    </>
  )
}
