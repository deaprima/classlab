import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { PendingPage } from '@/components/layout/page'

export const Route = createFileRoute('/_authenticated/admin/rooms')({
  beforeLoad: () => requireRole(ACCESS.manageRooms),
  component: () => <PendingPage title='Kelola Ruang' />,
})
