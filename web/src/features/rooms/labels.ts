import { type RoomType } from '@/lib/api-types'

export const ROOM_TYPES = [
  'KELAS',
  'LAB',
  'AUDITORIUM',
  'RUANG_UJIAN',
  'RUANG_SIDANG',
] as const satisfies readonly RoomType[]

export const ROOM_TYPE_LABELS: Record<RoomType, string> = {
  KELAS: 'Ruang Kelas',
  LAB: 'Laboratorium',
  AUDITORIUM: 'Auditorium',
  RUANG_UJIAN: 'Ruang Ujian',
  RUANG_SIDANG: 'Ruang Sidang',
}
