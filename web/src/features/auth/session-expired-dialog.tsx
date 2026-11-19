import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { useSessionExpiredStore } from '@/stores/session-expired-store'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

/**
 * Shown when a request got 401 and the refresh failed too. It cannot be dismissed: the only way on
 * is to sign in again. Local state is dropped only then, so the page behind does not refetch into
 * more 401s while the dialog is open.
 */
export function SessionExpiredDialog() {
  const { open, redirect, close } = useSessionExpiredStore()
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  function signInAgain() {
    useAuthStore.getState().auth.reset()
    queryClient.clear()
    close()
    navigate({ to: '/sign-in', search: { redirect }, replace: true })
  }

  return (
    <AlertDialog open={open}>
      <AlertDialogContent
        className='sm:max-w-sm'
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <AlertDialogHeader>
          <AlertDialogTitle>Sesi berakhir</AlertDialogTitle>
          <AlertDialogDescription>
            Demi keamanan, kamu perlu masuk lagi. Halaman yang sedang dibuka
            akan dibuka kembali setelah masuk.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogAction onClick={signInAgain}>
            Masuk Lagi
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
