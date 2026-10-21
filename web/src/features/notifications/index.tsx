import { getRouteApi } from '@tanstack/react-router'
import { AlertCircle, Bell, CheckCheck } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/layout/page'
import { ListFooter } from '@/components/list-footer'
import { SegmentedControl } from '@/components/segmented-control'
import { useMarkAllRead, useNotifications, useUnreadCount } from './api'
import { NotificationItem } from './components/notification-item'

const route = getRouteApi('/_authenticated/notifications/')
const PER_PAGE = 20

type Tab = 'all' | 'unread'

/** Notifikasi (Figma): the full list, all or only unread, newest first. */
export function Notifications() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const tab: Tab = search.filter === 'unread' ? 'unread' : 'all'
  const page = search.page ?? 1
  const list = useNotifications({
    page,
    per_page: PER_PAGE,
    ...(tab === 'unread' && { unread: true }),
  })
  const unread = useUnreadCount()
  const markAll = useMarkAllRead()
  const items = list.data?.data ?? []

  return (
    <Page
      title='Notifikasi'
      description={
        unread.data
          ? `${unread.data} notifikasi belum dibaca.`
          : 'Semua notifikasi sudah dibaca.'
      }
      actions={
        !!unread.data && (
          <Button
            variant='secondary'
            disabled={markAll.isPending}
            onClick={() => markAll.mutate()}
          >
            <CheckCheck />
            Tandai semua dibaca
          </Button>
        )
      }
    >
      <section
        aria-label='Daftar notifikasi'
        className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
      >
        <SegmentedControl
          aria-label='Filter notifikasi'
          value={tab}
          onValueChange={(t) =>
            navigate({
              search: { filter: t === 'unread' ? 'unread' : undefined },
            })
          }
          options={[
            { value: 'all', label: 'Semua' },
            { value: 'unread', label: 'Belum dibaca', count: unread.data },
          ]}
        />

        {list.isError && (
          <Alert variant='error'>
            <AlertCircle />
            <AlertDescription>{getErrorMessage(list.error)}</AlertDescription>
          </Alert>
        )}

        {list.isPending && <Skeleton className='h-40' />}
        {list.isSuccess && items.length === 0 && (
          <EmptyState
            icon={Bell}
            title={
              tab === 'unread'
                ? 'Tidak ada yang belum dibaca'
                : 'Belum ada notifikasi'
            }
            description='Kabar soal pengajuan, check-in, dan laporanmu muncul di sini.'
          />
        )}
        {items.length > 0 && (
          <ul className='flex flex-col gap-1'>
            {items.map((n) => (
              <li key={n.id}>
                <NotificationItem notification={n} />
              </li>
            ))}
          </ul>
        )}

        <ListFooter
          page={page}
          perPage={PER_PAGE}
          total={list.data?.meta?.total ?? 0}
          onPageChange={(p) =>
            navigate({ search: (prev) => ({ ...prev, page: p }) })
          }
        />
      </section>
    </Page>
  )
}
