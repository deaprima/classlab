import { type AvailabilityStatus, type BookingStatus } from './api-types'

export const AVAILABILITY_LABELS: Record<AvailabilityStatus, string> = {
  TERSEDIA: 'Tersedia',
  DIAJUKAN: 'Diajukan',
  DIGUNAKAN: 'Digunakan',
}

export const BOOKING_STATUS_LABELS: Record<BookingStatus, string> = {
  MENUNGGU: 'Menunggu',
  DISETUJUI: 'Disetujui',
  DITOLAK: 'Ditolak',
  DIBATALKAN: 'Dibatalkan',
}
