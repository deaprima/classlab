import { describe, expect, it } from 'vitest'
import { type BookingStatus, type UserRole } from '@/lib/api-types'
import { availableActions } from './transitions'

const NOW = Date.parse('2026-10-20T09:00:00+07:00')
const OWNER = 'owner-id'
const future = '2026-10-21T10:00:00+07:00'
const past = '2026-10-19T10:00:00+07:00'

function actionsFor(
  status: BookingStatus,
  role: UserRole,
  opts: { own?: boolean; endAt?: string } = {}
) {
  return availableActions(
    { status, requester_id: OWNER, end_at: opts.endAt ?? future },
    { id: opts.own ? OWNER : 'someone-else', role },
    NOW
  )
}

describe('availableActions (brief 4.1)', () => {
  it('lets TU/Kalab approve or reject a pending request', () => {
    expect(actionsFor('MENUNGGU', 'TU')).toEqual({
      approve: true,
      reject: true,
      cancelOwn: false,
      cancelApproved: false,
    })
  })

  it('only allows rejecting a pending request whose time has passed', () => {
    expect(actionsFor('MENUNGGU', 'TU', { endAt: past })).toMatchObject({
      approve: false,
      reject: true,
    })
  })

  it('lets the owner cancel only while pending', () => {
    expect(actionsFor('MENUNGGU', 'MAHASISWA', { own: true }).cancelOwn).toBe(
      true
    )
    expect(actionsFor('DISETUJUI', 'MAHASISWA', { own: true }).cancelOwn).toBe(
      false
    )
    expect(actionsFor('MENUNGGU', 'MAHASISWA').cancelOwn).toBe(false)
  })

  it('lets TU/Kalab cancel an approved booking that has not ended', () => {
    expect(actionsFor('DISETUJUI', 'TU').cancelApproved).toBe(true)
    expect(actionsFor('DISETUJUI', 'TU', { endAt: past }).cancelApproved).toBe(
      false
    )
  })

  it.each<BookingStatus>(['DITOLAK', 'DIBATALKAN', 'SELESAI', 'KEDALUWARSA'])(
    'offers nothing on final status %s',
    (status) => {
      for (const role of ['MAHASISWA', 'TU', 'KALAB', 'ADMIN'] as const) {
        expect(Object.values(actionsFor(status, role, { own: true }))).toEqual([
          false,
          false,
          false,
          false,
        ])
      }
    }
  )

  it.each<UserRole>(['KALAB', 'ADMIN'])(
    'lets %s act as an approver (the server scopes by room type)',
    (role) => {
      expect(actionsFor('MENUNGGU', role)).toMatchObject({
        approve: true,
        reject: true,
      })
    }
  )

  it.each<UserRole>(['ORGANISASI', 'OB'])(
    'gives %s no approval actions',
    (role) => {
      expect(Object.values(actionsFor('MENUNGGU', role)).some(Boolean)).toBe(
        false
      )
    }
  )
})
