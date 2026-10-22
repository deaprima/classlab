import { useState } from 'react'
import { AlertCircle, Pencil } from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type RoomTypePolicy } from '@/lib/api-types'
import { ROLE_LABELS } from '@/lib/rbac'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Page } from '@/components/layout/page'
import {
  type DataColumn,
  ResponsiveDataList,
} from '@/components/responsive-data-list'
import { StatusPill } from '@/components/status-pill'
import { useRoomTypes } from '@/features/room-types/api'
import { PolicyFormDialog } from './policy-form-dialog'

/** "Kebijakan Tipe Ruang" (Figma): who approves, the default hours and how each room type is approved. */
export function Policies() {
  const policies = useRoomTypes()
  const [editing, setEditing] = useState<RoomTypePolicy | null>(null)

  const columns: DataColumn<RoomTypePolicy>[] = [
    {
      key: 'type',
      header: 'Tipe',
      primary: true,
      cell: (p) => <span className='type-card-title text-fg'>{p.label}</span>,
    },
    {
      key: 'approvers',
      header: 'Approver',
      cell: (p) => (
        <span className='text-fg-secondary'>
          {p.approver_roles.map((r) => ROLE_LABELS[r]).join(', ')}
        </span>
      ),
    },
    {
      key: 'hours',
      header: 'Jam Default',
      cell: (p) => (
        <span className='text-fg-secondary tabular-nums'>
          {p.open_time}–{p.close_time}
        </span>
      ),
    },
    {
      key: 'approval',
      header: 'Persetujuan',
      cell: (p) =>
        p.approval_policy === 'AUTO_IF_ELIGIBLE' ? (
          <StatusPill status='Tersedia' label='Otomatis' />
        ) : (
          <StatusPill status='Terbatas' label='Manual' />
        ),
    },
    {
      key: 'actions',
      header: 'Aksi',
      actions: true,
      cell: (p) => (
        <Button
          variant='ghost'
          size='icon'
          aria-label={`Ubah kebijakan ${p.label}`}
          onClick={() => setEditing(p)}
        >
          <Pencil />
        </Button>
      ),
    },
  ]

  return (
    <Page
      title='Kebijakan Tipe Ruang'
      description='Aturan bawaan tiap tipe ruang: siapa yang menyetujui dan mengajukan, jam, durasi, dan surat.'
    >
      <section
        aria-label='Tipe Ruang'
        className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
      >
        <h2 className='type-card-title text-fg'>Tipe Ruang</h2>
        {policies.isError && (
          <Alert variant='error'>
            <AlertCircle />
            <AlertDescription>
              {getErrorMessage(policies.error)}
            </AlertDescription>
          </Alert>
        )}
        {policies.isPending ? (
          <Skeleton className='h-64' />
        ) : (
          <ResponsiveDataList
            caption='Kebijakan tipe ruang'
            rows={policies.data ?? []}
            columns={columns}
            rowKey={(p) => p.type}
            onRowClick={setEditing}
          />
        )}
        <p className='type-body-sm text-fg-secondary'>
          Otomatis = disetujui sistem bila ruang kosong dan riwayat pemohon
          baik. Perubahan tercatat di audit log dan berlaku untuk pengajuan
          berikutnya.
        </p>
      </section>
      <PolicyFormDialog policy={editing} onClose={() => setEditing(null)} />
    </Page>
  )
}
