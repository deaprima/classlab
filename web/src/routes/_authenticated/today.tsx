import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { dateParam } from '@/lib/search-params'
import { RoomBoard } from '@/features/room-board'

const searchSchema = z.object({
  /** The day shown; today when missing. */
  date: dateParam,
  group: z.enum(['KELAS', 'LAB', 'LAINNYA']).optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/today')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.viewRoomBoard),
  component: RoomBoard,
})
