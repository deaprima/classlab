import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { dateParam, positiveIntParam, uuidParam } from '@/lib/search-params'
import { BookingsEntry } from '@/features/my-bookings/entry'

const searchSchema = z.object({
  page: positiveIntParam,
  status: z
    .enum([
      'MENUNGGU',
      'DISETUJUI',
      'DITOLAK',
      'DIBATALKAN',
      'BERLANGSUNG',
      'SELESAI',
      'TIDAK_HADIR',
      'TANPA_CHECKOUT',
      'KEDALUWARSA',
    ])
    .optional()
    .catch(undefined),
  room_id: uuidParam,
  type: z
    .enum(['KELAS', 'LAB', 'AUDITORIUM', 'RUANG_UJIAN', 'RUANG_SIDANG'])
    .optional()
    .catch(undefined),
  /** Free-text filter of Pengajuan Saya. */
  q: z.string().optional().catch(undefined),
  date: dateParam,
  /** Open detail panel; notifications link here. */
  booking: uuidParam,
})

export const Route = createFileRoute('/_authenticated/bookings/')({
  validateSearch: searchSchema,
  beforeLoad: () => requireRole(ACCESS.viewBookings),
  component: BookingsEntry,
})
