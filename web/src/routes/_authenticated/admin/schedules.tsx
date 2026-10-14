import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { dateParam, uuidParam } from '@/lib/search-params'
import { AdminSchedules } from '@/features/admin/schedules'

const searchSchema = z.object({
  room_id: uuidParam,
  /** Any day of the week to show. */
  date: dateParam,
})

export const Route = createFileRoute('/_authenticated/admin/schedules')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.manageSchedules),
  component: AdminSchedules,
})
