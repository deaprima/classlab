import { type LinkProps } from '@tanstack/react-router'
import { type UserRole } from '@/lib/api-types'

type BaseNavItem = {
  title: string
  badge?: string
  icon?: React.ElementType
  /** Roles that see this item; omitted means every signed-in user. */
  roles?: readonly UserRole[]
}

type NavLink = BaseNavItem & {
  url: LinkProps['to'] | (string & {})
  items?: never
}

type NavCollapsible = BaseNavItem & {
  items: (BaseNavItem & { url: LinkProps['to'] | (string & {}) })[]
  url?: never
}

type NavItem = NavCollapsible | NavLink

type NavGroup = {
  title: string
  items: NavItem[]
}

export type { NavGroup, NavItem, NavCollapsible, NavLink }
