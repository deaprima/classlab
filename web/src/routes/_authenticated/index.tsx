import { createFileRoute } from '@tanstack/react-router'
import { PendingPage } from '@/components/layout/page'

export const Route = createFileRoute('/_authenticated/')({
  component: () => <PendingPage title='Ketersediaan Ruang' />,
})
