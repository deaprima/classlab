import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { PendingPage } from '@/components/layout/page'

export const Route = createFileRoute('/_authenticated/approvals/')({
  beforeLoad: () => requireRole(ACCESS.approveBookings),
  component: () => <PendingPage title='Persetujuan Peminjaman' />,
})
