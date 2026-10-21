import { z } from 'zod'
import {
  type Booking,
  type EditBookingRequest,
  type Room,
  type RoomTypePolicy,
} from '@/lib/api-types'
import { toApiDateTime, toWibDate, toWibTime } from '@/lib/datetime'
import { participantsRequired } from '@/features/bookings/request/policy'
import {
  NOTES_MAX,
  occurrenceIssues,
  PURPOSE_MAX,
} from '@/features/bookings/request/request-schema'

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/
const TIME_RE = /^([01]\d|2[0-3]):[0-5]\d$/
const PARTICIPANTS_MAX = 10000

const baseSchema = z.object({
  date: z.string().regex(DATE_RE, 'Pilih tanggal.'),
  start_time: z.string().regex(TIME_RE, 'Pilih jam mulai.'),
  end_time: z.string().regex(TIME_RE, 'Pilih jam selesai.'),
  purpose: z
    .string()
    .trim()
    .min(1, 'Nama kegiatan wajib diisi.')
    .max(PURPOSE_MAX, `Nama kegiatan maksimal ${PURPOSE_MAX} karakter.`),
  participants: z
    .string()
    .trim()
    .refine(
      (v) =>
        v === '' ||
        (/^\d+$/.test(v) && Number(v) >= 1 && Number(v) <= PARTICIPANTS_MAX),
      `Jumlah peserta harus bilangan bulat 1–${PARTICIPANTS_MAX}.`
    ),
  notes: z
    .string()
    .trim()
    .max(NOTES_MAX, `Catatan maksimal ${NOTES_MAX} karakter.`),
})

export type EditFormValues = z.input<typeof baseSchema>

/** The form starts from what the booking is now. */
export function editDefaults(booking: Booking): EditFormValues {
  return {
    date: toWibDate(booking.start_at),
    start_time: toWibTime(booking.start_at),
    end_time: toWibTime(booking.end_at),
    purpose: booking.purpose,
    participants: booking.participants ? String(booking.participants) : '',
    notes: booking.notes ?? '',
  }
}

/**
 * The same rules as a new request for the one date (slot step, policy limits, room hours, not in
 * the past) plus the headcount limits. The room cannot change, so it comes from the booking. The
 * server re-checks everything, including the cutoff before the start.
 */
export function createEditSchema(ctx: {
  room: Room
  policy?: RoomTypePolicy
  now?: () => Date
}) {
  const now = ctx.now ?? (() => new Date())
  return baseSchema.superRefine((v, ctx2) => {
    for (const issue of occurrenceIssues(v, {
      policy: ctx.policy,
      room: ctx.room,
      now: now(),
    })) {
      ctx2.addIssue({
        code: 'custom',
        path: [issue.field],
        message: issue.message,
      })
    }
    const participants = v.participants.trim()
    if (participantsRequired(ctx.room.type) && participants === '') {
      ctx2.addIssue({
        code: 'custom',
        path: ['participants'],
        message: 'Jumlah peserta wajib diisi.',
      })
    }
    if (participants !== '' && Number(participants) > ctx.room.capacity) {
      ctx2.addIssue({
        code: 'custom',
        path: ['participants'],
        message: `Melebihi kapasitas ruang (${ctx.room.capacity} orang).`,
      })
    }
  })
}

/**
 * Only what changed goes to the API (`PATCH` changes the fields it receives). Null when nothing
 * changed, so the dialog can say so instead of sending an empty request.
 */
export function toEditRequest(
  booking: Booking,
  v: EditFormValues
): EditBookingRequest | null {
  const before = editDefaults(booking)
  const body: EditBookingRequest = {}
  if (
    v.date !== before.date ||
    v.start_time !== before.start_time ||
    v.end_time !== before.end_time
  ) {
    body.start_at = toApiDateTime(v.date, v.start_time)
    body.end_at = toApiDateTime(v.date, v.end_time)
  }
  if (v.purpose.trim() !== before.purpose) body.purpose = v.purpose.trim()
  const participants = v.participants.trim()
  if (participants !== '' && participants !== before.participants) {
    body.participants = Number(participants)
  }
  // An empty string removes the note (the API's way of clearing it).
  if (v.notes.trim() !== before.notes) body.notes = v.notes.trim()
  return Object.keys(body).length > 0 ? body : null
}

/** API field -> form field, for 422 details. */
export const EDIT_FIELD_MAP = {
  start_at: 'start_time',
  end_at: 'end_time',
  purpose: 'purpose',
  participants: 'participants',
  notes: 'notes',
} as const
