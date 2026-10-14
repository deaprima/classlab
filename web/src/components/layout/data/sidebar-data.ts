import {
  Bug,
  Command,
  FileX,
  Lock,
  LayoutDashboard,
  ListTodo,
  Monitor,
  Bell,
  Palette,
  ServerOff,
  Construction,
  Settings,
  UserCog,
  UserX,
  Users,
  Wrench,
} from 'lucide-react'
import { type SidebarData } from '../types'

// Placeholder navigation: only routes that exist after the template cleanup.
// Phase 5 replaces this with the role-based ClassLab menu.
export const sidebarData: SidebarData = {
  user: {
    name: 'Pengguna',
    email: '',
    avatar: '/avatars/shadcn.jpg',
  },
  teams: [
    {
      name: 'ClassLab.io',
      logo: Command,
      plan: 'Rumpun TI',
    },
  ],
  navGroups: [
    {
      title: 'Umum',
      items: [
        { title: 'Ketersediaan Ruang', url: '/', icon: LayoutDashboard },
        { title: 'Tugas (Referensi)', url: '/tasks', icon: ListTodo },
        { title: 'Pengguna (Referensi)', url: '/users', icon: Users },
      ],
    },
    {
      title: 'Halaman',
      items: [
        {
          title: 'Errors',
          icon: Bug,
          items: [
            { title: 'Unauthorized', url: '/errors/unauthorized', icon: Lock },
            { title: 'Forbidden', url: '/errors/forbidden', icon: UserX },
            { title: 'Not Found', url: '/errors/not-found', icon: FileX },
            {
              title: 'Internal Server Error',
              url: '/errors/internal-server-error',
              icon: ServerOff,
            },
            {
              title: 'Maintenance Error',
              url: '/errors/maintenance-error',
              icon: Construction,
            },
          ],
        },
      ],
    },
    {
      title: 'Lainnya',
      items: [
        {
          title: 'Pengaturan',
          icon: Settings,
          items: [
            { title: 'Pengaturan Profil', url: '/settings', icon: UserCog },
            {
              title: 'Pengaturan Akun',
              url: '/settings/account',
              icon: Wrench,
            },
            {
              title: 'Pengaturan Tampilan',
              url: '/settings/appearance',
              icon: Palette,
            },
            {
              title: 'Pengaturan Notifikasi',
              url: '/settings/notifications',
              icon: Bell,
            },
            {
              title: 'Pengaturan Display',
              url: '/settings/display',
              icon: Monitor,
            },
          ],
        },
      ],
    },
  ],
}
