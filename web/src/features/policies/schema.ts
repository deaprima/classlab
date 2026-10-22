import { z } from 'zod'
import { minutesOfDay } from '@/lib/datetime'

// The API's rules for a room type policy (api: PATCH /room-types/{type}); the server re-checks them.
export const APPROVER_ROLES = ['TU', 'KALAB', 'ADMIN'] as const
export const REQUESTER_ROLES = ['MAHASISWA', 'ORGANISASI'] as const

// Half-hour steps; the closing time may be midnight (24:00).
const OPEN_TIME = /^([01][0-9]|2[0-3]):(00|30)$/
const CLOSE_TIME = /^(([01][0-9]|2[0-3]):(00|30)|24:00)$/

export const HOURS = Array.from({ length: 48 }, (_, i) => {
  const h = String(Math.floor(i / 2)).padStart(2, '0')
  return `${h}:${i % 2 ? '30' : '00'}`
})
export const CLOSE_HOURS = [...HOURS.slice(1), '24:00']

/** 30 minutes to 12 hours in half-hour steps, the durations a booking may have. */
export const DURATIONS = Array.from({ length: 24 }, (_, i) => (i + 1) * 30)

export const policyFormSchema = z
  .object({
    approver_roles: z
      .array(z.enum(APPROVER_ROLES))
      .min(1, 'Pilih minimal satu approver.'),
    requester_roles: z
      .array(z.enum(REQUESTER_ROLES))
      .min(1, 'Pilih minimal satu peran pengaju.'),
    approval_policy: z.enum(['AUTO_IF_ELIGIBLE', 'ALWAYS_MANUAL']),
    letter_required: z.boolean(),
    open_time: z
      .string()
      .regex(OPEN_TIME, 'Jam buka harus kelipatan 30 menit.'),
    close_time: z
      .string()
      .regex(CLOSE_TIME, 'Jam tutup harus kelipatan 30 menit.'),
    min_duration_min: z.number(),
    max_duration_min: z.number(),
    max_advance_days: z
      .string()
      .trim()
      .refine(
        (v) => /^\d+$/.test(v) && Number(v) >= 1 && Number(v) <= 365,
        'Isi bilangan bulat 1-365.'
      ),
  })
  .superRefine((v, ctx) => {
    if (
      OPEN_TIME.test(v.open_time) &&
      CLOSE_TIME.test(v.close_time) &&
      minutesOfDay(v.close_time) <= minutesOfDay(v.open_time)
    ) {
      ctx.addIssue({
        code: 'custom',
        path: ['close_time'],
        message: 'Jam tutup harus setelah jam buka.',
      })
    }
    if (v.min_duration_min > v.max_duration_min) {
      ctx.addIssue({
        code: 'custom',
        path: ['min_duration_min'],
        message: 'Durasi minimal tidak boleh melebihi durasi maksimal.',
      })
    }
  })

export type PolicyFormValues = z.input<typeof policyFormSchema>

/** API field -> form field for 422 details. */
export const POLICY_FIELDS = {
  approver_roles: 'approver_roles',
  requester_roles: 'requester_roles',
  approval_policy: 'approval_policy',
  letter_required: 'letter_required',
  open_time: 'open_time',
  close_time: 'close_time',
  min_duration_min: 'min_duration_min',
  max_duration_min: 'max_duration_min',
  max_advance_days: 'max_advance_days',
} as const
