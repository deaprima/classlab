import { type Violation } from '@/lib/api-types'

export const VIOLATION_LABELS: Record<Violation['kind'], string> = {
  TIDAK_HADIR: 'Tidak hadir',
  TANPA_CHECKOUT: 'Tidak check-out',
  BATAL_TERLAMBAT: 'Pembatalan terlambat',
}
