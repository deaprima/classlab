import { redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { type User, type UserRole } from './api-types'

export const ROLE_LABELS: Record<UserRole, string> = {
  MAHASISWA: 'Mahasiswa/PJ',
  TU_KALAB: 'TU / Kepala Lab',
  OB: 'OB',
  KAPRODI: 'Kaprodi',
}

// Who may open which page (brief.md 3 and 7.2). The server enforces the same rules; this only shapes
// navigation. OB has no booking list because GET /bookings answers 403 for OB (decisions.md, Phase 3).
export const ACCESS = {
  createBooking: ['MAHASISWA'],
  viewBookings: ['MAHASISWA', 'TU_KALAB', 'KAPRODI'],
  approveBookings: ['TU_KALAB'],
  manageRooms: ['TU_KALAB', 'KAPRODI'],
  manageSchedules: ['TU_KALAB', 'KAPRODI'],
  manageUsers: ['KAPRODI'],
  updateReportStatus: ['TU_KALAB'],
} as const satisfies Record<string, readonly UserRole[]>

export function hasRole(
  user: Pick<User, 'role'> | null,
  roles: readonly UserRole[]
): boolean {
  return !!user && roles.includes(user.role)
}

/** For `beforeLoad` of a child of `/_authenticated`, which has already restored the session. */
export function requireRole(roles: readonly UserRole[]) {
  if (!hasRole(useAuthStore.getState().auth.user, roles)) {
    throw redirect({ to: '/403' })
  }
}

/** Current user's role check for components. */
export function useHasRole(roles: readonly UserRole[]): boolean {
  return useAuthStore((state) => hasRole(state.auth.user, roles))
}
