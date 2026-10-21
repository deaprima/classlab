import { describe, expect, it } from 'vitest'
import { type RoomTypePolicy } from '@/lib/api-types'
import {
  approverLabel,
  availableGroups,
  effectiveHours,
  groupOf,
  letterRequired,
  participantsRequired,
  previewOutcome,
  requestablePolicies,
} from './policy'

function policy(over: Partial<RoomTypePolicy>): RoomTypePolicy {
  return {
    type: 'KELAS',
    label: 'Ruang Kelas',
    approver_roles: ['TU'],
    requester_roles: ['MAHASISWA', 'ORGANISASI'],
    approval_policy: 'AUTO_IF_ELIGIBLE',
    letter_required: false,
    open_time: '07:00',
    close_time: '21:00',
    min_duration_min: 30,
    max_duration_min: 480,
    max_advance_days: 30,
    updated_at: '2026-10-01T00:00:00Z',
    ...over,
  }
}

const KELAS = policy({})
const LAB = policy({
  type: 'LAB',
  label: 'Laboratorium',
  approver_roles: ['KALAB'],
  approval_policy: 'ALWAYS_MANUAL',
})
const AUD = policy({
  type: 'AUDITORIUM',
  label: 'Auditorium',
  approver_roles: ['TU', 'ADMIN'],
  requester_roles: ['ORGANISASI'],
  approval_policy: 'ALWAYS_MANUAL',
  letter_required: true,
})
const UJIAN = policy({
  type: 'RUANG_UJIAN',
  approver_roles: ['TU', 'ADMIN'],
  approval_policy: 'ALWAYS_MANUAL',
})
const ALL = [AUD, UJIAN, LAB, KELAS]

describe('type groups', () => {
  it('groups everything but Kelas and Lab under Lainnya', () => {
    expect(groupOf('KELAS')).toBe('KELAS')
    expect(groupOf('LAB')).toBe('LAB')
    expect(groupOf('AUDITORIUM')).toBe('LAINNYA')
    expect(groupOf('RUANG_SIDANG')).toBe('LAINNYA')
  })

  it('offers a role only the types its policy lists, in display order', () => {
    expect(requestablePolicies(ALL, 'MAHASISWA').map((p) => p.type)).toEqual([
      'KELAS',
      'LAB',
      'RUANG_UJIAN',
    ])
    expect(requestablePolicies(ALL, 'ORGANISASI').map((p) => p.type)).toEqual([
      'KELAS',
      'LAB',
      'AUDITORIUM',
      'RUANG_UJIAN',
    ])
    expect(requestablePolicies(ALL, 'TU')).toEqual([])
  })

  it('lists the groups that have a type, in order', () => {
    expect(availableGroups([UJIAN, KELAS])).toEqual(['KELAS', 'LAINNYA'])
    expect(availableGroups([AUD])).toEqual(['LAINNYA'])
  })
})

describe('rules from the policy', () => {
  it('uses the room hours, else the type default', () => {
    expect(
      effectiveHours({ open_time: null, close_time: null }, KELAS)
    ).toEqual({ open: '07:00', close: '21:00' })
    expect(
      effectiveHours({ open_time: '08:00', close_time: '17:00' }, KELAS)
    ).toEqual({ open: '08:00', close: '17:00' })
  })

  it('requires a letter for an organisation or a type that asks for one', () => {
    expect(letterRequired(KELAS, 'MAHASISWA')).toBe(false)
    expect(letterRequired(KELAS, 'ORGANISASI')).toBe(true)
    expect(letterRequired(AUD, 'MAHASISWA')).toBe(true)
  })

  it('requires the headcount only for an auditorium', () => {
    expect(participantsRequired('AUDITORIUM')).toBe(true)
    expect(participantsRequired('KELAS')).toBe(false)
  })

  it('names the approvers', () => {
    expect(approverLabel(AUD)).toBe('TU / Admin')
    expect(approverLabel(LAB)).toBe('Kepala Lab')
  })
})

describe('previewOutcome', () => {
  const base = { blocked: false, slotFree: true as boolean | null }

  it('approves a free classroom automatically for a student', () => {
    const o = previewOutcome({
      ...base,
      policy: KELAS,
      role: 'MAHASISWA',
      approverLabel: 'TU',
    })
    expect(o.kind).toBe('auto')
    expect(o.title).toBe('Akan disetujui otomatis')
  })

  it('sends a lab to its approver', () => {
    const o = previewOutcome({
      ...base,
      policy: LAB,
      role: 'MAHASISWA',
      approverLabel: 'Kepala Lab',
    })
    expect(o.kind).toBe('manual')
    expect(o.title).toBe('Menunggu validasi Kepala Lab')
  })

  it('never auto-approves an organisation', () => {
    const o = previewOutcome({
      ...base,
      policy: KELAS,
      role: 'ORGANISASI',
      approverLabel: 'TU',
    })
    expect(o.kind).toBe('manual')
    expect(o.description).toMatch(/organisasi/)
  })

  it('reports a clash and a block before anything else', () => {
    const input = {
      policy: KELAS,
      role: 'MAHASISWA' as const,
      approverLabel: 'TU',
    }
    expect(
      previewOutcome({ ...input, blocked: false, slotFree: false }).kind
    ).toBe('conflict')
    expect(
      previewOutcome({ ...input, blocked: true, slotFree: true }).kind
    ).toBe('blocked')
  })

  it('is undecided until the availability is known', () => {
    expect(
      previewOutcome({
        policy: KELAS,
        role: 'MAHASISWA',
        approverLabel: 'TU',
        blocked: false,
        slotFree: null,
      }).kind
    ).toBe('unknown')
  })
})
