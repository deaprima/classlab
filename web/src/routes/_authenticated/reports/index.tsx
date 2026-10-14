import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { positiveIntParam, uuidParam } from '@/lib/search-params'
import { PendingPage } from '@/components/layout/page'

const searchSchema = z.object({
  page: positiveIntParam,
  /** Open detail panel; notifications link here. */
  report: uuidParam,
})

export const Route = createFileRoute('/_authenticated/reports/')({
  validateSearch: searchSchema,
  component: () => <PendingPage title='Laporan Kerusakan' />,
})
