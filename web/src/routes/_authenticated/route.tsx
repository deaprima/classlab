import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ensureSession, passwordChangeRedirect } from '@/lib/session'
import { AuthenticatedGate } from '@/components/layout/authenticated-gate'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location }) => {
    // "/" is the one page a visitor may open without a session: it is the landing page (G4).
    // Everything else under here needs a session and sends the visitor to sign in.
    const isHome = location.pathname === '/'
    let signedIn: boolean
    try {
      signedIn = await ensureSession()
    } catch (error) {
      // An unreachable server must not hide the landing page; any other page reports the error.
      if (!isHome) throw error
      signedIn = false
    }
    if (!signedIn) {
      if (isHome) return { guest: true }
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
    return { guest: false }
  },
  component: AuthenticatedGate,
})
