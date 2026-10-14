import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { PendingPage } from '@/components/layout/page'

export const Route = createFileRoute('/_authenticated/admin/schedules')({
  beforeLoad: () => requireRole(ACCESS.manageSchedules),
  component: () => <PendingPage title='Kelola Jadwal' />,
})
