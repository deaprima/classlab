import {
  Bell,
  CalendarClock,
  CalendarPlus,
  ClipboardCheck,
  DoorOpen,
  History,
  LayoutGrid,
  Palette,
  TriangleAlert,
  Users,
} from 'lucide-react'
import { type UserRole } from '@/lib/api-types'
import { ACCESS, hasRole } from '@/lib/rbac'
import { type NavGroup } from '../types'

const navGroups: NavGroup[] = [
  {
    title: 'Peminjaman',
    items: [
      { title: 'Ketersediaan Ruang', url: '/', icon: LayoutGrid },
      {
        title: 'Ajukan Peminjaman',
        url: '/bookings/new',
        icon: CalendarPlus,
        roles: ACCESS.createBooking,
      },
      {
        title: 'Riwayat Peminjaman',
        url: '/bookings',
        icon: History,
        roles: ACCESS.viewBookings,
      },
      {
        title: 'Persetujuan',
        url: '/approvals',
        icon: ClipboardCheck,
        roles: ACCESS.approveBookings,
      },
    ],
  },
  {
    title: 'Umum',
    items: [
      { title: 'Notifikasi', url: '/notifications', icon: Bell },
      { title: 'Laporan Kerusakan', url: '/reports', icon: TriangleAlert },
    ],
  },
  {
    title: 'Administrasi',
    items: [
      {
        title: 'Kelola Ruang',
        url: '/admin/rooms',
        icon: DoorOpen,
        roles: ACCESS.manageRooms,
      },
      {
        title: 'Kelola Jadwal',
        url: '/admin/schedules',
        icon: CalendarClock,
        roles: ACCESS.manageSchedules,
      },
      {
        title: 'Kelola Pengguna',
        url: '/admin/users',
        icon: Users,
        roles: ACCESS.manageUsers,
      },
    ],
  },
  {
    title: 'Lainnya',
    items: [{ title: 'Tampilan', url: '/settings/appearance', icon: Palette }],
  },
]

/** Navigation for one role; groups left without items are dropped. */
export function navGroupsForRole(role: UserRole | undefined): NavGroup[] {
  if (!role) return []
  const user = { role }
  return navGroups
    .map((group) => ({
      ...group,
      items: group.items.filter(
        (item) => !item.roles || hasRole(user, item.roles)
      ),
    }))
    .filter((group) => group.items.length > 0)
}
