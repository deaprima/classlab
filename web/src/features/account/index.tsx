import { Link } from '@tanstack/react-router'
import { attendanceRules } from '@/config/attendance-rules'
import { AlertCircle, BookOpen } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage } from '@/lib/api-error'
import { type User, type Violation } from '@/lib/api-types'
import { formatDate, formatDateShort } from '@/lib/datetime'
import { ACCESS, ROLE_LABELS, hasRole } from '@/lib/rbac'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { BentoCard } from '@/components/bento-card'
import { Page } from '@/components/layout/page'
import { useMe } from '@/features/auth/api'
import { useMyViolations } from './api'

const HISTORY_LIMIT = 5

const IDENTIFIER_LABELS: Record<User['role'], string> = {
  MAHASISWA: 'NIM',
  ORGANISASI: 'Kode organisasi',
  TU: 'NIP',
  KALAB: 'NIP',
  OB: 'NIP',
  ADMIN: 'NIP',
}

const VIOLATION_LABELS: Record<Violation['kind'], string> = {
  TIDAK_HADIR: 'Tidak hadir',
  TANPA_CHECKOUT: 'Tidak check-out',
  BATAL_TERLAMBAT: 'Pembatalan terlambat',
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className='flex items-baseline justify-between gap-4'>
      <dt className='type-body-lg text-fg-secondary'>{label}</dt>
      <dd className='min-w-0 text-end type-body-lg font-medium text-fg'>
        {value}
      </dd>
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

/** Akun Saya: profile for every role; violation points and their history for MAHASISWA and ORGANISASI. */
export function Account() {
  const session = useAuthStore((state) => state.auth.user)
  const me = useMe()
  const user = me.data ?? session
  const hasPoints = user?.active_points !== undefined
  const history = useMyViolations(
    hasPoints ? user?.id : undefined,
    HISTORY_LIMIT
  )
  if (!user) return null

  const threshold = attendanceRules.blockThresholdPoints
  const points = user.active_points ?? 0
  const percent = Math.min(100, Math.round((points / threshold) * 100))

  return (
    <Page
      title='Akun Saya'
      description='Profil dan poin pelanggaran.'
      actions={
        hasRole(user, ACCESS.createBooking) && (
          <Button variant='secondary' asChild>
            <Link to='/guide'>
              <BookOpen />
              Panduan Penggunaan
            </Link>
          </Button>
        )
      }
    >
      {me.isError && (
        <Alert variant='error' className='mb-5'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(me.error)}</AlertDescription>
        </Alert>
      )}

      <div className='grid items-start gap-5 lg:grid-cols-2'>
        <div className='flex flex-col gap-5'>
          <BentoCard title='Profil'>
            <dl className='flex flex-col gap-4'>
              <Row label='Nama' value={user.name} />
              <Row
                label={IDENTIFIER_LABELS[user.role]}
                value={user.identifier}
              />
              <Row label='Peran' value={ROLE_LABELS[user.role]} />
              {user.org_unit_name && (
                <Row label='Program studi' value={user.org_unit_name} />
              )}
              {user.email && <Row label='Email' value={user.email} />}
              {user.pic_name && (
                <Row label='Penanggung jawab' value={user.pic_name} />
              )}
              {user.pic_phone && (
                <Row label='Telepon penanggung jawab' value={user.pic_phone} />
              )}
              <Row
                label='Status akun'
                value={accountStatus(user, new Date())}
              />
            </dl>
          </BentoCard>

          <BentoCard title='Keamanan'>
            <p className='type-body text-fg-secondary'>
              Ganti password secara berkala dan jangan bagikan ke siapa pun.
            </p>
            <Button variant='secondary' className='self-start' asChild>
              <Link to='/change-password'>Ganti Password</Link>
            </Button>
          </BentoCard>
        </div>

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
                tetap berlaku.{' '}
                <Link
                  to='/guide'
                  hash='poin'
                  className='underline underline-offset-4'
                >
                  Pelajari aturan poin
                </Link>
              </p>

              <ViolationHistory
                items={history.data?.data}
                loading={history.isPending}
                total={history.data?.meta?.total ?? 0}
              />
            </BentoCard>
          ))}
      </div>
    </Page>
  )
}

function ViolationHistory({
  items,
  loading,
  total,
}: {
  items: Violation[] | undefined
  loading: boolean
  total: number
}) {
  if (loading) return <Skeleton className='h-24' />
  if (!items || items.length === 0) return null
  return (
    <section aria-label='Riwayat pelanggaran' className='flex flex-col gap-2'>
      <h3 className='type-overline text-fg-secondary'>Riwayat</h3>
      <ul className='flex flex-col gap-2'>
        {items.map((v) => (
          <li
            key={v.id}
            className={cn(
              'flex items-center justify-between gap-4 rounded-xl border px-4 py-3',
              !v.active && 'opacity-60'
            )}
          >
            <div className='min-w-0'>
              <p className='type-card-title text-fg'>
                {VIOLATION_LABELS[v.kind]}
              </p>
              <p className='type-body-sm text-fg-secondary'>
                {v.room_code && `${v.room_code} · `}
                {formatDateShort(v.booking_start_at ?? v.created_at)}
                {!v.active && ' · tidak dihitung'}
              </p>
            </div>
            <span className='shrink-0 type-label text-fg-secondary'>
              +{v.points}
            </span>
          </li>
        ))}
      </ul>
      {total > items.length && (
        <p className='type-body-sm text-fg-tertiary'>
          Menampilkan {items.length} dari {total} catatan terbaru.
        </p>
      )}
    </section>
  )
}
