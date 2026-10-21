import { describe, expect, it } from 'vitest'
import { type Booking } from '@/lib/api-types'
import {
  canCancel,
  canEdit,
  isLateCancel,
  matchesQuery,
  mayNeedReapproval,
  statusNote,
} from './rules'

const START = '2026-10-21T14:00:00+07:00'
const at = (hhmm: string) => new Date(`2026-10-21T${hhmm}:00+07:00`)

function booking(over: Partial<Booking> = {}): Booking {
  return {
    status: 'DISETUJUI',
    approval_mode: 'AUTO',
    start_at: START,
    expires_at: null,
    decision_reason: '',
    checked_out_at: null,
    ...over,
  } as Booking
}

describe('canEdit', () => {
  it('is open until 60 minutes before the start', () => {
    expect(canEdit(booking(), at('12:59'))).toBe(true)
    expect(canEdit(booking(), at('13:00'))).toBe(false)
    expect(canEdit(booking(), at('13:30'))).toBe(false)
  })

  it('covers waiting and approved bookings only', () => {
    expect(canEdit(booking({ status: 'MENUNGGU' }), at('09:00'))).toBe(true)
    for (const status of [
      'DITOLAK',
      'DIBATALKAN',
      'BERLANGSUNG',
      'SELESAI',
      'KEDALUWARSA',
      'TIDAK_HADIR',
      'TANPA_CHECKOUT',
    ] as const) {
      expect(canEdit(booking({ status }), at('09:00'))).toBe(false)
    }
  })
})

describe('canCancel', () => {
  it('is open until the start, for waiting and approved bookings', () => {
    expect(canCancel(booking(), at('13:59'))).toBe(true)
    expect(canCancel(booking(), at('14:00'))).toBe(false)
    expect(canCancel(booking({ status: 'MENUNGGU' }), at('13:59'))).toBe(true)
    expect(canCancel(booking({ status: 'BERLANGSUNG' }), at('09:00'))).toBe(
      false
    )
    expect(canCancel(booking({ status: 'DITOLAK' }), at('09:00'))).toBe(false)
  })
})

describe('isLateCancel', () => {
  it('costs a point when an approved booking is cancelled under 2 hours before the start', () => {
    expect(isLateCancel(booking(), at('11:59'))).toBe(false)
    expect(isLateCancel(booking(), at('12:01'))).toBe(true)
  })

  it('never costs a point for a waiting request', () => {
    expect(isLateCancel(booking({ status: 'MENUNGGU' }), at('13:00'))).toBe(
      false
    )
  })
})

describe('mayNeedReapproval', () => {
  it('only for approved bookings', () => {
    expect(mayNeedReapproval({ status: 'DISETUJUI' })).toBe(true)
    expect(mayNeedReapproval({ status: 'MENUNGGU' })).toBe(false)
  })
})

describe('statusNote', () => {
  it('says who validates and how long is left for a waiting request', () => {
    const note = statusNote(
      booking({
        status: 'MENUNGGU',
        expires_at: '2026-10-22T09:40:00+07:00',
      }),
      at('10:00'),
      'TU'
    )
    expect(note.title).toBe('Menunggu validasi TU')
    expect(note.description).toBe(
      'Kedaluwarsa otomatis dalam 23 jam 40 menit bila tidak diputuskan.'
    )
  })

  it('tells approved-by-system from approved-by-person', () => {
    expect(statusNote(booking(), at('10:00'), 'TU').title).toBe(
      'Disetujui otomatis'
    )
    expect(
      statusNote(
        booking({ approval_mode: 'MANUAL' }),
        at('10:00'),
        'Kepala Lab'
      ).title
    ).toBe('Disetujui Kepala Lab')
  })

  it('gives the rejection reason in the error tone', () => {
    const note = statusNote(
      booking({ status: 'DITOLAK', decision_reason: 'Ruang dipakai ujian.' }),
      at('10:00'),
      'TU'
    )
    expect(note).toMatchObject({
      tone: 'error',
      description: 'Ruang dipakai ujian.',
    })
  })

  it('has a note for every status', () => {
    for (const status of [
      'MENUNGGU',
      'DISETUJUI',
      'DITOLAK',
      'DIBATALKAN',
      'BERLANGSUNG',
      'SELESAI',
      'TIDAK_HADIR',
      'TANPA_CHECKOUT',
      'KEDALUWARSA',
    ] as const) {
      expect(
        statusNote(booking({ status }), at('10:00'), 'TU').title
      ).toBeTruthy()
    }
  })
})

describe('matchesQuery', () => {
  const b = {
    purpose: 'Praktikum Basis Data',
    room: { code: 'LAB-RPL-02', name: 'Lab RPL' },
  }

  it('matches everything without a query', () => {
    expect(matchesQuery(b, undefined)).toBe(true)
    expect(matchesQuery(b, '  ')).toBe(true)
  })

  it('looks in the room code, room name and activity, ignoring case', () => {
    expect(matchesQuery(b, 'rpl-02')).toBe(true)
    expect(matchesQuery(b, 'lab rpl')).toBe(true)
    expect(matchesQuery(b, 'BASIS')).toBe(true)
    expect(matchesQuery(b, 'kimia')).toBe(false)
  })
})
