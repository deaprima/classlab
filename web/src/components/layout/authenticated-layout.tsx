import { Outlet } from '@tanstack/react-router'
import { cn } from '@/lib/utils'
import { SearchProvider } from '@/context/search-provider'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { AppSidebar } from '@/components/layout/app-sidebar'
import { Footer } from '@/components/layout/footer'
import { SkipToMain } from '@/components/skip-to-main'
import { useNotificationStream } from '@/features/notifications/use-notification-stream'

type AuthenticatedLayoutProps = {
  children?: React.ReactNode
}

export function AuthenticatedLayout({ children }: AuthenticatedLayoutProps) {
  useNotificationStream()
  return (
    <SearchProvider>
      {/* The desktop sidebar is fixed (DESIGN.md 3.1): controlled open, so neither Ctrl+B nor a cookie left by the old layout can hide it. Mobile uses the drawer. */}
      <SidebarProvider open onOpenChange={() => {}}>
        <SkipToMain />
        <AppSidebar />
        <SidebarInset
          className={cn(
            // Set content container, so we can use container queries
            '@container/content',

            // The mobile action bar of a page is fixed over the bottom; keep the footer clear of it
            'max-md:has-data-[slot=page-actions]:[&>footer]:pb-24',

            // If layout is fixed, set the height
            // to 100svh to prevent overflow
            'has-data-[layout=fixed]:h-svh',

            // If layout is fixed and sidebar is inset,
            // set the height to 100svh - spacing (total margins) to prevent overflow
            'peer-data-[variant=inset]:has-data-[layout=fixed]:h-[calc(100svh-(var(--spacing)*4))]'
          )}
        >
          {children ?? <Outlet />}
          <Footer />
        </SidebarInset>
      </SidebarProvider>
    </SearchProvider>
  )
}
