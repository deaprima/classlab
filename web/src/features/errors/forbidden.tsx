import { useNavigate, useRouter } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'

export function ForbiddenError() {
  const navigate = useNavigate()
  const { history } = useRouter()
  return (
    <div className='flex min-h-svh items-center justify-center px-4'>
      <div className='flex w-full max-w-md flex-col items-center gap-3 rounded-3xl border bg-card p-8 text-center'>
        <p className='type-stat-xl text-fg-tertiary'>403</p>
        <h1 className='type-page-title text-fg'>Akses ditolak</h1>
        <p className='type-body text-fg-secondary'>
          Perannya tidak punya izin untuk membuka halaman ini. Kembali ke
          halaman sebelumnya atau ke beranda.
        </p>
        <div className='mt-4 flex w-full flex-col gap-3 sm:flex-row sm:justify-center'>
          <Button variant='secondary' onClick={() => history.go(-1)}>
            Kembali
          </Button>
          <Button onClick={() => navigate({ to: '/' })}>Ke Beranda</Button>
        </div>
      </div>
    </div>
  )
}
