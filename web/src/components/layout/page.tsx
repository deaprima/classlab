import { type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { NotificationBell } from '@/features/notifications/components/notification-bell'
import { Header } from './header'
import { Main } from './main'

type PageProps = {
  title: string
  /** Title in the Top Bar when it differs from the heading (Dashboard vs "Hai, Nisrina"). */
  barTitle?: string
  description?: ReactNode
  /** Buttons shown next to the title; on mobile they become a bar fixed to the bottom. */
  actions?: ReactNode
  children: ReactNode
}

/** DESIGN.md 3.1: Top Bar, then the page header (title and subtitle), then the content. */
export function Page({
  title,
  barTitle,
  description,
  actions,
  children,
}: PageProps) {
  // The Top Bar already shows the title on desktop. With nothing else in the header it would be said
  // twice, so the heading stays for screen readers and for mobile (whose bar has no title).
  const repeatsBar = !description && !actions && !barTitle
  return (
    <>
      <Header fixed>
        <p className='hidden min-w-0 truncate type-section text-fg md:block'>
          {barTitle ?? title}
        </p>
        <Search className='ms-auto' />
        <NotificationBell />
        <ProfileDropdown />
      </Header>
      <Main id='content' tabIndex={-1} className='outline-none'>
        <div
          className={cn(
            'mb-5 flex flex-wrap items-end justify-between gap-4',
            repeatsBar && 'md:mb-0'
          )}
        >
          <div className='min-w-0 space-y-1'>
            <h1
              className={cn(
                'type-page-title text-fg',
                repeatsBar && 'md:sr-only'
              )}
            >
              {title}
            </h1>
            {description && (
              <p className='type-body text-fg-secondary'>{description}</p>
            )}
          </div>
          {actions && (
            <div
              data-slot='page-actions'
              className={cn(
                'flex flex-wrap gap-2',
                // Mobile: a sticky action bar (Check-in mobile frame in Figma) that clears the iOS home bar.
                'max-md:fixed max-md:inset-x-0 max-md:bottom-0 max-md:z-40 max-md:flex-nowrap max-md:border-t max-md:bg-surface max-md:p-4 max-md:pb-[max(1rem,env(safe-area-inset-bottom))] max-md:*:flex-1'
              )}
            >
              {actions}
            </div>
          )}
        </div>
        {children}
      </Main>
    </>
  )
}
