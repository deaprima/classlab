import { getRouteApi } from '@tanstack/react-router'
import { AlertCircle, CheckCheck } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Page } from '@/components/layout/page'
import { Pager } from '@/components/pager'
import { useMarkAllRead, useNotifications, useUnreadCount } from './api'
import { NotificationItem } from './components/notification-item'

const route = getRouteApi('/_authenticated/notifications/')
const PER_PAGE = 20

export function Notifications() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const onlyUnread = search.filter === 'unread'
  const list = useNotifications({
    page: search.page ?? 1,
    per_page: PER_PAGE,
    ...(onlyUnread && { unread: true }),
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
            variant='outline'
            disabled={markAll.isPending}
            onClick={() => markAll.mutate()}
          >
            <CheckCheck />
            Tandai semua dibaca
          </Button>
        )
      }
    >
      <Tabs
        value={onlyUnread ? 'unread' : 'all'}
        onValueChange={(v) =>
          navigate({
            search: { filter: v === 'unread' ? 'unread' : undefined },
          })
        }
        className='mb-4'
      >
        <TabsList>
          <TabsTrigger value='all'>Semua</TabsTrigger>
          <TabsTrigger value='unread'>Belum dibaca</TabsTrigger>
        </TabsList>
      </Tabs>

      {list.isError && (
        <Alert variant='destructive' className='mb-4'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(list.error)}</AlertDescription>
        </Alert>
      )}

      <Card className='max-w-3xl py-2'>
        <CardContent className='px-2'>
          {list.isPending && <Skeleton className='h-40' />}
          {list.isSuccess && items.length === 0 && (
            <p className='py-10 text-center text-muted-foreground'>
              {onlyUnread
                ? 'Tidak ada notifikasi yang belum dibaca.'
                : 'Belum ada notifikasi.'}
            </p>
          )}
          <ul className='divide-y'>
            {items.map((n) => (
              <li key={n.id} className='py-1'>
                <NotificationItem notification={n} />
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>

      <div className='max-w-3xl'>
        <Pager
          meta={list.data?.meta}
          onPageChange={(page) =>
            navigate({ search: (prev) => ({ ...prev, page }) })
          }
        />
      </div>
    </Page>
  )
}
