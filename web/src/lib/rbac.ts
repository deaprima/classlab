import { redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { type User, type UserRole } from './api-types'

export const ROLE_LABELS: Record<UserRole, string> = {
  MAHASISWA: 'Mahasiswa',
  ORGANISASI: 'Organisasi',
  TU: 'TU',
  KALAB: 'Kepala Lab',
  OB: 'OB',
  ADMIN: 'Admin',
}

// Who may open which page (brief.md 3 and 7.2). The server enforces the same rules; this only shapes
// navigation. OB has no booking list because GET /bookings answers 403 for OB (decisions.md, Phase 3).
export const ACCESS = {
  createBooking: ['MAHASISWA', 'ORGANISASI'],
  viewBookings: ['MAHASISWA', 'ORGANISASI', 'TU', 'KALAB', 'ADMIN'],
  // The server narrows approval by room type (TU: classrooms, halls, exam and defense rooms;
  // Kalab: labs; Admin: only types that list ADMIN).
  approveBookings: ['TU', 'KALAB', 'ADMIN'],
  manageRooms: ['TU', 'KALAB', 'ADMIN'],
  manageSchedules: ['TU', 'KALAB', 'ADMIN'],
  // The all-rooms day board; requesters pick a room on /schedule instead.
  // Photos of how rooms were left; the server scopes TU and KALAB to the types they approve.
  verifyCheckouts: ['TU', 'KALAB', 'OB', 'ADMIN'],
  viewRoomBoard: ['TU', 'KALAB', 'OB', 'ADMIN'],
  manageUsers: ['ADMIN'],
  updateReportStatus: ['TU', 'KALAB', 'ADMIN'],
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
