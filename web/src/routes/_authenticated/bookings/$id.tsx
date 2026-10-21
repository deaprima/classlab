import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { BookingDetailPage } from '@/features/my-bookings/booking-detail-page'

export const Route = createFileRoute('/_authenticated/bookings/$id')({
  beforeLoad: () => requireRole(ACCESS.viewBookings),
  component: BookingDetailPage,
})
