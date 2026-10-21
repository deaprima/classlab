import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { CheckOut } from '@/features/attendance/check-out-page'

export const Route = createFileRoute('/_authenticated/check-out/$id')({
  beforeLoad: () => requireRole(ACCESS.createBooking),
  component: CheckOutRoute,
})

function CheckOutRoute() {
  const { id } = Route.useParams()
  return <CheckOut bookingId={id} />
}
