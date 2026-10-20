import { z } from 'zod'
import { ROOM_TYPES } from '@/features/rooms/labels'

// Mirrors the API's room rules (decisions.md, Phase 2); the server re-checks them.
const MAX_FACILITIES = 30
const MAX_FACILITY_LENGTH = 60

export const roomFormSchema = z.object({
  code: z
    .string()
    .trim()
    .regex(
      /^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$/,
      'Kode 1-32 karakter (huruf, angka, titik, garis), diawali huruf atau angka.'
    ),
  name: z
    .string()
    .trim()
    .min(1, 'Nama wajib diisi.')
    .max(120, 'Nama maksimal 120 karakter.'),
  type: z.enum(ROOM_TYPES, 'Pilih tipe ruang.'),
  capacity: z
    .string()
    .trim()
    .refine(
      (v) => /^\d+$/.test(v) && Number(v) >= 1 && Number(v) <= 10000,
      'Kapasitas harus bilangan bulat 1-10000.'
    ),
  /** One facility per line or comma-separated. */
  facilities: z
    .string()
    .refine(
      (v) => parseFacilities(v).length <= MAX_FACILITIES,
      `Maksimal ${MAX_FACILITIES} fasilitas.`
    )
    .refine(
      (v) => parseFacilities(v).every((f) => f.length <= MAX_FACILITY_LENGTH),
      `Setiap fasilitas maksimal ${MAX_FACILITY_LENGTH} karakter.`
    ),
  is_active: z.boolean(),
})

export type RoomFormValues = z.input<typeof roomFormSchema>

/** "Proyektor, AC\nWhiteboard" -> ["Proyektor", "AC", "Whiteboard"], blanks dropped. */
export function parseFacilities(text: string): string[] {
  return text
    .split(/[,\n]/)
    .map((f) => f.trim())
    .filter(Boolean)
}
