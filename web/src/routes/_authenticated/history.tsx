import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { positiveIntParam } from '@/lib/search-params'
import { History } from '@/features/my-bookings/history-page'

const searchSchema = z.object({
  page: positiveIntParam,
  status: z.enum(['SELESAI', 'DITOLAK']).optional().catch(undefined),
  /** YYYY-MM; absent means all time. */
  month: z
    .string()
    .regex(/^\d{4}-\d{2}$/)
    .optional()
    .catch(undefined),
})

export const Route = createFileRoute('/_authenticated/history')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.createBooking),
  component: History,
})
