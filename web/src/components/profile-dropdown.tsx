import { Link } from '@tanstack/react-router'
import { LogOut, Palette } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE_LABELS } from '@/lib/rbac'
import { getDisplayNameInitials } from '@/lib/utils'
import useDialogState from '@/hooks/use-dialog-state'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { SignOutDialog } from '@/components/sign-out-dialog'

export function ProfileDropdown() {
  const [open, setOpen] = useDialogState()
  const user = useAuthStore((state) => state.auth.user)
  if (!user) return null

  return (
    <>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <button
            type='button'
            className='flex h-10 items-center gap-2 rounded-full border bg-surface ps-1 pe-1 outline-none hover:border-line-hover focus-visible:ring-2 focus-visible:ring-ring sm:pe-4'
            aria-label='Menu akun'
          >
            <Avatar className='size-8'>
              <AvatarFallback className='bg-primary type-label-sm text-primary-foreground'>
                {getDisplayNameInitials(user.name)}
              </AvatarFallback>
            </Avatar>
            <span className='hidden min-w-0 flex-col text-start sm:flex'>
              <span className='max-w-32 truncate type-label text-fg'>
                {user.name}
              </span>
              <span className='type-label-sm text-fg-secondary'>
                {ROLE_LABELS[user.role]}
              </span>
            </span>
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent className='w-56' align='end' forceMount>
          <DropdownMenuLabel className='font-normal'>
            <div className='flex flex-col gap-1.5'>
              <p className='text-sm leading-none font-medium'>{user.name}</p>
              <p className='text-xs leading-none text-muted-foreground'>
                {ROLE_LABELS[user.role]} · {user.identifier}
              </p>
            </div>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem asChild>
            <Link to='/settings/appearance'>
              <Palette />
              Tampilan
            </Link>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => setOpen(true)}>
            <LogOut />
            Keluar
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <SignOutDialog open={!!open} onOpenChange={setOpen} />
    </>
  )
}
