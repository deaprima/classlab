import { z } from 'zod'
import { type ReportStatus } from '@/lib/api-types'

// API limits (brief.md 8, decisions.md Phase 4). The server checks the real file type by its bytes;
// these checks only spare the user a slow upload that would be refused.
export const DESCRIPTION_MAX = 2000
export const NOTE_MAX = 500
const PHOTO_MAX_BYTES = 5 * 1024 * 1024
export const PHOTO_TYPES = ['image/jpeg', 'image/png', 'image/webp']

const FLOW: ReportStatus[] = ['BARU', 'DITERIMA', 'DIPROSES', 'SELESAI']

/** BR-08: a report only moves one step forward. Null when it is finished. */
export function nextReportStatus(status: ReportStatus): ReportStatus | null {
  const i = FLOW.indexOf(status)
  return i >= 0 && i < FLOW.length - 1 ? FLOW[i + 1] : null
}

export const reportFormSchema = z.object({
  room_id: z.string().min(1, 'Pilih ruang.'),
  description: z
    .string()
    .trim()
    .min(1, 'Jelaskan kerusakannya.')
    .max(DESCRIPTION_MAX, `Deskripsi maksimal ${DESCRIPTION_MAX} karakter.`),
  photo: z
    .instanceof(File)
    .optional()
    .refine(
      (f) => !f || PHOTO_TYPES.includes(f.type),
      'Foto harus berformat JPG, PNG, atau WebP.'
    )
    .refine(
      (f) => !f || f.size <= PHOTO_MAX_BYTES,
      'Ukuran foto maksimal 5 MB.'
    ),
})

export type ReportFormValues = z.input<typeof reportFormSchema>
