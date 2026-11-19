import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { positiveIntParam, uuidParam } from '@/lib/search-params'
import { ROOM_TYPES } from '@/features/rooms/labels'
import { Verify } from '@/features/verify'
import { RANGES } from '@/features/verify/rules'

const searchSchema = z.object({
  page: positiveIntParam,
  type: z.enum(ROOM_TYPES).optional().catch(undefined),
  /** One room only. */
  room: uuidParam,
  range: z.enum(RANGES).optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/verify')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.verifyCheckouts),
  component: Verify,
})
