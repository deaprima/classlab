import { describe, expect, it } from 'vitest'
import {
  type BookingFormValues,
  createBookingSchema,
  toCreateBookingRequest,
} from './booking-schema'

// Tuesday 20 Oct 2026, 09:10 WIB.
const NOW = new Date('2026-10-20T09:10:00+07:00')
const schema = createBookingSchema(() => NOW)

const valid: BookingFormValues = {
  room_id: '0b6a3a8e-1d2c-4c7e-9f0a-1234567890ab',
  date: '2026-10-21',
  start_time: '08:00',
  end_time: '10:00',
  purpose: 'Latihan kelompok ManPro',
  participants: '',
  notes: '',
}

function errorsFor(patch: Partial<BookingFormValues>) {
  const result = schema.safeParse({ ...valid, ...patch })
  if (result.success) return {}
  const out: Record<string, string[]> = {}
  for (const issue of result.error.issues) {
    const key = String(issue.path[0])
    ;(out[key] ??= []).push(issue.message)
  }
  return out
}

describe('booking schema (BR-01)', () => {
  it('accepts a valid request', () => {
    expect(schema.safeParse(valid).success).toBe(true)
  })

  it('requires the end after the start', () => {
    expect(
      errorsFor({ start_time: '10:00', end_time: '10:00' }).end_time
    ).toEqual(['Jam selesai harus setelah jam mulai.'])
  })

  it('requires 30-minute steps', () => {
    expect(errorsFor({ start_time: '08:15' }).start_time).toContain(
      'Jam mulai harus kelipatan 30 menit.'
    )
  })

  it('enforces the maximum duration', () => {
    expect(
      errorsFor({ start_time: '07:00', end_time: '15:30' }).end_time
    ).toEqual(['Durasi maksimal 8 jam.'])
    expect(errorsFor({ start_time: '07:00', end_time: '15:00' })).toEqual({})
  })

  it('enforces operating hours', () => {
    expect(errorsFor({ start_time: '06:30' }).start_time).toEqual([
      'Jam operasional dimulai pukul 07:00 WIB.',
    ])
    expect(
      errorsFor({ start_time: '20:00', end_time: '21:30' }).end_time
    ).toEqual(['Jam operasional berakhir pukul 21:00 WIB.'])
    expect(errorsFor({ start_time: '19:00', end_time: '21:00' })).toEqual({})
  })

  it('rejects the past, today included', () => {
    expect(errorsFor({ date: '2026-10-19' }).date).toEqual([
      'Tanggal sudah lewat.',
    ])
    expect(
      errorsFor({ date: '2026-10-20', start_time: '09:00', end_time: '10:00' })
        .start_time
    ).toEqual(['Jam mulai sudah lewat.'])
    expect(
      errorsFor({ date: '2026-10-20', start_time: '09:30', end_time: '10:00' })
    ).toEqual({})
  })

  it('allows booking at most 30 days ahead', () => {
    expect(errorsFor({ date: '2026-11-19' })).toEqual({})
    expect(errorsFor({ date: '2026-11-20' }).date).toEqual([
      'Peminjaman maksimal 30 hari ke depan.',
    ])
  })

  it('validates the optional fields', () => {
    expect(errorsFor({ participants: '0' }).participants).toBeDefined()
    expect(errorsFor({ participants: '2.5' }).participants).toBeDefined()
    expect(errorsFor({ participants: '12' })).toEqual({})
    expect(errorsFor({ purpose: '   ' }).purpose).toEqual([
      'Keperluan wajib diisi.',
    ])
    expect(errorsFor({ purpose: 'x'.repeat(201) }).purpose).toBeDefined()
  })
})

describe('toCreateBookingRequest', () => {
  it('builds WIB timestamps and drops empty optional fields', () => {
    expect(toCreateBookingRequest(valid)).toEqual({
      room_id: valid.room_id,
      start_at: '2026-10-21T08:00:00+07:00',
      end_at: '2026-10-21T10:00:00+07:00',
      purpose: 'Latihan kelompok ManPro',
    })
  })

  it('sends participants as a number and trims notes', () => {
    expect(
      toCreateBookingRequest({
        ...valid,
        participants: ' 8 ',
        notes: ' Proyektor ',
      })
    ).toMatchObject({ participants: 8, notes: 'Proyektor' })
  })
})
