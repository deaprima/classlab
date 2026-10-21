import { isRedirect } from '@tanstack/react-router'
import { sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, describe, expect, it } from 'vitest'
import { useAuthStore } from '@/stores/auth-store'
import {
  navGroupsForRole,
  plannedMenuForRole,
} from '@/components/layout/data/sidebar-data'
import { type UserRole } from './api-types'
import { ACCESS, requireRole } from './rbac'

const ROLES: UserRole[] = [
  'MAHASISWA',
  'ORGANISASI',
  'TU',
  'KALAB',
  'OB',
  'ADMIN',
]

function signInAs(role: UserRole) {
  useAuthStore.getState().auth.setSession('token', { ...sampleUser, role })
}

function caught(fn: () => void): unknown {
  try {
    fn()
  } catch (error) {
    return error
  }
  return undefined
}

afterEach(() => {
  useAuthStore.getState().auth.reset()
})

describe('requireRole', () => {
  it('lets an allowed role through', () => {
    signInAs('TU')
    expect(caught(() => requireRole(ACCESS.approveBookings))).toBeUndefined()
  })

  it.each<[UserRole, readonly UserRole[]]>([
    ['MAHASISWA', ACCESS.approveBookings],
    ['ORGANISASI', ACCESS.approveBookings],
    ['OB', ACCESS.approveBookings],
    ['TU', ACCESS.createBooking],
    ['ADMIN', ACCESS.createBooking],
    ['OB', ACCESS.viewBookings],
    ['TU', ACCESS.manageUsers],
    ['KALAB', ACCESS.manageUsers],
  ])('redirects %s to 403', (role, roles) => {
    signInAs(role)
    const error = caught(() => requireRole(roles))
    expect(isRedirect(error)).toBe(true)
  })
})

describe('sidebar menu per role (DESIGN.md section 5)', () => {
  const planned = (role: UserRole) =>
    plannedMenuForRole(role).map((i) => i.title)
  const shown = (role: UserRole) =>
    navGroupsForRole(role).flatMap((g) => g.items.map((i) => i.title))

  it.each<[UserRole, string[]]>([
    [
      'MAHASISWA',
      [
        'Dashboard',
        'Jadwal Ruang',
        'Pengajuan Saya',
        'Riwayat',
        'Lapor Kerusakan',
      ],
    ],
    [
      'ORGANISASI',
      [
        'Dashboard',
        'Jadwal Ruang',
        'Pengajuan Saya',
        'Riwayat',
        'Lapor Kerusakan',
      ],
    ],
    [
      'TU',
      [
        'Beranda',
        'Approval',
        'Ruang & Jadwal',
        'Verifikasi',
        'Laporan',
        'Mahasiswa',
      ],
    ],
    ['KALAB', ['Beranda', 'Approval Lab', 'Ruang & Jadwal', 'Laporan']],
    ['OB', ['Beranda', 'Verifikasi', 'Laporan', 'Notifikasi']],
    [
      'ADMIN',
      [
        'Beranda',
        'Approval',
        'Ruang & Jadwal',
        'Pengguna',
        'Kebijakan Ruang',
        'Audit Log',
      ],
    ],
  ])('%s sees the designed menu in order', (role, titles) => {
    expect(planned(role)).toEqual(titles)
    expect(titles.length).toBeLessThanOrEqual(6)
  })

  it('only links to pages that exist, and never shows a link without a page', () => {
    expect(shown('MAHASISWA')).toEqual([
      'Dashboard',
      'Jadwal Ruang',
      'Pengajuan Saya',
      'Riwayat',
      'Lapor Kerusakan',
    ])
    for (const role of ROLES) {
      for (const item of navGroupsForRole(role).flatMap((g) => g.items)) {
        expect(item.url).toBeTruthy()
      }
    }
  })

  it('keeps approval and administration away from requesters', () => {
    expect(shown('MAHASISWA')).not.toContain('Approval')
    expect(shown('ORGANISASI')).not.toContain('Ruang & Jadwal')
    expect(shown('TU')).not.toContain('Pengguna')
    expect(shown('KALAB')).not.toContain('Pengajuan Saya')
  })

  it('gives Admin user management and OB no booking list', () => {
    expect(shown('ADMIN')).toContain('Pengguna')
    expect(planned('OB')).not.toContain('Pengajuan Saya')
  })

  it('shows nothing without a role', () => {
    expect(navGroupsForRole(undefined)).toEqual([])
  })
})
