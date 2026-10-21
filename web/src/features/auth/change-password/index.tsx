import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useRouter } from '@tanstack/react-router'
import { AlertCircle, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { applyFieldErrors, getApiError, getErrorMessage } from '@/lib/api-error'
import {
  changePasswordSchema,
  MIN_PASSWORD_LENGTH,
  type ChangePasswordValues,
} from '@/lib/password-rules'
import { safeRedirectPath } from '@/lib/session'
import useDialogState from '@/hooks/use-dialog-state'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { PasswordInput } from '@/components/password-input'
import { SignOutDialog } from '@/components/sign-out-dialog'
import { AuthLayout } from '../auth-layout'

type ChangePasswordProps = {
  /** Page to open after the change; defaults to the beranda. */
  redirectTo?: string
}

/**
 * The only page an account with a temporary password may use (the API forbids everything else),
 * but any signed-in user can open it to change their password.
 */
export function ChangePassword({ redirectTo }: ChangePasswordProps) {
  const router = useRouter()
  const forced = useAuthStore((state) => state.auth.user?.must_change_password)
  const [signOutOpen, setSignOutOpen] = useDialogState()

  const form = useForm<ChangePasswordValues>({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: {
      current_password: '',
      new_password: '',
      confirm_password: '',
    },
  })

  const change = useMutation({
    mutationFn: async (v: ChangePasswordValues) =>
      api.post('/auth/change-password', {
        current_password: v.current_password,
        new_password: v.new_password,
      }),
    onSuccess: () => {
      const { auth } = useAuthStore.getState()
      if (auth.user) auth.setUser({ ...auth.user, must_change_password: false })
      toast.success('Kata sandi diganti.')
      router.history.replace(safeRedirectPath(redirectTo))
    },
    onError: (error) => {
      const details = getApiError(error)?.details ?? []
      const unmatched = applyFieldErrors(error, form.setError, {
        current_password: 'current_password',
        new_password: 'new_password',
      })
      // A wrong current password is a 422 on its field; 429 and the like carry no field.
      if (unmatched.length > 0 || details.length === 0) {
        form.setError('root', { message: getErrorMessage(error) })
      }
    },
  })

  const rootError = form.formState.errors.root?.message

  return (
    <AuthLayout>
      <div className='flex flex-col gap-6 rounded-3xl border bg-card p-6 sm:p-8'>
        <div className='flex flex-col gap-1'>
          <h1 className='type-page-title text-fg'>Ganti kata sandi</h1>
          <p className='type-body text-fg-secondary'>
            {forced
              ? 'Akunmu masih memakai kata sandi sementara. Ganti dulu sebelum memakai ClassLab.'
              : 'Sesi di perangkat lain akan keluar setelah kata sandi diganti.'}
          </p>
        </div>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit((v) => {
              form.clearErrors('root')
              change.mutate(v)
            })}
            className='grid gap-4'
            noValidate
          >
            {rootError && (
              <Alert variant='error'>
                <AlertCircle />
                <AlertDescription>{rootError}</AlertDescription>
              </Alert>
            )}
            <FormField
              control={form.control}
              name='current_password'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Kata sandi saat ini</FormLabel>
                  <FormControl>
                    <PasswordInput
                      autoComplete='current-password'
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
              name='new_password'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Kata sandi baru</FormLabel>
                  <FormControl>
                    <PasswordInput autoComplete='new-password' {...field} />
                  </FormControl>
                  <FormDescription>
                    Minimal {MIN_PASSWORD_LENGTH} karakter.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='confirm_password'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Ulangi kata sandi baru</FormLabel>
                  <FormControl>
                    <PasswordInput autoComplete='new-password' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <Button className='mt-2' disabled={change.isPending}>
              {change.isPending && <Loader2 className='animate-spin' />}
              Simpan
            </Button>
          </form>
        </Form>
        <Button
          type='button'
          variant='ghost'
          onClick={() => setSignOutOpen(true)}
        >
          Keluar
        </Button>
      </div>
      <SignOutDialog open={!!signOutOpen} onOpenChange={setSignOutOpen} />
    </AuthLayout>
  )
}
