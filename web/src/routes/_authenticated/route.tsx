import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ensureSession, passwordChangeRedirect } from '@/lib/session'
import { AuthenticatedLayout } from '@/components/layout/authenticated-layout'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location }) => {
    if (!(await ensureSession())) {
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
      })
    }
    // A temporary password must be replaced first; every other endpoint answers 403 until then.
    const to = passwordChangeRedirect(
      useAuthStore.getState().auth.user,
      location.pathname
    )
    if (to) throw redirect({ to, search: { redirect: location.href } })
  },
  component: AuthenticatedLayout,
})
