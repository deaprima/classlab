import { useSearch } from '@tanstack/react-router'
import { AuthLayout } from '../auth-layout'
import { UserAuthForm } from './components/user-auth-form'

export function SignIn() {
  const { redirect } = useSearch({ from: '/(auth)/sign-in' })

  return (
    <AuthLayout>
      <div className='flex flex-col gap-6 rounded-3xl border bg-card p-6 sm:p-8'>
        <div className='flex flex-col gap-1'>
          <h1 className='type-page-title text-fg'>Masuk ke ClassLab</h1>
          <p className='type-body text-fg-secondary'>
            Gunakan NIM atau NIP kamu. Akun dibuat oleh TU kampus.
          </p>
        </div>
        <UserAuthForm redirectTo={redirect} />
      </div>
    </AuthLayout>
  )
}
