import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { positiveIntParam } from '@/lib/search-params'
import { AdminRooms } from '@/features/admin/rooms'
import { ROOM_TYPES } from '@/features/rooms/labels'

const searchSchema = z.object({
  page: positiveIntParam,
  q: z.string().optional().catch(undefined),
  type: z.enum(ROOM_TYPES).optional().catch(undefined),
  status: z.enum(['active', 'inactive']).optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/admin/rooms')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.manageRooms),
  component: AdminRooms,
})
