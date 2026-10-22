import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { positiveIntParam, uuidParam } from '@/lib/search-params'
import { Audit } from '@/features/audit'
import { ENTITY_GROUPS, RANGES } from '@/features/audit/rules'

const searchSchema = z.object({
  page: positiveIntParam,
  entity: z
    .enum(ENTITY_GROUPS.map((g) => g.value) as [string, ...string[]])
    .optional()
    .catch(undefined),
  range: z.enum(RANGES).optional().catch(undefined),
  /** The entry shown in the detail panel. */
  log: uuidParam,
})

export const Route = createFileRoute('/_authenticated/admin/audit')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.viewAudit),
  component: Audit,
})
