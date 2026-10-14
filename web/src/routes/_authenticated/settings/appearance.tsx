import { createFileRoute } from '@tanstack/react-router'
import { Appearance } from '@/features/appearance'

export const Route = createFileRoute('/_authenticated/settings/appearance')({
  component: Appearance,
})
