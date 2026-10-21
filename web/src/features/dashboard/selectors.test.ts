import { describe, expect, it } from 'vitest'
import { type Booking } from '@/lib/api-types'
import {
  attendanceState,
  countActive,
  expiresInLabel,
  greetingName,
  pickFeatured,
  pickSubmissions,
  pickUpcoming,
  startsInLabel,
} from './selectors'

let n = 0
function booking(over: Partial<Booking>): Booking {
  n += 1
  return {
    id: `b-${n}`,
    start_at: '2026-10-21T14:00:00+07:00',
    end_at: '2026-10-21T15:30:00+07:00',
    status: 'DISETUJUI',
    group_id: null,
    expires_at: null,
    ...over,
  } as Booking
}

const at = (hhmm: string) => new Date(`2026-10-21T${hhmm}:00+07:00`)

describe('pickFeatured', () => {
  it('prefers the room in use over a later approved one', () => {
    const later = booking({
      id: 'later',
      start_at: '2026-10-21T16:00:00+07:00',
    })
    const running = booking({ id: 'running', status: 'BERLANGSUNG' })
    expect(pickFeatured([later, running])?.id).toBe('running')
  })

  it('otherwise takes the earliest approved booking', () => {
    const a = booking({ id: 'a', start_at: '2026-10-22T08:00:00+07:00' })
    const b = booking({ id: 'b', start_at: '2026-10-21T16:00:00+07:00' })
    expect(pickFeatured([a, b])?.id).toBe('b')
  })

  it('ignores bookings that are not approved or running', () => {
    expect(pickFeatured([booking({ status: 'MENUNGGU' })])).toBeUndefined()
    expect(pickFeatured([])).toBeUndefined()
  })
})

describe('pickUpcoming', () => {
  it('lists later active bookings in order, without the featured one', () => {
    const f = booking({ id: 'f' })
    const x = booking({ id: 'x', start_at: '2026-10-23T09:00:00+07:00' })
    const y = booking({ id: 'y', start_at: '2026-10-22T09:00:00+07:00' })
    const pending = booking({ id: 'p', status: 'MENUNGGU' })
    expect(pickUpcoming([x, f, y, pending], f, 5).map((b) => b.id)).toEqual([
      'y',
      'x',
    ])
  })

  it('respects the limit', () => {
    const list = [1, 2, 3, 4].map((d) =>
      booking({ id: `d${d}`, start_at: `2026-10-2${d}T09:00:00+07:00` })
    )
    expect(pickUpcoming(list, undefined, 3)).toHaveLength(3)
  })
})

describe('pickSubmissions', () => {
  it('puts waiting requests first by expiry, then rejections', () => {
    const late = booking({
      id: 'late',
      status: 'MENUNGGU',
      expires_at: '2026-10-22T10:00:00+07:00',
    })
    const soon = booking({
      id: 'soon',
      status: 'MENUNGGU',
      expires_at: '2026-10-21T20:00:00+07:00',
    })
    const no = booking({ id: 'no', status: 'DITOLAK' })
    const ok = booking({ id: 'ok', status: 'DISETUJUI' })
    expect(pickSubmissions([no, late, ok, soon], 5).map((b) => b.id)).toEqual([
      'soon',
      'late',
      'no',
    ])
  })
})

describe('countActive', () => {
  it('counts a multi-date request once', () => {
    const g = 'group-1'
    const list = [
      booking({ group_id: g, status: 'DISETUJUI' }),
      booking({ group_id: g, status: 'DISETUJUI' }),
      booking({ status: 'MENUNGGU' }),
      booking({ status: 'DITOLAK' }),
      booking({ status: 'SELESAI' }),
    ]
    expect(countActive(list)).toBe(2)
  })
})

describe('attendanceState', () => {
  const b = booking({})

  it('opens check-in 15 minutes before the start, inclusive', () => {
    expect(attendanceState(b, at('13:44')).canCheckIn).toBe(false)
    expect(attendanceState(b, at('13:45')).canCheckIn).toBe(true)
  })

  it('closes check-in 15 minutes after the start, inclusive', () => {
    expect(attendanceState(b, at('14:15')).canCheckIn).toBe(true)
    expect(attendanceState(b, at('14:16')).canCheckIn).toBe(false)
  })

  it('offers check-out only while the booking is running', () => {
    const running = booking({ status: 'BERLANGSUNG' })
    expect(attendanceState(running, at('14:30'))).toEqual({
      step: 'check-out',
      canCheckIn: false,
      canCheckOut: true,
    })
    expect(attendanceState(b, at('14:00')).canCheckOut).toBe(false)
  })

  it('offers nothing for a booking that is not approved', () => {
    expect(
      attendanceState(booking({ status: 'MENUNGGU' }), at('14:00')).step
    ).toBe('none')
  })
})

describe('labels', () => {
  const b = booking({})

  it('says when the booking starts', () => {
    expect(startsInLabel(b, at('13:45'))).toBe('Mulai dalam 15 menit')
    expect(startsInLabel(b, at('11:00'))).toBe('Mulai dalam 3 jam')
    expect(startsInLabel(b, at('14:05'))).toBe('Sudah dimulai')
    expect(startsInLabel(booking({ status: 'BERLANGSUNG' }), at('14:05'))).toBe(
      'Sedang berlangsung'
    )
    expect(
      startsInLabel(booking({ status: 'MENUNGGU' }), at('14:05'))
    ).toBeNull()
  })

  it('says how long a waiting request has left', () => {
    const w = booking({
      status: 'MENUNGGU',
      expires_at: '2026-10-22T10:00:00+07:00',
    })
    expect(expiresInLabel(w, at('11:00'))).toBe('sisa 23 j')
    expect(expiresInLabel(w, new Date('2026-10-22T09:30:00+07:00'))).toBe(
      'sisa 30 mnt'
    )
    expect(expiresInLabel(w, new Date('2026-10-22T10:01:00+07:00'))).toBe(
      'kedaluwarsa'
    )
    expect(expiresInLabel(b, at('11:00'))).toBeNull()
  })
})

describe('greetingName', () => {
  it('uses the first name for students and the full name for organisations', () => {
    expect(greetingName({ name: 'Nisrina Nur Fatin', role: 'MAHASISWA' })).toBe(
      'Nisrina'
    )
    expect(greetingName({ name: 'HIMA Informatika', role: 'ORGANISASI' })).toBe(
      'HIMA Informatika'
    )
  })
})
