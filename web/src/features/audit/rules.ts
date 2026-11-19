import { type AuditLog } from '@/lib/api-types'
import { addDays, toApiDateTime, todayWib } from '@/lib/datetime'

/** Indonesian names of the actions the API writes (api/internal/audit/actions.go); an unknown one shows as is. */
const ACTION_LABELS: Record<string, string> = {
  USER_CREATED: 'Buat pengguna',
  USER_UPDATED: 'Ubah pengguna',
  USER_ROLE_CHANGED: 'Ubah peran pengguna',
  USER_DEACTIVATED: 'Nonaktifkan pengguna',
  USER_ACTIVATED: 'Aktifkan pengguna',
  USER_BLOCK_CLEARED: 'Buka blokir',
  USER_PASSWORD_RESET: 'Reset password',
  USERS_IMPORTED: 'Impor akun mahasiswa',
  ROOM_CREATED: 'Tambah ruang',
  ROOM_UPDATED: 'Ubah ruang',
  ROOM_DELETED: 'Hapus ruang',
  ROOM_QR_ROTATED: 'Ganti token QR',
  SCHEDULE_CREATED: 'Tambah jadwal',
  SCHEDULE_UPDATED: 'Ubah jadwal',
  SCHEDULE_DELETED: 'Hapus jadwal',
  ROOM_TYPE_POLICY_UPDATED: 'Ubah kebijakan ruang',
  SCHEDULES_IMPORTED: 'Impor jadwal akademik',
  IMPORT_ROLLED_BACK: 'Rollback impor',
  VIOLATION_ANNULLED: 'Annul pelanggaran',
}

export function actionLabel(action: string): string {
  return ACTION_LABELS[action] ?? action
}

/** The entity types the API writes, as the filter chips group them. */
export const ENTITY_GROUPS = [
  { value: 'USER', label: 'Pengguna' },
  { value: 'ROOM', label: 'Ruang' },
  { value: 'SCHEDULE', label: 'Jadwal' },
  { value: 'IMPORT', label: 'Impor' },
  { value: 'ROOM_TYPE', label: 'Kebijakan' },
  { value: 'VIOLATION', label: 'Pelanggaran' },
] as const

const ENTITY_LABELS: Record<string, string> = {
  USER: 'Pengguna',
  ROOM: 'Ruang',
  SCHEDULE: 'Jadwal',
  IMPORT: 'Impor',
  ROOM_TYPE: 'Kebijakan ruang',
  VIOLATION: 'Pelanggaran',
}

export function entityLabel(type: string): string {
  return ENTITY_LABELS[type] ?? type
}

export const RANGES = ['day', 'week', 'month', 'all'] as const
export type RangeKey = (typeof RANGES)[number]

export const RANGE_LABELS: Record<RangeKey, string> = {
  day: '24 jam terakhir',
  week: '7 hari terakhir',
  month: '30 hari terakhir',
  all: 'Semua waktu',
}

/** The `from` of the chosen range (RFC 3339 with the WIB offset), or undefined for "all". */
export function rangeStart(key: RangeKey, now: Date): string | undefined {
  if (key === 'all') return undefined
  if (key === 'day')
    return new Date(now.getTime() - 24 * 3_600_000).toISOString()
  const days = key === 'week' ? 6 : 29
  return toApiDateTime(addDays(todayWib(now), -days), '00:00')
}

/** Who did it: name, else identifier, else "Sistem" for what the system itself did. */
export function actorLabel(
  log: Pick<AuditLog, 'actor_name' | 'actor_identifier'>
) {
  return log.actor_name ?? log.actor_identifier ?? 'Sistem'
}

/**
 * The before or after state as readable lines. The API sends free JSON (secrets already masked):
 * an object becomes "key: value" lines, anything else one line, null becomes no lines.
 */
export function stateLines(value: unknown): string[] {
  if (value === null || value === undefined) return []
  if (typeof value !== 'object') return [String(value)]
  if (Array.isArray(value)) return value.map((v) => formatValue(v))
  return Object.entries(value).map(([k, v]) => `${k}: ${formatValue(v)}`)
}

function formatValue(v: unknown): string {
  if (v === null) return '–'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}
