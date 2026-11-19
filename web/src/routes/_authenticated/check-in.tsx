import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { CheckInPage } from '@/features/attendance/pages'

// The QR on the door encodes `/check-in?room=<code>&t=<token>`. A signed-out visitor is sent to
// sign-in with this URL as `redirect`, so the scan survives the login.
const searchSchema = z.object({
  room: z.string().optional().catch(undefined),
  t: z.string().optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/check-in')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.createBooking),
  component: CheckInPage,
})
