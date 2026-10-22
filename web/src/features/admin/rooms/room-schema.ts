import { z } from 'zod'
import { minutesOfDay } from '@/lib/datetime'
import { ROOM_TYPES } from '@/features/rooms/labels'

// Mirrors the API's room rules (decisions.md, Phase 2); the server re-checks them.
const MAX_FACILITIES = 30
const MAX_FACILITY_LENGTH = 60

const TIME_RE = /^([01][0-9]|2[0-3]):(00|30)$/

export const roomFormSchema = z
  .object({
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
    /** HH:MM in 30-minute steps; empty or missing follows the room type's default (BR-29). */
    open_time: z.string().optional(),
    close_time: z.string().optional(),
  })
  .superRefine((v, ctx) => {
    const open = v.open_time ?? ''
    const close = v.close_time ?? ''
    if (!open && !close) return
    if (!open || !close) {
      ctx.addIssue({
        code: 'custom',
        path: [open ? 'close_time' : 'open_time'],
        message:
          'Isi jam buka dan jam tutup bersamaan, atau kosongkan keduanya.',
      })
      return
    }
    if (!TIME_RE.test(open) || !TIME_RE.test(close)) {
      ctx.addIssue({
        code: 'custom',
        path: ['open_time'],
        message: 'Jam harus kelipatan 30 menit.',
      })
    } else if (minutesOfDay(close) <= minutesOfDay(open)) {
      ctx.addIssue({
        code: 'custom',
        path: ['close_time'],
        message: 'Jam tutup harus setelah jam buka.',
      })
    }
  })

export type RoomFormValues = z.input<typeof roomFormSchema>

/** "Proyektor, AC\nWhiteboard" -> ["Proyektor", "AC", "Whiteboard"], blanks dropped. */
export function parseFacilities(text: string): string[] {
  return text
    .split(/[,\n]/)
    .map((f) => f.trim())
    .filter(Boolean)
}

/** Every half hour of the day, for the opening-hours selects. */
export const HOUR_OPTIONS = Array.from({ length: 48 }, (_, i) => {
  const h = String(Math.floor(i / 2)).padStart(2, '0')
  return `${h}:${i % 2 ? '30' : '00'}`
})
