import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { positiveIntParam } from '@/lib/search-params'
import { AdminUsers } from '@/features/admin/users'

const searchSchema = z.object({
  page: positiveIntParam,
  q: z.string().optional().catch(undefined),
  role: z
    .enum(['MAHASISWA', 'ORGANISASI', 'TU', 'KALAB', 'OB', 'ADMIN'])
    .optional()
    .catch(undefined),
  status: z.enum(['active', 'inactive']).optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/admin/users')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.manageUsers),
  component: AdminUsers,
})
