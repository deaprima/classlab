import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ACCESS, hasRole } from '@/lib/rbac'
import { Dashboard } from '@/features/dashboard'

export const Route = createFileRoute('/_authenticated/')({
  // Only requesters have a dashboard so far; the other roles' beranda is built later and until then
  // they land on the room schedule, as before.
  beforeLoad: () => {
    if (!hasRole(useAuthStore.getState().auth.user, ACCESS.createBooking)) {
      throw redirect({ to: '/schedule' })
    }
  },
  component: Dashboard,
})
