import { useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from '@tanstack/react-router'
import { Loader2, LogIn } from 'lucide-react'
import { toast } from 'sonner'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'
import { sleep, cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/password-input'

const formSchema = z.object({
  identifier: z.string().min(1, 'NIM atau NIP wajib diisi.'),
  password: z.string().min(1, 'Kata sandi wajib diisi.'),
})

interface UserAuthFormProps extends React.HTMLAttributes<HTMLFormElement> {
  redirectTo?: string
}

export function UserAuthForm({
  className,
  redirectTo,
  ...props
}: UserAuthFormProps) {
  const [isLoading, setIsLoading] = useState(false)
  const navigate = useNavigate()
  const { auth } = useAuthStore()

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      identifier: '',
      password: '',
    },
  })

  function onSubmit(data: z.infer<typeof formSchema>) {
    setIsLoading(true)

    toast.promise(sleep(1000), {
      loading: 'Memverifikasi kredensial...',
      success: () => {
        setIsLoading(false)

        const mockUser: AuthUser = {
          id: '018f23a4-8f0b-789a-bcde-0123456789ab',
          org_unit_id: '018f23a4-8f0b-789a-bcde-000000000001',
          identifier: data.identifier,
          name: 'PJ Kelas TI',
          email: `${data.identifier}@student.univ.ac.id`,
          role: 'MAHASISWA',
          is_active: true,
        }

        auth.setUser(mockUser)
        auth.setAccessToken('mock_access_token_dev')

        const targetPath = redirectTo || '/'
        navigate({ to: targetPath, replace: true })

        return `Selamat datang kembali, ${mockUser.name}!`
      },
      error: () => {
        setIsLoading(false)
        return 'Gagal masuk. Periksa NIM/NIP dan kata sandi.'
      },
    })
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', className)}
        {...props}
      >
        <FormField
          control={form.control}
          name='identifier'
          render={({ field }) => (
            <FormItem>
              <FormLabel>NIM / NIP</FormLabel>
              <FormControl>
                <Input
                  placeholder='Contoh: 220101001 atau 19850101...'
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem>
              <FormLabel>Kata Sandi</FormLabel>
              <FormControl>
                <PasswordInput placeholder='Masukkan kata sandi' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button className='mt-2' disabled={isLoading}>
          {isLoading ? (
            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
          ) : (
            <LogIn className='mr-2 h-4 w-4' />
          )}
          Masuk
        </Button>
      </form>
    </Form>
  )
}
