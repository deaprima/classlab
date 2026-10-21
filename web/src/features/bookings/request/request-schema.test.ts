import { describe, expect, it } from 'vitest'
import { type Room, type RoomTypePolicy } from '@/lib/api-types'
import {
  createRequestSchema,
  EMPTY_REQUEST,
  lastBookableDate,
  MAX_OCCURRENCES,
  type RequestFormValues,
  toCreateBookingRequest,
} from './request-schema'

// Wednesday 2026-10-21, 09:00 WIB.
const NOW = new Date('2026-10-21T09:00:00+07:00')
const now = () => NOW

const policy: RoomTypePolicy = {
  type: 'KELAS',
  label: 'Ruang Kelas',
  approver_roles: ['TU'],
  requester_roles: ['MAHASISWA', 'ORGANISASI'],
  approval_policy: 'AUTO_IF_ELIGIBLE',
  letter_required: false,
  open_time: '07:00',
  close_time: '21:00',
  min_duration_min: 30,
  max_duration_min: 240,
  max_advance_days: 7,
  updated_at: '2026-10-01T00:00:00Z',
}

const room: Room = {
  id: 'room-1',
  org_unit_id: 'org',
  code: 'R-302',
  name: 'Ruang 302',
  type: 'KELAS',
  capacity: 50,
  facilities: [],
  is_active: true,
  open_time: null,
  close_time: null,
}

const valid: RequestFormValues = {
  ...EMPTY_REQUEST,
  room_id: 'room-1',
  occurrences: [{ date: '2026-10-22', start_time: '08:00', end_time: '10:00' }],
  purpose: 'Kuliah pengganti Kalkulus',
  participants: '35',
}

function issues(
  values: RequestFormValues,
  ctx: Partial<Parameters<typeof createRequestSchema>[0]> = {}
) {
  const result = createRequestSchema({
    role: 'MAHASISWA',
    policy,
    room,
    now,
    ...ctx,
  }).safeParse(values)
  return result.success
    ? []
    : result.error.issues.map((i) => `${i.path.join('.')}: ${i.message}`)
}

