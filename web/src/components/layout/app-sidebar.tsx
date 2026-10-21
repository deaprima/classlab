import { useMemo } from 'react'
import { Link } from '@tanstack/react-router'
import { LogOut, SlidersHorizontal } from 'lucide-react'
import { Logo } from '@/assets/logo'
import { useAuthStore } from '@/stores/auth-store'
import useDialogState from '@/hooks/use-dialog-state'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from '@/components/ui/sidebar'
import { SignOutDialog } from '@/components/sign-out-dialog'
import { ThemeToggle } from '@/components/theme-toggle'
import { useUnreadCount } from '@/features/notifications/api'
import { navGroupsForRole } from './data/sidebar-data'
import { NavGroup } from './nav-group'

/** DESIGN.md 3.1: a fixed 240 px sidebar with MENU and AKUN sections; a drawer on mobile. */
export function AppSidebar() {
  const role = useAuthStore((state) => state.auth.user?.role)
  const unread = useUnreadCount().data ?? 0
  const menu = useMemo(
    () => ({
      title: 'Menu',
      items: navGroupsForRole(role)
        .flatMap((group) => group.items)
        .map((item) =>
          item.url === '/notifications' && unread > 0
            ? { ...item, badge: unread > 99 ? '99+' : String(unread) }
            : item
        ),
    }),
    [role, unread]
  )

  return (
    <Sidebar collapsible='offcanvas' variant='sidebar'>
      <SidebarHeader className='px-4 pt-6 pb-2'>
        <AppBrand />
      </SidebarHeader>
      <SidebarContent className='gap-4 px-2'>
        <NavGroup {...menu} />
        <AccountGroup />
      </SidebarContent>
      <SidebarFooter className='px-4 pb-6'>
        <ThemeToggle />
      </SidebarFooter>
    </Sidebar>
  )
}

function AppBrand() {
  const { setOpenMobile } = useSidebar()
  return (
    <Link
      to='/'
      onClick={() => setOpenMobile(false)}
      className='flex items-center gap-2 rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring'
    >
      <span className='flex size-9 items-center justify-center rounded-full bg-sidebar-primary text-sidebar-primary-foreground'>
        <Logo className='size-5' />
      </span>
      <span className='type-section text-sidebar-foreground'>ClassLab.io</span>
    </Link>
  )
}

function AccountGroup() {
  const { setOpenMobile } = useSidebar()
  const [open, setOpen] = useDialogState()
  return (
    <SidebarGroup>
      <SidebarGroupLabel>Akun</SidebarGroupLabel>
      <SidebarMenu>
        <SidebarMenuItem>
          <SidebarMenuButton asChild>
            <Link
              to='/account'
              activeProps={{ 'data-active': true }}
              onClick={() => setOpenMobile(false)}
            >
              <SlidersHorizontal />
              <span>Akun Saya</span>
            </Link>
          </SidebarMenuButton>
        </SidebarMenuItem>
        <SidebarMenuItem>
          <SidebarMenuButton onClick={() => setOpen(true)}>
            <LogOut />
            <span>Keluar</span>
          </SidebarMenuButton>
        </SidebarMenuItem>
      </SidebarMenu>
      <SignOutDialog open={!!open} onOpenChange={setOpen} />
    </SidebarGroup>
  )
}
