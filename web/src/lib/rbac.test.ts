import { isRedirect } from '@tanstack/react-router'
import { sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, describe, expect, it } from 'vitest'
import { useAuthStore } from '@/stores/auth-store'
import { navGroupsForRole } from '@/components/layout/data/sidebar-data'
import { type UserRole } from './api-types'
import { ACCESS, requireRole } from './rbac'

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
    signInAs('TU_KALAB')
    expect(caught(() => requireRole(ACCESS.approveBookings))).toBeUndefined()
  })

  it.each<[UserRole, readonly UserRole[]]>([
    ['MAHASISWA', ACCESS.approveBookings],
    ['KAPRODI', ACCESS.approveBookings],
    ['TU_KALAB', ACCESS.createBooking],
    ['OB', ACCESS.viewBookings],
    ['TU_KALAB', ACCESS.manageUsers],
  ])('redirects %s to 403', (role, roles) => {
    signInAs(role)
    const error = caught(() => requireRole(roles))
    expect(isRedirect(error)).toBe(true)
  })
})

describe('navGroupsForRole', () => {
  const titles = (role: UserRole) =>
    navGroupsForRole(role).flatMap((g) => g.items.map((i) => i.title))

  it('shows students how to book but not how to approve', () => {
    expect(titles('MAHASISWA')).toContain('Ajukan Peminjaman')
    expect(titles('MAHASISWA')).not.toContain('Persetujuan')
    expect(titles('MAHASISWA')).not.toContain('Kelola Ruang')
  })

  it('gives TU/Kalab the approval queue and room admin, not user admin', () => {
    expect(titles('TU_KALAB')).toEqual(
      expect.arrayContaining(['Persetujuan', 'Kelola Ruang', 'Kelola Jadwal'])
    )
    expect(titles('TU_KALAB')).not.toContain('Kelola Pengguna')
    expect(titles('TU_KALAB')).not.toContain('Ajukan Peminjaman')
  })

  it('hides the booking list from OB, which the API forbids', () => {
    expect(titles('OB')).not.toContain('Riwayat Peminjaman')
    expect(titles('OB')).toContain('Laporan Kerusakan')
    expect(navGroupsForRole('OB').map((g) => g.title)).not.toContain(
      'Administrasi'
    )
  })

  it('gives Kaprodi user admin', () => {
    expect(titles('KAPRODI')).toContain('Kelola Pengguna')
  })

  it('shows nothing without a role', () => {
    expect(navGroupsForRole(undefined)).toEqual([])
  })
})
