import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { Imports } from '@/features/imports'

const searchSchema = z.object({
  /** Which import is open; the class schedule when missing. */
  tab: z.enum(['SCHEDULES', 'USERS']).optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/manage/imports')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.manageImports),
  component: Imports,
})
