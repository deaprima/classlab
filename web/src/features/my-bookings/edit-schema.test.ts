import { describe, expect, it } from 'vitest'
import { type Booking, type Room, type RoomTypePolicy } from '@/lib/api-types'
import {
  createEditSchema,
  editDefaults,
  toEditRequest,
  type EditFormValues,
} from './edit-schema'

// Wednesday 2026-10-21, 09:00 WIB.
const NOW = new Date('2026-10-21T09:00:00+07:00')

const policy: RoomTypePolicy = {
  type: 'KELAS',
  label: 'Ruang Kelas',
  approver_roles: ['TU'],
  requester_roles: ['MAHASISWA'],
  approval_policy: 'AUTO_IF_ELIGIBLE',
  letter_required: false,
  open_time: '07:00',
  close_time: '21:00',
  min_duration_min: 30,
  max_duration_min: 240,
  max_advance_days: 30,
  updated_at: 'x',
}

const room: Room = {
  id: 'r',
  org_unit_id: 'o',
  code: 'R-302',
  name: 'R-302',
  type: 'KELAS',
  capacity: 50,
  facilities: [],
  is_active: true,
  open_time: null,
  close_time: null,
}

const booking = {
  start_at: '2026-10-22T10:00:00+07:00',
  end_at: '2026-10-22T12:00:00+07:00',
  purpose: 'Praktikum',
  participants: 32,
  notes: '',
} as Booking

function issues(v: EditFormValues, r: Room = room) {
  const result = createEditSchema({
    room: r,
    policy,
    now: () => NOW,
  }).safeParse(v)
  return result.success
    ? []
    : result.error.issues.map((i) => `${i.path.join('.')}: ${i.message}`)
}

describe('editDefaults', () => {
  it('starts from what the booking is now, in WIB', () => {
    expect(editDefaults(booking)).toEqual({
      date: '2026-10-22',
      start_time: '10:00',
      end_time: '12:00',
      purpose: 'Praktikum',
      participants: '32',
      notes: '',
    })
  })
})

describe('createEditSchema', () => {
  const ok = editDefaults(booking)

  it('accepts the booking as it is', () => {
    expect(issues(ok)).toEqual([])
  })

  it('applies the policy limits to the new time', () => {
    expect(issues({ ...ok, end_time: '16:00' })).toContain(
      'end_time: Durasi maksimal 4 jam.'
    )
    expect(issues({ ...ok, start_time: '12:00', end_time: '11:00' })).toContain(
      'end_time: Jam selesai harus setelah jam mulai.'
    )
    expect(issues({ ...ok, date: '2026-10-20' })).toContain(
      'date: Tanggal sudah lewat.'
    )
  })

  it('keeps the headcount within the room capacity', () => {
    expect(issues({ ...ok, participants: '51' })).toContain(
      'participants: Melebihi kapasitas ruang (50 orang).'
    )
  })

  it('requires a purpose', () => {
    expect(issues({ ...ok, purpose: ' ' })).toContain(
      'purpose: Nama kegiatan wajib diisi.'
    )
  })

  it('requires the headcount for an auditorium', () => {
    const aud = { ...room, type: 'AUDITORIUM' as const, capacity: 200 }
    expect(issues({ ...ok, participants: '' }, aud)).toContain(
      'participants: Jumlah peserta wajib diisi.'
    )
  })
})

describe('toEditRequest', () => {
  const ok = editDefaults(booking)

  it('is null when nothing changed', () => {
    expect(toEditRequest(booking, ok)).toBeNull()
  })

  it('sends only what changed', () => {
    expect(
      toEditRequest(booking, { ...ok, purpose: 'Praktikum Basis Data' })
    ).toEqual({
      purpose: 'Praktikum Basis Data',
    })
    expect(toEditRequest(booking, { ...ok, participants: '40' })).toEqual({
      participants: 40,
    })
  })

  it('sends both times when either the date or a time changed', () => {
    expect(toEditRequest(booking, { ...ok, end_time: '11:00' })).toEqual({
      start_at: '2026-10-22T10:00:00+07:00',
      end_at: '2026-10-22T11:00:00+07:00',
    })
    expect(toEditRequest(booking, { ...ok, date: '2026-10-23' })).toEqual({
      start_at: '2026-10-23T10:00:00+07:00',
      end_at: '2026-10-23T12:00:00+07:00',
    })
  })

  it('clears the note with an empty string', () => {
    const withNote = { ...booking, notes: 'Butuh proyektor' } as Booking
    expect(
      toEditRequest(withNote, { ...editDefaults(withNote), notes: '' })
    ).toEqual({
      notes: '',
    })
  })
})
