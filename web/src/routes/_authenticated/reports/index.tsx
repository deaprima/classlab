import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { positiveIntParam, uuidParam } from '@/lib/search-params'
import { Reports } from '@/features/reports'
import { ROOM_TYPES } from '@/features/rooms/labels'

const searchSchema = z.object({
  page: positiveIntParam,
  status: z
    .enum(['BARU', 'DITERIMA', 'DIPROSES', 'SELESAI'])
    .optional()
    .catch(undefined),
  room_id: uuidParam,
  /** Board filter by room type (managers). */
  type: z.enum(ROOM_TYPES).optional().catch(undefined),
  /** Open detail panel; notifications link here. */
  report: uuidParam,
})

export const Route = createFileRoute('/_authenticated/reports/')({
  validateSearch: searchSchema,
  component: Reports,
})
