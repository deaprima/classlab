import {
  Bell,
  CalendarClock,
  CalendarDays,
  ClipboardCheck,
  DoorOpen,
  FileClock,
  History,
  LayoutGrid,
  ScrollText,
  ShieldCheck,
  SlidersHorizontal,
  TriangleAlert,
  Users,
} from 'lucide-react'
import { type UserRole } from '@/lib/api-types'
import { type NavItem } from '../types'

// The sidebar menu of each role, in order (docs/DESIGN.md section 5; at most 6 items). An item
// whose page does not exist yet has no `url`: it is part of the design but stays out of the UI
// until its route lands (Phase 6.2/6.3), so no link ever leads nowhere.
type MenuItem = Omit<NavItem, 'items' | 'roles'> & { url?: string }

const REQUESTER_MENU: MenuItem[] = [
  { title: 'Dashboard', url: '/', icon: LayoutGrid },
  { title: 'Jadwal Ruang', url: '/schedule', icon: CalendarDays },
  { title: 'Pengajuan Saya', url: '/bookings', icon: FileClock },
  { title: 'Riwayat', url: '/history', icon: History },
  { title: 'Lapor Kerusakan', url: '/reports', icon: TriangleAlert },
]

const MENUS: Record<UserRole, MenuItem[]> = {
  MAHASISWA: REQUESTER_MENU,
  ORGANISASI: REQUESTER_MENU,
  TU: [
    { title: 'Beranda', url: '/', icon: LayoutGrid },
    { title: 'Approval', url: '/approvals', icon: ClipboardCheck },
    { title: 'Ruang & Jadwal', url: '/admin/rooms', icon: DoorOpen },
    { title: 'Verifikasi', url: '/verify', icon: ShieldCheck },
    { title: 'Laporan', url: '/reports', icon: TriangleAlert },
    { title: 'Mahasiswa', icon: Users },
  ],
  KALAB: [
    { title: 'Beranda', url: '/', icon: LayoutGrid },
    { title: 'Approval Lab', url: '/approvals', icon: ClipboardCheck },
    { title: 'Ruang & Jadwal', url: '/admin/rooms', icon: DoorOpen },
    { title: 'Papan Ruang', url: '/today', icon: CalendarClock },
    { title: 'Laporan', url: '/reports', icon: TriangleAlert },
  ],
  OB: [
    { title: 'Beranda', url: '/', icon: LayoutGrid },
    { title: 'Papan Ruang', url: '/today', icon: CalendarClock },
    { title: 'Verifikasi', url: '/verify', icon: ShieldCheck },
    { title: 'Laporan', url: '/reports', icon: TriangleAlert },
    { title: 'Notifikasi', url: '/notifications', icon: Bell },
  ],
  ADMIN: [
    { title: 'Beranda', url: '/', icon: LayoutGrid },
    { title: 'Approval', url: '/approvals', icon: ClipboardCheck },
    { title: 'Ruang & Jadwal', url: '/admin/rooms', icon: DoorOpen },
    { title: 'Pengguna', url: '/admin/users', icon: Users },
    { title: 'Kebijakan Ruang', icon: SlidersHorizontal },
    { title: 'Audit Log', icon: ScrollText },
  ],
}

/** The full menu of a role as designed, including items whose page is not built yet. */
export function plannedMenuForRole(role: UserRole): readonly MenuItem[] {
  return MENUS[role]
}

/** Navigation for one role: only items that already have a page. */
export function navGroupsForRole(
  role: UserRole | undefined
): { title: string; items: NavItem[] }[] {
  if (!role) return []
  const items = MENUS[role].filter(
    (item): item is MenuItem & { url: string } => !!item.url
  )
  return [{ title: 'Menu', items }]
}
