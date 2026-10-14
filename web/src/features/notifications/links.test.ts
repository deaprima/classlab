import { describe, expect, it } from 'vitest'
import { formatRelative } from '@/lib/datetime'
import { notificationTarget } from './links'

const id = '12b750c3-627d-45be-b3d3-cb7cc41845d1'

describe('notificationTarget', () => {
  it('sends TU/Kalab from a new request to the approval queue', () => {
    expect(
      notificationTarget(
        { type: 'BOOKING_CREATED', ref_type: 'BOOKING', ref_id: id },
        'TU_KALAB'
      )
    ).toEqual({ to: '/approvals', search: { booking: id } })
  })

  it('sends the requester to their booking', () => {
    expect(
      notificationTarget(
        { type: 'BOOKING_REJECTED', ref_type: 'BOOKING', ref_id: id },
        'MAHASISWA'
      )
    ).toEqual({ to: '/bookings', search: { booking: id } })
  })

  it('opens reports for any role', () => {
    expect(
      notificationTarget(
        { type: 'REPORT_STATUS_CHANGED', ref_type: 'REPORT', ref_id: id },
        'OB'
      )
    ).toEqual({ to: '/reports', search: { report: id } })
  })

  it('has no target without a reference or a page the role can open', () => {
    expect(
      notificationTarget(
        { type: 'X', ref_type: undefined, ref_id: undefined },
        'OB'
      )
    ).toBeNull()
    expect(
      notificationTarget(
        { type: 'BOOKING_APPROVED', ref_type: 'BOOKING', ref_id: id },
        'OB'
      )
    ).toBeNull()
  })
})

describe('formatRelative', () => {
  const now = new Date('2026-10-20T12:00:00+07:00')
  it.each([
    ['2026-10-20T11:59:30+07:00', 'baru saja'],
    ['2026-10-20T11:55:00+07:00', '5 menit yang lalu'],
    ['2026-10-20T09:00:00+07:00', '3 jam yang lalu'],
    ['2026-10-19T12:00:00+07:00', 'kemarin'],
    ['2026-10-01T08:00:00+07:00', 'Kam, 1 Okt 2026, 08:00'],
  ])('%s -> %s', (value, expected) => {
    expect(formatRelative(value, now)).toBe(expected)
  })
})
