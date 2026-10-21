import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { dateParam, timeParam, uuidParam } from '@/lib/search-params'
import { NewBooking } from '@/features/bookings/new-booking'
import { ROOM_TYPES } from '@/features/rooms/labels'

const searchSchema = z.object({
  type: z.enum(ROOM_TYPES).optional().catch(undefined),
  room_id: uuidParam,
  date: dateParam,
  start: timeParam,
  end: timeParam,
})

export const Route = createFileRoute('/_authenticated/bookings/new')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.createBooking),
  component: NewBooking,
})
