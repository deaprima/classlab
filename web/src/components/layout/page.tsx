import { type ReactNode } from 'react'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { NotificationBell } from '@/features/notifications/components/notification-bell'
import { Header } from './header'
import { Main } from './main'

type PageProps = {
  title: string
  description?: ReactNode
  /** Buttons shown next to the title. */
  actions?: ReactNode
  children: ReactNode
}

/** Header bar plus a titled main area; every ClassLab page uses it. */
export function Page({ title, description, actions, children }: PageProps) {
  return (
    <>
      <Header fixed>
        <Search className='me-auto' />
        <NotificationBell />
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>
      <Main>
        <div className='mb-6 flex flex-wrap items-end justify-between gap-4'>
          <div className='space-y-1'>
            <h1 className='text-2xl font-bold tracking-tight'>{title}</h1>
            {description && (
              <p className='text-muted-foreground'>{description}</p>
            )}
          </div>
          {actions && <div className='flex flex-wrap gap-2'>{actions}</div>}
        </div>
        {children}
      </Main>
    </>
  )
}

/** Temporary body for pages that are routed and guarded but not built yet. */
export function PendingPage({ title }: { title: string }) {
  return (
    <Page title={title}>
      <p className='text-muted-foreground'>Halaman ini sedang dikerjakan.</p>
    </Page>
  )
}