describe('createRequestSchema', () => {
  it('accepts a valid request', () => {
    expect(issues(valid)).toEqual([])
  })

  it('requires a room, a purpose and at least one date', () => {
    expect(issues({ ...valid, room_id: '' })).toContain('room_id: Pilih ruang.')
    expect(issues({ ...valid, purpose: '  ' })).toContain(
      'purpose: Nama kegiatan wajib diisi.'
    )
    expect(issues({ ...valid, occurrences: [] })).toContain(
      'occurrences: Tambahkan minimal satu tanggal.'
    )
  })

  it('checks every date against the policy limits', () => {
    const at = (start: string, end: string, date = '2026-10-22') => ({
      ...valid,
      occurrences: [{ date, start_time: start, end_time: end }],
    })
    expect(issues(at('08:15', '10:00'))).toContain(
      'occurrences.0.start_time: Jam mulai harus kelipatan 30 menit.'
    )
    expect(issues(at('10:00', '10:00'))).toContain(
      'occurrences.0.end_time: Jam selesai harus setelah jam mulai.'
    )
    expect(issues(at('08:00', '13:00'))).toContain(
      'occurrences.0.end_time: Durasi maksimal 4 jam.'
    )
    expect(issues(at('06:30', '08:00'))).toContain(
      'occurrences.0.start_time: Ruang buka mulai pukul 07:00 WIB.'
    )
    expect(issues(at('20:00', '21:30'))).toContain(
      'occurrences.0.end_time: Ruang tutup pukul 21:00 WIB.'
    )
  })

  it('uses the hours of the room when it has its own', () => {
    const early = { ...room, open_time: '09:00', close_time: '12:00' }
    expect(issues(valid, { room: early })).toContain(
      'occurrences.0.start_time: Ruang buka mulai pukul 09:00 WIB.'
    )
  })

  it('rejects the past and dates beyond the advance limit', () => {
    expect(
      issues({
        ...valid,
        occurrences: [
          { date: '2026-10-20', start_time: '08:00', end_time: '10:00' },
        ],
      })
    ).toContain('occurrences.0.date: Tanggal sudah lewat.')
    expect(
      issues({
        ...valid,
        occurrences: [
          { date: '2026-10-21', start_time: '08:00', end_time: '09:00' },
        ],
      })
    ).toContain('occurrences.0.start_time: Jam mulai sudah lewat.')
    expect(lastBookableDate(policy, NOW)).toBe('2026-10-28')
    expect(
      issues({
        ...valid,
        occurrences: [
          { date: '2026-10-29', start_time: '08:00', end_time: '10:00' },
        ],
      })
    ).toContain('occurrences.0.date: Bisa diajukan maksimal 7 hari ke depan.')
  })

  it('allows several dates but not overlapping ones on the same day', () => {
    const two = {
      ...valid,
      occurrences: [
        { date: '2026-10-22', start_time: '08:00', end_time: '10:00' },
        { date: '2026-10-23', start_time: '09:00', end_time: '12:00' },
      ],
    }
    expect(issues(two)).toEqual([])
    const overlap = {
      ...valid,
      occurrences: [
        { date: '2026-10-22', start_time: '08:00', end_time: '10:00' },
        { date: '2026-10-22', start_time: '09:30', end_time: '11:00' },
      ],
    }
    expect(issues(overlap)).toContain(
      'occurrences.1.start_time: Bertumpuk dengan tanggal lain di pengajuan ini.'
    )
    const touching = {
      ...valid,
      occurrences: [
        { date: '2026-10-22', start_time: '08:00', end_time: '10:00' },
        { date: '2026-10-22', start_time: '10:00', end_time: '11:00' },
      ],
    }
    expect(issues(touching)).toEqual([])
  })

  it(`caps a request at ${MAX_OCCURRENCES} dates`, () => {
    const many = Array.from({ length: MAX_OCCURRENCES + 1 }, (_, i) => ({
      date: `2026-10-${String(22 + (i % 6)).padStart(2, '0')}`,
      start_time: '08:00',
      end_time: '09:00',
    }))
    expect(issues({ ...valid, occurrences: many })).toContain(
      `occurrences: Maksimal ${MAX_OCCURRENCES} tanggal sekali ajuan.`
    )
  })

  it('keeps the headcount within the room capacity', () => {
    expect(issues({ ...valid, participants: '51' })).toContain(
      'participants: Melebihi kapasitas ruang (50 orang).'
    )
    expect(issues({ ...valid, participants: '50' })).toEqual([])
    expect(issues({ ...valid, participants: '' })).toEqual([])
  })

  it('requires the headcount for an auditorium', () => {
    const aud = { ...room, type: 'AUDITORIUM' as const, capacity: 200 }
    expect(issues({ ...valid, participants: '' }, { room: aud })).toContain(
      'participants: Jumlah peserta wajib diisi.'
    )
  })

  it('requires a letter for an organisation or a type that asks for one', () => {
    expect(issues(valid, { role: 'ORGANISASI' })).toContain(
      'letter_attachment_id: Surat wajib dilampirkan.'
    )
    expect(
      issues(
        { ...valid, letter_attachment_id: 'att-1' },
        { role: 'ORGANISASI' }
      )
    ).toEqual([])
    expect(
      issues(valid, { policy: { ...policy, letter_required: true } })
    ).toContain('letter_attachment_id: Surat wajib dilampirkan.')
  })

  it('limits purpose, notes and letter number', () => {
    expect(issues({ ...valid, purpose: 'x'.repeat(201) })).toContain(
      'purpose: Nama kegiatan maksimal 200 karakter.'
    )
    expect(issues({ ...valid, notes: 'x'.repeat(1001) })).toContain(
      'notes: Catatan maksimal 1000 karakter.'
    )
    expect(issues({ ...valid, letter_number: 'x'.repeat(101) })).toContain(
      'letter_number: Nomor surat maksimal 100 karakter.'
    )
  })
})

describe('toCreateBookingRequest', () => {
  it('sends one date as start_at/end_at, never both forms', () => {
    const body = toCreateBookingRequest(valid)
    expect(body).toEqual({
      room_id: 'room-1',
      start_at: '2026-10-22T08:00:00+07:00',
      end_at: '2026-10-22T10:00:00+07:00',
      purpose: 'Kuliah pengganti Kalkulus',
      participants: 35,
    })
    expect(body).not.toHaveProperty('occurrences')
  })

  it('sends several dates as occurrences', () => {
    const body = toCreateBookingRequest({
      ...valid,
      occurrences: [
        { date: '2026-10-22', start_time: '08:00', end_time: '10:00' },
        { date: '2026-10-23', start_time: '09:00', end_time: '12:00' },
      ],
      notes: ' Butuh proyektor ',
      letter_attachment_id: 'att-1',
      letter_number: '012/HMIF/X/2026',
    })
    expect(body).toEqual({
      room_id: 'room-1',
      occurrences: [
        {
          start_at: '2026-10-22T08:00:00+07:00',
          end_at: '2026-10-22T10:00:00+07:00',
        },
        {
          start_at: '2026-10-23T09:00:00+07:00',
          end_at: '2026-10-23T12:00:00+07:00',
        },
      ],
      purpose: 'Kuliah pengganti Kalkulus',
      participants: 35,
      notes: 'Butuh proyektor',
      letter_attachment_id: 'att-1',
      letter_number: '012/HMIF/X/2026',
    })
    expect(body).not.toHaveProperty('start_at')
  })

  it('leaves out empty optional fields', () => {
    const body = toCreateBookingRequest({ ...valid, participants: '' })
    expect(body).not.toHaveProperty('participants')
    expect(body).not.toHaveProperty('notes')
    expect(body).not.toHaveProperty('letter_attachment_id')
  })
})
