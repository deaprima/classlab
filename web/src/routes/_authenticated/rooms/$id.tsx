import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { dateParam } from '@/lib/search-params'
import { RoomTimeline } from '@/features/rooms/timeline'

const searchSchema = z.object({
  date: dateParam,
  view: z.enum(['day', 'week']).optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/rooms/$id')({
  validateSearch: searchSchema,
  component: RoomTimeline,
})
