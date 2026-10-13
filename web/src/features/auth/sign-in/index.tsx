import { useSearch } from '@tanstack/react-router'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { AuthLayout } from '../auth-layout'
import { UserAuthForm } from './components/user-auth-form'

export function SignIn() {
  const { redirect } = useSearch({ from: '/(auth)/sign-in' })

  return (
    <AuthLayout>
      <Card className='max-w-sm gap-4'>
        <CardHeader>
          <CardTitle className='text-lg tracking-tight'>Masuk ke ClassLab</CardTitle>
          <CardDescription>
            Gunakan NIM atau NIP yang telah terdaftar oleh administrator program studi.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <UserAuthForm redirectTo={redirect} />
        </CardContent>
        <CardFooter>
          <p className='px-4 text-center text-xs text-muted-foreground'>
            Sistem Peminjaman Ruang Kelas dan Laboratorium Kampus (MVP Rumpun TI).
          </p>
        </CardFooter>
      </Card>
    </AuthLayout>
  )
}
