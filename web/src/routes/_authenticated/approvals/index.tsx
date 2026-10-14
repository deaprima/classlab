import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { positiveIntParam, uuidParam } from '@/lib/search-params'
import { Approvals } from '@/features/approvals'

const searchSchema = z.object({
  page: positiveIntParam,
  booking: uuidParam,
})

export const Route = createFileRoute('/_authenticated/approvals/')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.approveBookings),
  component: Approvals,
})
