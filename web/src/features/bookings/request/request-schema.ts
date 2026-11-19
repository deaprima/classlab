import { z } from 'zod'
import {
  type CreateBookingRequest,
  type Room,
  type RoomTypePolicy,
  type UserRole,
} from '@/lib/api-types'
import {
  addDays,
  formatDuration,
  minutesOfDay,
  toApiDateTime,
  toWibDate,
} from '@/lib/datetime'
import { effectiveHours, letterRequired, participantsRequired } from './policy'

// Limits the API enforces beyond the policy (decisions.md, Phase 3 and 5).
export const PURPOSE_MAX = 200
export const NOTES_MAX = 1000
export const LETTER_NUMBER_MAX = 100
/** MAX_OCCURRENCES of the API (BR-27); the server stays the authority. */
export const MAX_OCCURRENCES = 14
/** Booking slots are fixed by BR-01. */
export const SLOT_MINUTES = 30
const PARTICIPANTS_MAX = 10000

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/
const TIME_RE = /^([01]\d|2[0-3]):[0-5]\d$/

const occurrenceSchema = z.object({
  date: z.string().regex(DATE_RE, 'Pilih tanggal.'),
  start_time: z.string().regex(TIME_RE, 'Pilih jam mulai.'),
  end_time: z.string().regex(TIME_RE, 'Pilih jam selesai.'),
})

export type OccurrenceValues = z.input<typeof occurrenceSchema>

const baseSchema = z.object({
  room_id: z.string().min(1, 'Pilih ruang.'),
  occurrences: z
    .array(occurrenceSchema)
    .min(1, 'Tambahkan minimal satu tanggal.')
    .max(MAX_OCCURRENCES, `Maksimal ${MAX_OCCURRENCES} tanggal sekali ajuan.`),
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
  letter_attachment_id: z.string(),
  letter_number: z
    .string()
    .trim()
    .max(
      LETTER_NUMBER_MAX,
      `Nomor surat maksimal ${LETTER_NUMBER_MAX} karakter.`
    ),
})

export type RequestFormValues = z.input<typeof baseSchema>

export type RequestContext = {
  role: UserRole
  /** Policy of the selected room's type; undefined until a room is chosen. */
  policy?: RoomTypePolicy
  room?: Room
  /** Injectable so tests are deterministic. */
  now?: () => Date
}

export function emptyOccurrence(): OccurrenceValues {
  return { date: '', start_time: '', end_time: '' }
}

export const EMPTY_REQUEST: RequestFormValues = {
  room_id: '',
  occurrences: [emptyOccurrence()],
  purpose: '',
  participants: '',
  notes: '',
  letter_attachment_id: '',
  letter_number: '',
}

/** Last date a request may be for under the policy, YYYY-MM-DD. */
export function lastBookableDate(
  policy: Pick<RoomTypePolicy, 'max_advance_days'>,
  now: Date
): string {
  return addDays(toWibDate(now), policy.max_advance_days)
}

type OccurrenceIssue = {
  field: 'date' | 'start_time' | 'end_time'
  message: string
}

/**
 * What is wrong with one date of a request, or of an edit: slot step, order, the duration limits of
 * the policy, the hours of the room, not in the past, and the advance limit. Empty when it is fine.
 */
export function occurrenceIssues(
  o: OccurrenceValues,
  ctx: { policy?: RoomTypePolicy; room?: Room; now: Date }
): OccurrenceIssue[] {
  if (
    !DATE_RE.test(o.date) ||
    !TIME_RE.test(o.start_time) ||
    !TIME_RE.test(o.end_time)
  ) {
    return []
  }
  const { policy, room, now } = ctx
  const issues: OccurrenceIssue[] = []
  const at = (field: OccurrenceIssue['field'], message: string) =>
    issues.push({ field, message })
  const start = minutesOfDay(o.start_time)
  const end = minutesOfDay(o.end_time)

  if (start % SLOT_MINUTES !== 0) {
    at('start_time', `Jam mulai harus kelipatan ${SLOT_MINUTES} menit.`)
  }
  if (end % SLOT_MINUTES !== 0) {
    at('end_time', `Jam selesai harus kelipatan ${SLOT_MINUTES} menit.`)
  }
  if (end <= start) {
    at('end_time', 'Jam selesai harus setelah jam mulai.')
  } else if (policy) {
    if (end - start < policy.min_duration_min) {
      at(
        'end_time',
        `Durasi minimal ${formatDuration(policy.min_duration_min)}.`
      )
    } else if (end - start > policy.max_duration_min) {
      at(
        'end_time',
        `Durasi maksimal ${formatDuration(policy.max_duration_min)}.`
      )
    }
  }
  if (policy && room) {
    const hours = effectiveHours(room, policy)
    if (start < minutesOfDay(hours.open)) {
      at('start_time', `Ruang buka mulai pukul ${hours.open} WIB.`)
    }
    if (end > minutesOfDay(hours.close)) {
      at('end_time', `Ruang tutup pukul ${hours.close} WIB.`)
    }
  }
  if (o.date < toWibDate(now)) {
    at('date', 'Tanggal sudah lewat.')
  } else if (Date.parse(toApiDateTime(o.date, o.start_time)) < now.getTime()) {
    at('start_time', 'Jam mulai sudah lewat.')
  }
  if (policy && o.date > lastBookableDate(policy, now)) {
    at(
      'date',
      `Bisa diajukan maksimal ${policy.max_advance_days} hari ke depan.`
    )
  }
  return issues
}

