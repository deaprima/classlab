import { useState } from 'react'
import { attendanceRules } from '@/config/attendance-rules'
import { AlertCircle, XCircle } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type User, type Violation } from '@/lib/api-types'
import { formatDate, formatDateShort } from '@/lib/datetime'
import { ROLE_LABELS } from '@/lib/rbac'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { VIOLATION_LABELS } from '@/features/account/violation-labels'
import { ReasonDialog } from '@/features/bookings/components/reason-dialog'
import { useAnnulViolation, useUpdateUser, useUserViolations } from './api'

/** Violations of one requester: the points that count, each violation with Annul, and Buka Blokir. */
export function ViolationsDialog({
  user,
  onClose,
}: {
  user: User | null
  onClose: () => void
}) {
  return (
    <Dialog open={user !== null} onOpenChange={(o) => !o && onClose()}>
      {user && <ViolationsContent user={user} />}
    </Dialog>
  )
}

function ViolationsContent({ user }: { user: User }) {
  const violations = useUserViolations(user.id)
  const annul = useAnnulViolation(user.id)
  const unblock = useUpdateUser()
  const [annulling, setAnnulling] = useState<Violation | null>(null)
  const [unblocking, setUnblocking] = useState(false)
  const threshold = attendanceRules.blockThresholdPoints
  const data = violations.data
  const points = data?.active_points ?? 0
  const blockedUntil = data?.booking_blocked_until
  const blocked = !!blockedUntil && new Date(blockedUntil) > new Date()

  return (
    <DialogContent className='max-h-[90dvh] overflow-y-auto sm:max-w-lg'>
      <DialogHeader>
        <DialogTitle>Pelanggaran · {user.name}</DialogTitle>
        <DialogDescription>
          {ROLE_LABELS[user.role]} · {user.identifier}
        </DialogDescription>
      </DialogHeader>

      {violations.isError && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertDescription>
            {getErrorMessage(violations.error)}
          </AlertDescription>
        </Alert>
      )}
      {violations.isPending && <Skeleton className='h-40' />}

      {data && (
        <>
          {blocked && blockedUntil && (
            <Alert variant='error'>
              <XCircle />
              <AlertTitle>
                Diblokir sampai {formatDate(blockedUntil)}
              </AlertTitle>
              <AlertDescription>
                Poin aktif mencapai ambang {threshold}. Booking yang sudah
                disetujui tetap berlaku.
              </AlertDescription>
            </Alert>
          )}

          <div className='flex flex-col gap-2'>
            <div className='flex items-baseline justify-between gap-3'>
              <p className='type-card-title text-fg'>Poin pelanggaran aktif</p>
              <p className='type-card-title text-fg tabular-nums'>
                {points} / {threshold}
              </p>
            </div>
            <div
              role='progressbar'
              aria-label='Poin pelanggaran aktif'
              aria-valuemin={0}
              aria-valuemax={threshold}
              aria-valuenow={Math.min(points, threshold)}
              className='h-2 overflow-hidden rounded-full bg-muted'
            >
              <div
                className='h-full rounded-full bg-primary'
                style={{
                  width: `${Math.min(100, (points / threshold) * 100)}%`,
                }}
              />
            </div>
          </div>

          {data.data.length === 0 ? (
            <p className='type-body text-fg-secondary'>
              Belum ada pelanggaran.
            </p>
          ) : (
            <ul className='flex flex-col gap-3'>
              {data.data.map((v) => (
                <li
                  key={v.id}
                  className='flex items-start justify-between gap-3'
                >
                  <div className='min-w-0'>
                    <p className='type-card-title text-fg'>
                      {VIOLATION_LABELS[v.kind]}
                      {v.room_code && ` · ${v.room_code}`}
                    </p>
                    <p className='type-body-sm text-fg-secondary'>
                      {v.points} poin · {formatDateShort(v.created_at)}
                      {v.annulled_at && ` · di-annul: ${v.annul_reason ?? '-'}`}
                      {!v.annulled_at && !v.active && ' · sudah kedaluwarsa'}
                    </p>
                  </div>
                  {v.active && (
                    <Button
                      variant='secondary'
                      size='sm'
                      onClick={() => setAnnulling(v)}
                    >
                      Annul
                      <span className='sr-only'>
                        {' '}
                        {VIOLATION_LABELS[v.kind]}
                      </span>
                    </Button>
                  )}
                </li>
              ))}
            </ul>
          )}

          {blocked && (
            <div className='flex justify-end'>
              <Button variant='secondary' onClick={() => setUnblocking(true)}>
                Buka Blokir
              </Button>
            </div>
          )}
        </>
      )}

      <ReasonDialog
        open={annulling !== null}
        onOpenChange={(o) => !o && setAnnulling(null)}
        title='Annul pelanggaran?'
        description={
          <p>
            Poinnya tidak dihitung lagi. Blokir yang sudah berjalan tidak
            otomatis dicabut; gunakan Buka Blokir. Tercatat di audit log.
          </p>
        }
        label='Alasan annul'
        placeholder='Minimal 5 karakter. Contoh: kendala jaringan kampus.'
        confirmText='Annul'
        required
        isPending={annul.isPending}
        onSubmit={(reason) =>
          annulling &&
          annul.mutate(
            { id: annulling.id, reason },
            { onSuccess: () => setAnnulling(null) }
          )
        }
      />
      <ConfirmDialog
        open={unblocking}
        onOpenChange={(o) => !o && !unblock.isPending && setUnblocking(false)}
        title='Buka blokir booking?'
        desc={
          <p>
            {user.name} bisa mengajukan ruang lagi sekarang. Poin yang masih
            aktif tetap tercatat, jadi pelanggaran berikutnya bisa memblokir
            lagi.
          </p>
        }
        confirmText='Buka Blokir'
        isLoading={unblock.isPending}
        handleConfirm={() =>
          unblock.mutate(
            { id: user.id, body: { clear_booking_block: true } },
            {
              onSuccess: () => {
                setUnblocking(false)
                violations.refetch()
              },
            }
          )
        }
      />
    </DialogContent>
  )
}
