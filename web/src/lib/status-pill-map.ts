import { type PillStatus } from '@/components/status-pill'
import {
  type AvailabilityStatus,
  type BookingStatus,
  type ReportStatus,
} from './api-types'

// API status -> Figma pill style. Statuses the design has no pill for reuse the closest existing
// style instead of adding colours (docs/tasks.md, C5); the visible text stays the API label.

export const AVAILABILITY_PILL: Record<AvailabilityStatus, PillStatus> = {
  TERSEDIA: 'Tersedia',
  DIAJUKAN: 'Diajukan',
  DIGUNAKAN: 'Digunakan',
}

export const BOOKING_PILL: Record<BookingStatus, PillStatus> = {
  MENUNGGU: 'Menunggu',
  DISETUJUI: 'Disetujui',
  DITOLAK: 'Ditolak',
  DIBATALKAN: 'Terbatas',
  BERLANGSUNG: 'Digunakan',
  SELESAI: 'Selesai',
  TIDAK_HADIR: 'Ditolak',
  TANPA_CHECKOUT: 'Ditolak',
  KEDALUWARSA: 'Terbatas',
}

export const REPORT_PILL: Record<ReportStatus, PillStatus> = {
  BARU: 'Diajukan',
  DITERIMA: 'Diterima',
  DIPROSES: 'Diproses',
  SELESAI: 'Selesai',
}
