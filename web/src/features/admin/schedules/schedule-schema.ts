import { z } from 'zod'
import { minutesOfDay } from '@/lib/datetime'

// BR-09 and the API's schedule rules (decisions.md, Phase 2): one WIB day per session, weekly
// repeats up to 26 sessions with `until` inclusive.
export const MAX_SESSIONS = 26
const TITLE_MAX = 200
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/
const TIME_RE = /^([01]\d|2[0-3]):[0-5]\d$/

/** Weekly sessions from `date` through `until` inclusive. */
export function sessionCount(date: string, until: string): number {
  if (!DATE_RE.test(date) || !DATE_RE.test(until) || until < date) return 0
  const days =
    (Date.parse(`${until}T00:00:00Z`) - Date.parse(`${date}T00:00:00Z`)) /
    86_400_000
  return Math.floor(days / 7) + 1
}

export const scheduleFormSchema = z
  .object({
    room_id: z.string().min(1, 'Pilih ruang.'),
    title: z
      .string()
      .trim()
      .min(1, 'Judul wajib diisi.')
      .max(TITLE_MAX, `Judul maksimal ${TITLE_MAX} karakter.`),
    date: z.string().regex(DATE_RE, 'Pilih tanggal.'),
    start_time: z.string().regex(TIME_RE, 'Isi jam mulai.'),
    end_time: z.string().regex(TIME_RE, 'Isi jam selesai.'),
    source: z.enum(['AKADEMIK', 'MANUAL']),
    repeat: z.boolean(),
    until: z.string(),
  })
  .superRefine((v, ctx) => {
    if (
      TIME_RE.test(v.start_time) &&
      TIME_RE.test(v.end_time) &&
      minutesOfDay(v.end_time) <= minutesOfDay(v.start_time)
    ) {
      ctx.addIssue({
        code: 'custom',
        path: ['end_time'],
        message: 'Jam selesai harus setelah jam mulai (dalam hari yang sama).',
      })
    }
    if (!v.repeat) return
    if (!DATE_RE.test(v.until)) {
      ctx.addIssue({
        code: 'custom',
        path: ['until'],
        message: 'Isi tanggal akhir pengulangan.',
      })
    } else if (v.until < v.date) {
      ctx.addIssue({
        code: 'custom',
        path: ['until'],
        message: 'Tanggal akhir tidak boleh sebelum tanggal mulai.',
      })
    } else if (sessionCount(v.date, v.until) > MAX_SESSIONS) {
      ctx.addIssue({
        code: 'custom',
        path: ['until'],
        message: `Maksimal ${MAX_SESSIONS} sesi mingguan.`,
      })
    }
  })

export type ScheduleFormValues = z.input<typeof scheduleFormSchema>

/** API field -> form field for 422 details. */
export const SCHEDULE_FIELD_MAP = {
  room_id: 'room_id',
  title: 'title',
  start_at: 'start_time',
  end_at: 'end_time',
  source: 'source',
  'repeat.until': 'until',
} as const
