import { ConfirmDialog } from '@/components/confirm-dialog'
import { useSignOut } from '@/features/auth/use-sign-out'

interface SignOutDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SignOutDialog({ open, onOpenChange }: SignOutDialogProps) {
  const signOut = useSignOut()

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title='Keluar'
      desc='Anda yakin ingin keluar? Anda perlu masuk kembali untuk menggunakan ClassLab.'
      confirmText='Keluar'
      destructive
      isLoading={signOut.isPending}
      handleConfirm={() => signOut.mutate()}
      className='sm:max-w-sm'
    />
  )
}
