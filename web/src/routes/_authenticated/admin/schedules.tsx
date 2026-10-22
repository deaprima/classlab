import { createFileRoute, redirect } from '@tanstack/react-router'

// The schedule list now lives in the "Jadwal" tab of Ruang & Jadwal; old links land there.
export const Route = createFileRoute('/_authenticated/admin/schedules')({
  beforeLoad: () => {
    throw redirect({ to: '/admin/rooms', search: { tab: 'jadwal' } })
  },
})
