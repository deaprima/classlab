import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useRouter } from '@tanstack/react-router'
import { AlertCircle, Loader2, LogIn } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { applyFieldErrors, getApiError, getErrorMessage } from '@/lib/api-error'
import { type LoginRequest, type LoginResponse } from '@/lib/api-types'
import { CHANGE_PASSWORD_PATH, safeRedirectPath } from '@/lib/session'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
  identifier: z.string().trim().min(1, 'NIM atau NIP wajib diisi.'),
  password: z.string().min(1, 'Kata sandi wajib diisi.'),
})

type FormValues = z.infer<typeof formSchema>

interface UserAuthFormProps extends React.HTMLAttributes<HTMLFormElement> {
  redirectTo?: string
}

export function UserAuthForm({
  className,
  redirectTo,
  ...props
}: UserAuthFormProps) {
  const router = useRouter()

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: { identifier: '', password: '' },
  })

  const login = useMutation({
    mutationFn: async (body: LoginRequest) =>
      (await api.post<LoginResponse>('/auth/login', body)).data,
    onSuccess: (data) => {
      useAuthStore.getState().auth.setSession(data.access_token, data.user)
      const target = safeRedirectPath(redirectTo)
      // A temporary password must be replaced before anything else; the change page returns to `target`.
      router.history.replace(
        data.user.must_change_password
          ? `${CHANGE_PASSWORD_PATH}?redirect=${encodeURIComponent(target)}`
          : target
      )
    },
    onError: (error) => {
      const details = getApiError(error)?.details ?? []
      const unmatched = applyFieldErrors(error, form.setError, {
        identifier: 'identifier',
        password: 'password',
      })
      // Wrong credentials (401) and rate limiting (429) carry no field; show the server's message.
      if (unmatched.length > 0 || unmatched.length === details.length) {
        form.setError('root', { message: getErrorMessage(error) })
      }
      form.resetField('password', { keepError: true })
    },
  })

  function onSubmit(data: FormValues) {
    form.clearErrors('root')
    login.mutate(data)
  }

  const rootError = form.formState.errors.root?.message

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', className)}
        noValidate
        {...props}
      >
        {rootError && (
          <Alert variant='error'>
            <AlertCircle />
            <AlertDescription>{rootError}</AlertDescription>
          </Alert>
        )}
        <FormField
          control={form.control}
          name='identifier'
          render={({ field }) => (
            <FormItem>
              <FormLabel>NIM / NIP</FormLabel>
              <FormControl>
                <Input
                  placeholder='Contoh: 2021101001'
                  autoComplete='username'
                  autoFocus
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
                <PasswordInput
                  placeholder='Masukkan kata sandi'
                  autoComplete='current-password'
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button className='mt-2' disabled={login.isPending}>
          {login.isPending ? <Loader2 className='animate-spin' /> : <LogIn />}
          Masuk
        </Button>
      </form>
    </Form>
  )
}
