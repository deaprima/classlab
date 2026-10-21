import { Link } from '@tanstack/react-router'
import { attendanceRules } from '@/config/attendance-rules'
import { AlertCircle } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage } from '@/lib/api-error'
import { type User } from '@/lib/api-types'
import { formatDate } from '@/lib/datetime'
import { ROLE_LABELS } from '@/lib/rbac'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { BentoCard } from '@/components/bento-card'
import { Page } from '@/components/layout/page'
import { useMe } from '@/features/auth/api'

const IDENTIFIER_LABELS: Record<User['role'], string> = {
  MAHASISWA: 'NIM',
  ORGANISASI: 'Kode organisasi',
  TU: 'NIP',
  KALAB: 'NIP',
  OB: 'NIP',
  ADMIN: 'NIP',
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className='flex items-start justify-between gap-4'>
      <dt className='type-body text-fg-secondary'>{label}</dt>
      <dd className='min-w-0 text-end type-label text-fg'>{value}</dd>
    </div>
  )
}

/** Status line of the account, in words: active, blocked from booking until a date, or disabled. */
function accountStatus(user: User, now: Date): string {
  if (!user.is_active) return 'Nonaktif'
  const until = user.booking_blocked_until
  if (until && new Date(until) > now) {
    return `Diblokir mengajukan sampai ${formatDate(until)}`
  }
  return 'Aktif'
}

/** Akun Saya: profile for every role; violation points for MAHASISWA and ORGANISASI. */
export function Account() {
  const session = useAuthStore((state) => state.auth.user)
  const me = useMe()
  const user = me.data ?? session
  if (!user) return null

  const hasPoints = user.active_points !== undefined
  const threshold = attendanceRules.blockThresholdPoints
  const points = user.active_points ?? 0
  const percent = Math.min(100, Math.round((points / threshold) * 100))

  return (
    <Page title='Akun Saya' description='Profil dan poin pelanggaran.'>
      {me.isError && (
        <Alert variant='error' className='mb-5'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(me.error)}</AlertDescription>
        </Alert>
      )}

      <div className='grid gap-5 lg:grid-cols-2'>
        <BentoCard title='Profil'>
          <dl className='flex flex-col gap-3'>
            <Row label='Nama' value={user.name} />
            <Row label={IDENTIFIER_LABELS[user.role]} value={user.identifier} />
            <Row label='Peran' value={ROLE_LABELS[user.role]} />
            {user.email && <Row label='Email' value={user.email} />}
            {user.pic_name && (
              <Row label='Penanggung jawab' value={user.pic_name} />
            )}
            {user.pic_phone && (
              <Row label='Telepon penanggung jawab' value={user.pic_phone} />
            )}
            <Row label='Status akun' value={accountStatus(user, new Date())} />
          </dl>
          <Button variant='secondary' className='self-start' asChild>
            <Link to='/change-password'>Ganti Password</Link>
          </Button>
        </BentoCard>

        {hasPoints &&
          (me.isPending ? (
            <Skeleton className='h-60 rounded-2xl' />
          ) : (
            <BentoCard title='Poin Pelanggaran'>
              <p className='type-stat-l text-fg'>
                {points}{' '}
                <span className='type-body text-fg-secondary'>
                  dari {threshold} poin
                </span>
              </p>
              <div
                role='progressbar'
                aria-label='Poin pelanggaran'
                aria-valuemin={0}
                aria-valuemax={threshold}
                aria-valuenow={Math.min(points, threshold)}
                className='h-2 overflow-hidden rounded-full bg-muted'
              >
                <div
                  className='h-full rounded-full bg-primary'
                  style={{ width: `${percent}%` }}
                />
              </div>
              <p className='type-body text-fg-secondary'>
                Mencapai {threshold} poin: akun diblokir mengajukan ruang selama{' '}
                {attendanceRules.blockDays} hari. Booking yang sudah disetujui
                tetap berlaku.
              </p>
            </BentoCard>
          ))}
      </div>
    </Page>
  )
}
