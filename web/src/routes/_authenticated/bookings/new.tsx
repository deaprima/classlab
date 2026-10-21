import { z } from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { dateParam, timeParam, uuidParam } from '@/lib/search-params'
import { groupOf } from '@/features/bookings/request/policy'
import { ROOM_TYPES } from '@/features/rooms/labels'

const searchSchema = z.object({
  type: z.enum(ROOM_TYPES).optional().catch(undefined),
  room_id: uuidParam,
  date: dateParam,
  start: timeParam,
  end: timeParam,
})

// Requests are made from the schedule now (a popup over it); this keeps old links working by
// opening the schedule on the same room, type and date.
export const Route = createFileRoute('/_authenticated/bookings/new')({
  validateSearch: searchSchema,
  beforeLoad: ({ search }) => {
    requireRole(ACCESS.createBooking)
    throw redirect({
      to: '/schedule',
      search: {
        date: search.date,
        start: search.start,
        end: search.end,
        room: search.room_id,
        group: search.type ? groupOf(search.type) : undefined,
      },
    })
  },
})
