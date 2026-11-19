import {
  type Room,
  type RoomType,
  type RoomTypePolicy,
  type UserRole,
} from '@/lib/api-types'
import { ROOM_TYPE_CONFIG } from '@/features/room-types/config'

/** What the segmented control on the request page offers: Kelas, Lab, and everything else. */
export type TypeGroup = 'KELAS' | 'LAB' | 'LAINNYA'

export const TYPE_GROUP_LABELS: Record<TypeGroup, string> = {
  KELAS: 'Kelas',
  LAB: 'Lab',
  LAINNYA: 'Lainnya',
}

export function groupOf(type: RoomType): TypeGroup {
  return type === 'KELAS' || type === 'LAB' ? type : 'LAINNYA'
}

/** Policies of the room types `role` may request, in the order of the room type config. */
export function requestablePolicies(
  policies: readonly RoomTypePolicy[],
  role: UserRole
): RoomTypePolicy[] {
  const order = ROOM_TYPE_CONFIG.map((c) => c.type as RoomType)
  return policies
    .filter((p) => p.requester_roles.includes(role))
    .sort((a, b) => order.indexOf(a.type) - order.indexOf(b.type))
}

/** The groups that have at least one requestable type, in display order. */
export function availableGroups(
  policies: readonly RoomTypePolicy[]
): TypeGroup[] {
  const groups = new Set(policies.map((p) => groupOf(p.type)))
  return (['KELAS', 'LAB', 'LAINNYA'] as const).filter((g) => groups.has(g))
}

/** Hours a room can be booked: its own, else the default of its type (BR-29). */
export function effectiveHours(
  room: Pick<Room, 'open_time' | 'close_time'>,
  policy: Pick<RoomTypePolicy, 'open_time' | 'close_time'>
): { open: string; close: string } {
  return {
    open: room.open_time ?? policy.open_time,
    close: room.close_time ?? policy.close_time,
  }
}

/** A letter is needed when the room type asks for one, and always for an organisation (BR-18). */
export function letterRequired(
  policy: Pick<RoomTypePolicy, 'letter_required'>,
  role: UserRole
): boolean {
  return policy.letter_required || role === 'ORGANISASI'
}

/** Only an auditorium needs the headcount (BR-21); elsewhere it is optional. */
export function participantsRequired(type: RoomType): boolean {
  return type === 'AUDITORIUM'
}

type OutcomeInput = {
  policy: RoomTypePolicy
  role: UserRole
  /** Whether booking is blocked right now (violation points reached the limit). */
  blocked: boolean
  /** Result of the live availability check; null while unknown. */
  slotFree: boolean | null
  /** Who decides, for the wording: "TU", "Pengurus Lab", "TU / Admin". */
  approverLabel: string
}

export type Outcome = {
  kind: 'auto' | 'manual' | 'blocked' | 'conflict' | 'unknown'
  title: string
  description: string
}

const APPROVER_NAMES: Partial<Record<UserRole, string>> = {
  TU: 'TU',
  KALAB: 'Pengurus Lab',
  ADMIN: 'Admin',
}

export function approverLabel(policy: Pick<RoomTypePolicy, 'approver_roles'>) {
  return policy.approver_roles.map((r) => APPROVER_NAMES[r] ?? r).join(' / ')
}

/**
 * The preview under "Ringkasan" (docs/DESIGN.md, Ajukan Ruang): will this go through on its own or
 * wait for a person? It mirrors BR-17 for display only; the server decides on submit.
 */
export function previewOutcome(input: OutcomeInput): Outcome {
  const { policy, role, blocked, slotFree, approverLabel: who } = input
  if (blocked) {
    return {
      kind: 'blocked',
      title: 'Pengajuan diblokir',
      description: 'Poin pelanggaran aktif mencapai batas.',
    }
  }
  if (slotFree === false) {
    return {
      kind: 'conflict',
      title: 'Slot tidak kosong',
      description: 'Pilih jam atau ruang lain.',
    }
  }
  if (slotFree === null) {
    return {
      kind: 'unknown',
      title: 'Memeriksa ketersediaan',
      description: 'Pilih ruang, tanggal, dan jam.',
    }
  }
  const auto =
    policy.approval_policy === 'AUTO_IF_ELIGIBLE' && role === 'MAHASISWA'
  if (auto) {
    return {
      kind: 'auto',
      title: 'Akan disetujui otomatis',
      description: 'Ruang kosong dan riwayatmu baik.',
    }
  }
  return {
    kind: 'manual',
    title: `Menunggu validasi ${who}`,
    description:
      role === 'ORGANISASI'
        ? 'Pengajuan organisasi divalidasi manual. Kedaluwarsa dalam 24 jam bila tidak diputuskan.'
        : 'Kedaluwarsa dalam 24 jam bila tidak diputuskan.',
  }
}
