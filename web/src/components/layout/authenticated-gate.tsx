import { getRouteApi } from '@tanstack/react-router'
import { Landing } from '@/features/landing'
import { AuthenticatedLayout } from './authenticated-layout'

const route = getRouteApi('/_authenticated')

/**
 * What `/_authenticated` renders: the app shell for a signed-in user, and for a visitor at "/" the
 * landing page instead (the route's `beforeLoad` marks that visitor as `guest`).
 */
export function AuthenticatedGate() {
  const { guest } = route.useRouteContext()
  return guest ? <Landing /> : <AuthenticatedLayout />
}
