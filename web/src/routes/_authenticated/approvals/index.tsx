import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { positiveIntParam, uuidParam } from '@/lib/search-params'
import { Approvals } from '@/features/approvals'

const searchSchema = z.object({
  page: positiveIntParam,
  status: z
    .enum(['MENUNGGU', 'DISETUJUI', 'DITOLAK'])
    .optional()
    .catch(undefined),
  type: z
    .enum(['KELAS', 'LAB', 'AUDITORIUM', 'RUANG_UJIAN', 'RUANG_SIDANG'])
    .optional()
    .catch(undefined),
  /** Free-text filter of the queue. */
  q: z.string().optional().catch(undefined),
  booking: uuidParam,
})

export const Route = createFileRoute('/_authenticated/approvals/')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.approveBookings),
  component: Approvals,
})
