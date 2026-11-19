import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { dateParam } from '@/lib/search-params'
import { RoomDetail } from '@/features/schedule/room-detail'

const searchSchema = z.object({
  date: dateParam,
})

export const Route = createFileRoute('/_authenticated/rooms/$id')({
  validateSearch: searchSchema,
  component: RoomDetail,
})
