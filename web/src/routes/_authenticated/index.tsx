import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { dateParam, positiveIntParam, timeParam } from '@/lib/search-params'
import { Availability } from '@/features/availability'
import { ROOM_TYPES } from '@/features/rooms/labels'

const searchSchema = z.object({
  date: dateParam,
  start: timeParam,
  end: timeParam,
  type: z.enum(ROOM_TYPES).optional().catch(undefined),
  capacity_min: positiveIntParam,
})

export const Route = createFileRoute('/_authenticated/')({
  validateSearch: searchSchema,
  component: Availability,
})
