import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { positiveIntParam, uuidParam } from '@/lib/search-params'
import { Reports } from '@/features/reports'

const searchSchema = z.object({
  page: positiveIntParam,
  status: z
    .enum(['BARU', 'DITERIMA', 'DIPROSES', 'SELESAI'])
    .optional()
    .catch(undefined),
  room_id: uuidParam,
  /** Open detail panel; notifications link here. */
  report: uuidParam,
})

export const Route = createFileRoute('/_authenticated/reports/')({
  validateSearch: searchSchema,
  component: Reports,
})
