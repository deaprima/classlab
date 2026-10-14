import { StrictMode } from 'react'
import ReactDOM from 'react-dom/client'
import { AxiosError } from 'axios'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { setSessionExpiredHandler } from '@/lib/api-client'
import { handleServerError } from '@/lib/handle-server-error'
import { DirectionProvider } from './context/direction-provider'
import { FontProvider } from './context/font-provider'
import { ThemeProvider } from './context/theme-provider'
// Generated Routes
import { routeTree } from './routeTree.gen'
// Styles
import './styles/index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // 4xx answers will not change on retry; network and 5xx errors might.
      retry: (failureCount, error) => {
        if (error instanceof AxiosError) {
          const status = error.response?.status ?? 0
          if (status >= 400 && status < 500) return false
        }
        return failureCount < 2
      },
      refetchOnWindowFocus: import.meta.env.PROD,
      staleTime: 10 * 1000, // 10s
    },
    mutations: {
      // Forms that map field errors pass their own onError, which replaces this one.
      onError: handleServerError,
    },
  },
})

// Create a new router instance
const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: 'intent',
  defaultPreloadStaleTime: 0,
})

setSessionExpiredHandler(() => {
  const { auth } = useAuthStore.getState()
  // Several requests can fail together; only the first one redirects.
  if (!auth.user && !auth.accessToken) return
  auth.reset()
  queryClient.clear()
  toast.error('Sesi Anda telah berakhir. Silakan masuk kembali.')
  router.navigate({
    to: '/sign-in',
    search: { redirect: router.history.location.href },
    replace: true,
  })
})

// Register the router instance for type safety
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

// Render the app
const rootElement = document.getElementById('root')!
if (!rootElement.innerHTML) {
  const root = ReactDOM.createRoot(rootElement)
  root.render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <ThemeProvider>
          <FontProvider>
            <DirectionProvider>
              <RouterProvider router={router} />
            </DirectionProvider>
          </FontProvider>
        </ThemeProvider>
      </QueryClientProvider>
    </StrictMode>
  )
}
