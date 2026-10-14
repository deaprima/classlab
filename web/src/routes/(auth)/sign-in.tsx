import { z } from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { ensureSession, safeRedirectPath } from '@/lib/session'
import { SignIn } from '@/features/auth/sign-in'

const searchSchema = z.object({
  redirect: z.string().optional(),
})

export const Route = createFileRoute('/(auth)/sign-in')({
  validateSearch: searchSchema,
  beforeLoad: async ({ search }) => {
    let signedIn: boolean
    try {
      signedIn = await ensureSession()
    } catch (_unreachable) {
      // Server unreachable: show the form anyway; submitting it reports the real error.
      signedIn = false
    }
    if (signedIn) {
      throw redirect({ href: safeRedirectPath(search.redirect) })
    }
  },
  component: SignIn,
})
