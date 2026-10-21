import { useSearch } from '@tanstack/react-router'
import { cn } from '@/lib/utils'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { SplitAuthLayout } from '../auth-layout'
import { UserAuthForm } from './components/user-auth-form'

export function SignIn() {
  const { redirect } = useSearch({ from: '/(auth)/sign-in' })

  return (
    <SplitAuthLayout aside={<NoAccount className='hidden lg:block' />}>
      <div className='flex flex-col items-center gap-1 text-center'>
        <h1 className='type-page-title text-fg'>Selamat datang kembali</h1>
        <p className='type-body text-fg-secondary'>
          Masuk dengan akun kampus kamu
        </p>
      </div>
      <UserAuthForm redirectTo={redirect} />
      {/* Narrow screens: under the button instead of crowding the header. */}
      <NoAccount align='center' className='self-center lg:hidden' />
    </SplitAuthLayout>
  )
}

/** "Belum punya akun?": opens the explanation that TU creates accounts, there is no self sign-up. */
function NoAccount({
  className,
  align = 'end',
}: {
  className?: string
  align?: 'center' | 'end'
}) {
  return (
    <Popover>
      <PopoverTrigger
        className={cn(
          'type-body-sm text-fg underline underline-offset-2',
          className
        )}
      >
        Belum punya akun?
      </PopoverTrigger>
      <PopoverContent align={align} className='text-sm'>
        Hubungi TU kampus: akun dibuat oleh TU dan tidak ada pendaftaran
        mandiri. Datangi TU dengan membawa NIM atau NIP kamu.
      </PopoverContent>
    </Popover>
  )
}