type Interval = { index: number; start: number; end: number }

/**
 * The request form schema: BR-01 and the policy of the room's type (slot step, duration, hours,
 * advance limit), the letter and headcount rules (BR-18, BR-21), and that the dates of one request
 * do not overlap (BR-27). The server re-checks everything and stays the authority.
 */
export function createRequestSchema(ctx: RequestContext) {
  const { role, policy, room } = ctx
  const now = ctx.now ?? (() => new Date())

  return baseSchema.superRefine((v, ctx2) => {
    const add = (path: (string | number)[], message: string) =>
      ctx2.addIssue({ code: 'custom', path, message })

    const current = now()
    const byDate = new Map<string, Interval[]>()

    v.occurrences.forEach((o, i) => {
      if (
        !DATE_RE.test(o.date) ||
        !TIME_RE.test(o.start_time) ||
        !TIME_RE.test(o.end_time)
      ) {
        return
      }
      const start = minutesOfDay(o.start_time)
      const end = minutesOfDay(o.end_time)
      for (const issue of occurrenceIssues(o, { policy, room, now: current })) {
        add(['occurrences', i, issue.field], issue.message)
      }

      if (end > start) {
        const list = byDate.get(o.date) ?? []
        list.push({ index: i, start, end })
        byDate.set(o.date, list)
      }
    })

    for (const list of byDate.values()) {
      list.sort((a, b) => a.start - b.start)
      for (let k = 1; k < list.length; k++) {
        if (list[k].start < list[k - 1].end) {
          add(
            ['occurrences', list[k].index, 'start_time'],
            'Bertumpuk dengan tanggal lain di pengajuan ini.'
          )
        }
      }
    }

    const participants = v.participants.trim()
    if (room && policy) {
      if (participantsRequired(room.type) && participants === '') {
        add(['participants'], 'Jumlah peserta wajib diisi.')
      }
      if (participants !== '' && Number(participants) > room.capacity) {
        add(
          ['participants'],
          `Melebihi kapasitas ruang (${room.capacity} orang).`
        )
      }
    }
    if (
      policy &&
      letterRequired(policy, role) &&
      v.letter_attachment_id === ''
    ) {
      add(['letter_attachment_id'], 'Surat wajib dilampirkan.')
    }
  })
}

/** One date is sent as start_at/end_at, several as `occurrences` (BR-27); never both. */
export function toCreateBookingRequest(
  v: RequestFormValues
): CreateBookingRequest {
  const participants = v.participants.trim()
  const notes = v.notes.trim()
  const letterNumber = v.letter_number.trim()
  const times = v.occurrences.map((o) => ({
    start_at: toApiDateTime(o.date, o.start_time),
    end_at: toApiDateTime(o.date, o.end_time),
  }))
  return {
    room_id: v.room_id,
    ...(times.length === 1 ? times[0] : { occurrences: times }),
    purpose: v.purpose.trim(),
    ...(participants && { participants: Number(participants) }),
    ...(notes && { notes }),
    ...(v.letter_attachment_id && {
      letter_attachment_id: v.letter_attachment_id,
    }),
    ...(letterNumber && { letter_number: letterNumber }),
  }
}

/** API field -> form field, for 422 details. */
export const REQUEST_FIELD_MAP = {
  room_id: 'room_id',
  purpose: 'purpose',
  participants: 'participants',
  notes: 'notes',
  letter_attachment_id: 'letter_attachment_id',
  letter_number: 'letter_number',
} as const
