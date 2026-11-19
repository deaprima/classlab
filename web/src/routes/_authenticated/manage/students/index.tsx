import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { positiveIntParam } from '@/lib/search-params'
import { Students } from '@/features/students'

const searchSchema = z.object({
  page: positiveIntParam,
  q: z.string().optional().catch(undefined),
  status: z.enum(['active', 'inactive']).optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/manage/students/')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.manageStudents),
  component: Students,
})
