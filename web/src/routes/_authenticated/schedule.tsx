import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { dateParam, timeParam, uuidParam } from '@/lib/search-params'
import { Schedule } from '@/features/schedule'

const searchSchema = z.object({
  date: dateParam,
  start: timeParam,
  end: timeParam,
  group: z.enum(['KELAS', 'LAB', 'LAINNYA']).optional().catch(undefined),
  room: uuidParam,
})

export const Route = createFileRoute('/_authenticated/schedule')({
  validateSearch: searchSchema,
  component: Schedule,
})
