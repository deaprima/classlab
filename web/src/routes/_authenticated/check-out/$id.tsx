import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { CheckOutPage } from '@/features/attendance/pages'

export const Route = createFileRoute('/_authenticated/check-out/$id')({
  beforeLoad: () => requireRole(ACCESS.createBooking),
  component: CheckOutPage,
})
