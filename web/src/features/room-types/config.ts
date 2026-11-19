import {
  type LucideIcon,
  Building2,
  ClipboardList,
  DoorOpen,
  FlaskConical,
  GraduationCap,
} from 'lucide-react'
import { type RoomType } from '@/lib/api-types'

/**
 * One entry per room type: the URL segment of its page group (`/kelas/...`, brief 7.2), its
 * label and icon. Behaviour that differs per type comes from the server policy
 * (`useRoomTypes`), never from branching on the type in components.
 */
type RoomTypeConfig = {
  type: RoomType
  slug: string
  /** Fallback label; the server policy carries the authoritative one. */
  label: string
  icon: LucideIcon
}

export const ROOM_TYPE_CONFIG = [
  { type: 'KELAS', slug: 'kelas', label: 'Ruang Kelas', icon: DoorOpen },
  { type: 'LAB', slug: 'lab', label: 'Laboratorium', icon: FlaskConical },
  {
    type: 'AUDITORIUM',
    slug: 'auditorium',
    label: 'Auditorium',
    icon: Building2,
  },
  {
    type: 'RUANG_UJIAN',
    slug: 'ujian',
    label: 'Ruang Ujian',
    icon: ClipboardList,
  },
  {
    type: 'RUANG_SIDANG',
    slug: 'sidang',
    label: 'Ruang Sidang',
    icon: GraduationCap,
  },
] as const satisfies readonly RoomTypeConfig[]

export function roomTypeConfig(type: RoomType): RoomTypeConfig {
  const config = ROOM_TYPE_CONFIG.find((c) => c.type === type)
  if (!config) throw new Error(`Unknown room type: ${type}`)
  return config
}

export function roomTypeFromSlug(slug: string): RoomType | undefined {
  return ROOM_TYPE_CONFIG.find((c) => c.slug === slug)?.type
}
