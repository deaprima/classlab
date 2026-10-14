import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { PendingPage } from '@/components/layout/page'

export const Route = createFileRoute('/_authenticated/bookings/')({
  beforeLoad: () => requireRole(ACCESS.viewBookings),
  component: () => <PendingPage title='Riwayat Peminjaman' />,
})
