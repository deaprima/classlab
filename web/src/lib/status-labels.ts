import {
  type AvailabilityStatus,
  type BookingStatus,
  type ReportStatus,
} from './api-types'

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
  BERLANGSUNG: 'Berlangsung',
  SELESAI: 'Selesai',
  TIDAK_HADIR: 'Tidak hadir',
  TANPA_CHECKOUT: 'Tanpa check-out',
  KEDALUWARSA: 'Kedaluwarsa',
}

export const REPORT_STATUS_LABELS: Record<ReportStatus, string> = {
  BARU: 'Baru',
  DITERIMA: 'Diterima',
  DIPROSES: 'Diproses',
  SELESAI: 'Selesai',
}
