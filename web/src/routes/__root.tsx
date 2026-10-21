import { type QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { Toaster } from '@/components/ui/sonner'
import { NavigationProgress } from '@/components/navigation-progress'
import { SessionExpiredDialog } from '@/features/auth/session-expired-dialog'
import { GeneralError } from '@/features/errors/general-error'
import { NotFoundError } from '@/features/errors/not-found-error'

export const Route = createRootRouteWithContext<{
  queryClient: QueryClient
}>()({
  component: () => {
    return (
      <>
        <NavigationProgress />
        <Outlet />
        <SessionExpiredDialog />
        <Toaster duration={5000} />
        {import.meta.env.MODE === 'development' &&
          import.meta.env.VITE_SHOW_DEVTOOLS === 'true' && (
            <>
              {/* Both toggles sit bottom right: Router in its corner, Query's logo just left of it. */}
              <div className='fixed right-40 bottom-3 z-50'>
                <ReactQueryDevtools buttonPosition='relative' />
              </div>
              <TanStackRouterDevtools position='bottom-right' />
            </>
          )}
      </>
    )
  },
  notFoundComponent: NotFoundError,
  errorComponent: GeneralError,
})
