import { z } from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { ensureSession } from '@/lib/session'
import { ChangePasswordPage } from '@/features/auth/change-password/page'

const searchSchema = z.object({
  redirect: z.string().optional(),
})

// Outside `_authenticated` on purpose: an account with a temporary password sees this page and no shell.
export const Route = createFileRoute('/change-password')({
  validateSearch: searchSchema,
  beforeLoad: async ({ location }) => {
    if (!(await ensureSession())) {
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
      })
    }
  },
  component: ChangePasswordPage,
})
