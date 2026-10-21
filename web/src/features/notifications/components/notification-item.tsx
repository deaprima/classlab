import { useNavigate } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { type AppNotification } from '@/lib/api-types'
import { formatDateTime, formatRelative } from '@/lib/datetime'
import { cn } from '@/lib/utils'
import { useMarkRead } from '../api'
import { notificationTarget } from '../links'

/**
 * One notification. Activating it marks it read and opens what it is about; a notification with
 * nowhere to go is just marked read. Unread ones carry the orange dot and a heavier title.
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
        'flex min-h-11 w-full gap-3 rounded-xl px-3 py-3 text-start transition-colors hover:bg-subtle focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
        unread && 'bg-accent-subtle/60 hover:bg-accent-subtle'
      )}
    >
      <span
        className={cn(
          'mt-1.5 size-2 shrink-0 rounded-full',
          unread ? 'bg-indicator' : 'bg-transparent'
        )}
        aria-hidden
      />
      <span className='grid min-w-0 flex-1 gap-0.5'>
        <span
          className={cn(
            'type-body leading-snug text-fg',
            unread && 'font-semibold'
          )}
        >
          {n.title}
          {unread && <span className='sr-only'> (belum dibaca)</span>}
        </span>
        <span
          className={cn(
            'type-body-sm text-fg-secondary',
            compact && 'line-clamp-2'
          )}
        >
          {n.body}
        </span>
        <time
          dateTime={n.created_at}
          title={formatDateTime(n.created_at)}
          className='type-label-sm text-fg-tertiary'
        >
          {formatRelative(n.created_at)}
        </time>
      </span>
    </button>
  )
}
