import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { positiveIntParam, uuidParam } from '@/lib/search-params'
import { RoomManager } from '@/features/admin/rooms/room-manager'
import { ROOM_TYPES } from '@/features/rooms/labels'

const searchSchema = z.object({
  page: positiveIntParam,
  q: z.string().optional().catch(undefined),
  type: z.enum(ROOM_TYPES).optional().catch(undefined),
  /** Room open in the side editor. */
  room: uuidParam,
})

export const Route = createFileRoute('/_authenticated/admin/rooms')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.manageRooms),
  component: RoomManager,
})
