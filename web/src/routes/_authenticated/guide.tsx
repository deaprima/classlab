import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { Guide } from '@/features/guide'

export const Route = createFileRoute('/_authenticated/guide')({
  beforeLoad: () => requireRole(ACCESS.createBooking),
  component: Guide,
})
