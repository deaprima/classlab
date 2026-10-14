import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { positiveIntParam } from '@/lib/search-params'
import { Notifications } from '@/features/notifications'

const searchSchema = z.object({
  page: positiveIntParam,
  filter: z.enum(['unread']).optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/notifications/')({
  validateSearch: searchSchema,
  component: Notifications,
})
