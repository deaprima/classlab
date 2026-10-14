import { useNavigate } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { type AppNotification } from '@/lib/api-types'
import { formatDateTime, formatRelative } from '@/lib/datetime'
import { cn } from '@/lib/utils'
import { useMarkRead } from '../api'
import { notificationTarget } from '../links'

/**
 * One notification. Activating it marks it read and opens what it is about; a notification with
 * nowhere to go is just marked read.
 */
export function NotificationItem({
  notification: n,
  compact = false,
  onOpen,
}: {
  notification: AppNotification
  compact?: boolean
  onOpen?: () => void
}) {
  const navigate = useNavigate()
  const role = useAuthStore((s) => s.auth.user?.role)
  const markRead = useMarkRead()
  const target = notificationTarget(n, role)
  const unread = !n.read_at

  const open = () => {
    if (unread) markRead.mutate(n.id)
    onOpen?.()
    if (target) navigate(target)
  }

  return (
    <button
      type='button'
      onClick={open}
      className={cn(
        'flex w-full gap-3 rounded-md px-3 py-2 text-start text-sm transition-colors hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
        unread && 'bg-primary/5'
      )}
    >
      <span
        className={cn(
          'mt-1.5 size-2 shrink-0 rounded-full',
          unread ? 'bg-primary' : 'bg-transparent'
        )}
        aria-hidden
      />
      <span className='grid flex-1 gap-0.5'>
        <span className={cn('leading-snug', unread && 'font-semibold')}>
          {n.title}
          {unread && <span className='sr-only'> (belum dibaca)</span>}
        </span>
        <span
          className={cn('text-muted-foreground', compact && 'line-clamp-2')}
        >
          {n.body}
        </span>
        <time
          dateTime={n.created_at}
          title={formatDateTime(n.created_at)}
          className='text-xs text-muted-foreground'
        >
          {formatRelative(n.created_at)}
        </time>
      </span>
    </button>
  )
}
