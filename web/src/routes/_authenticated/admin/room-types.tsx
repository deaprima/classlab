import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { Policies } from '@/features/policies'

export const Route = createFileRoute('/_authenticated/admin/room-types')({
  beforeLoad: () => requireRole(ACCESS.managePolicies),
  component: Policies,
})
