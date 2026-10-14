import { createFileRoute } from '@tanstack/react-router'
import { PendingPage } from '@/components/layout/page'

export const Route = createFileRoute('/_authenticated/rooms/$id')({
  component: () => <PendingPage title='Jadwal Ruang' />,
})
