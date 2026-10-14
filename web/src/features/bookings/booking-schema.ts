import { z } from 'zod'
import { bookingRules } from '@/config/booking-rules'
import { type CreateBookingRequest } from '@/lib/api-types'
import { addDays, minutesOfDay, toApiDateTime, toWibDate } from '@/lib/datetime'

// Limits the API enforces beyond BR-01 (decisions.md, Phase 3).
export const PURPOSE_MAX = 200
export const NOTES_MAX = 1000
const PARTICIPANTS_MAX = 10000

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/
const TIME_RE = /^([01]\d|2[0-3]):[0-5]\d$/

function formatMinutes(total: number): string {
  const h = Math.floor(total / 60)
  const m = total % 60
  if (h === 0) return `${m} menit`
  return m === 0 ? `${h} jam` : `${h} jam ${m} menit`
}

/**
 * Booking form schema mirroring BR-01: same WIB day, 30-minute steps, duration limits, operating
 * hours, not in the past, at most N days ahead. `now` is injectable so tests are deterministic.
 * The server re-checks everything and stays authoritative.
 */
export function createBookingSchema(now: () => Date = () => new Date()) {
  const r = bookingRules
  return z
    .object({
      room_id: z.string().min(1, 'Pilih ruang.'),
      date: z.string().regex(DATE_RE, 'Pilih tanggal.'),
      start_time: z.string().regex(TIME_RE, 'Pilih jam mulai.'),
      end_time: z.string().regex(TIME_RE, 'Pilih jam selesai.'),
      purpose: z
        .string()
        .trim()
        .min(1, 'Keperluan wajib diisi.')
        .max(PURPOSE_MAX, `Keperluan maksimal ${PURPOSE_MAX} karakter.`),
      participants: z
        .string()
        .trim()
        .refine(
          (v) =>
            v === '' ||
            (/^\d+$/.test(v) &&
              Number(v) >= 1 &&
              Number(v) <= PARTICIPANTS_MAX),
          `Jumlah peserta harus bilangan bulat 1–${PARTICIPANTS_MAX}.`
        ),
      notes: z
        .string()
        .trim()
        .max(NOTES_MAX, `Catatan maksimal ${NOTES_MAX} karakter.`),
    })
    .superRefine((v, ctx) => {
      if (
        !DATE_RE.test(v.date) ||
        !TIME_RE.test(v.start_time) ||
        !TIME_RE.test(v.end_time)
      ) {
        return
      }
      const start = minutesOfDay(v.start_time)
      const end = minutesOfDay(v.end_time)
      const issue = (
        path: 'start_time' | 'end_time' | 'date',
        message: string
      ) => ctx.addIssue({ code: 'custom', path: [path], message })

      if (start % r.slotMinutes !== 0) {
        issue('start_time', `Jam mulai harus kelipatan ${r.slotMinutes} menit.`)
      }
      if (end % r.slotMinutes !== 0) {
        issue('end_time', `Jam selesai harus kelipatan ${r.slotMinutes} menit.`)
      }
      if (end <= start) {
        issue('end_time', 'Jam selesai harus setelah jam mulai.')
      } else if (end - start < r.minDurationMinutes) {
        issue(
          'end_time',
          `Durasi minimal ${formatMinutes(r.minDurationMinutes)}.`
        )
      } else if (end - start > r.maxDurationMinutes) {
        issue(
          'end_time',
          `Durasi maksimal ${formatMinutes(r.maxDurationMinutes)}.`
        )
      }
      if (start < minutesOfDay(r.openTime)) {
        issue('start_time', `Jam operasional dimulai pukul ${r.openTime} WIB.`)
      }
      if (end > minutesOfDay(r.closeTime)) {
        issue('end_time', `Jam operasional berakhir pukul ${r.closeTime} WIB.`)
      }

      const current = now()
      const today = toWibDate(current)
      if (v.date < today) {
        issue('date', 'Tanggal sudah lewat.')
      } else if (
        Date.parse(toApiDateTime(v.date, v.start_time)) < current.getTime()
      ) {
        issue('start_time', 'Jam mulai sudah lewat.')
      }
      if (v.date > addDays(today, r.maxFutureDays)) {
        issue('date', `Peminjaman maksimal ${r.maxFutureDays} hari ke depan.`)
      }
    })
}

export type BookingFormValues = z.input<ReturnType<typeof createBookingSchema>>

export function toCreateBookingRequest(
  v: BookingFormValues
): CreateBookingRequest {
  const participants = v.participants.trim()
  const notes = v.notes.trim()
  return {
    room_id: v.room_id,
    start_at: toApiDateTime(v.date, v.start_time),
    end_at: toApiDateTime(v.date, v.end_time),
    purpose: v.purpose.trim(),
    ...(participants && { participants: Number(participants) }),
    ...(notes && { notes }),
  }
}

/** API field -> form field, for 422 details. */
export const BOOKING_FIELD_MAP = {
  room_id: 'room_id',
  start_at: 'start_time',
  end_at: 'end_time',
  purpose: 'purpose',
  participants: 'participants',
  notes: 'notes',
} as const
