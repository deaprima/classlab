import { useState } from 'react'
import { Link, getRouteApi } from '@tanstack/react-router'
import {
  AlertCircle,
  FileUp,
  KeyRound,
  Pencil,
  UserPlus,
  Users,
} from 'lucide-react'
import { getErrorMessage } from '@/lib/api-error'
import { type User } from '@/lib/api-types'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/layout/page'
import { ListFooter } from '@/components/list-footer'
import {
  type DataColumn,
  ResponsiveDataList,
} from '@/components/responsive-data-list'
import { SearchInput } from '@/components/search-input'
import { SegmentedControl } from '@/components/segmented-control'
import { StatusPill } from '@/components/status-pill'
import { useUsers } from '@/features/admin/users/api'
import {
  type UserDialogState,
  UserDialog,
} from '@/features/admin/users/user-dialogs'

const route = getRouteApi('/_authenticated/manage/students/')
const PER_PAGE = 20

type Status = 'all' | 'active' | 'inactive'

/** "Mahasiswa" (Figma): the student accounts TU manages, one by one here or in bulk by CSV. */
export function Students() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const [dialog, setDialog] = useState<UserDialogState>(null)
  const status: Status = search.status ?? 'all'
  const students = useUsers({
    role: 'MAHASISWA',
    page: search.page ?? 1,
    per_page: PER_PAGE,
    q: search.q,
    is_active: status === 'all' ? undefined : status === 'active',
  })
  // The total of all students, whatever the filter, for the count beside the title.
  const everyone = useUsers({ role: 'MAHASISWA', per_page: 1 })
  const rows = students.data?.data ?? []

  const refine = (patch: Partial<typeof search>) =>
    navigate({ search: (prev) => ({ ...prev, page: undefined, ...patch }) })

  const columns: DataColumn<User>[] = [
    {
      key: 'name',
      header: 'Nama',
      primary: true,
      cell: (u) => <span className='type-card-title text-fg'>{u.name}</span>,
    },
    {
      key: 'identifier',
      header: 'NIM',
      cell: (u) => (
        <span className='text-fg-secondary tabular-nums'>{u.identifier}</span>
      ),
    },
    {
      key: 'email',
      header: 'Email',
      cell: (u) => <span className='text-fg-secondary'>{u.email ?? '–'}</span>,
    },
    {
      key: 'status',
      header: 'Status',
      cell: (u) =>
        u.is_active ? (
          <StatusPill status='Tersedia' label='Aktif' />
        ) : (
          <StatusPill status='Terbatas' label='Nonaktif' />
        ),
    },
    {
      key: 'actions',
      header: 'Aksi',
      actions: true,
      cell: (u) => (
        <div className='flex justify-end gap-1'>
          <Button
            variant='ghost'
            size='icon'
            aria-label={`Ubah ${u.name}`}
            onClick={() => setDialog({ mode: 'edit', user: u })}
          >
            <Pencil />
          </Button>
          <Button
            variant='ghost'
            size='icon'
            aria-label={`Reset password ${u.name}`}
            onClick={() => setDialog({ mode: 'password', user: u })}
          >
            <KeyRound />
          </Button>
        </div>
      ),
    },
  ]

  return (
    <Page
      title='Mahasiswa'
      description='Akun mahasiswa: tambah satu per satu, impor dari CSV, ubah data, atau reset password.'
      actions={
        <>
          <Button variant='secondary' asChild>
            <Link to='/manage/students/import'>
              <FileUp />
              Impor CSV
            </Link>
          </Button>
          <Button onClick={() => setDialog({ mode: 'create' })}>
            <UserPlus />
            Tambah Mahasiswa
          </Button>
        </>
      }
    >
      <section
        aria-label='Daftar Mahasiswa'
        className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
      >
        <h2 className='type-card-title text-fg'>Daftar Mahasiswa</h2>
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <SegmentedControl<Status>
            aria-label='Status akun'
            value={status}
            onValueChange={(v) =>
              refine({ status: v === 'all' ? undefined : v })
            }
            options={[
              {
                value: 'all',
                label: 'Semua',
                count: everyone.data?.meta.total,
              },
              { value: 'active', label: 'Aktif' },
              { value: 'inactive', label: 'Nonaktif' },
            ]}
          />
          <div className='min-w-48 flex-1 sm:max-w-72'>
            <SearchInput
              label='Cari mahasiswa'
              placeholder='Cari nama, NIM, atau email'
              value={search.q}
              onCommit={(q) => refine({ q })}
            />
          </div>
        </div>

        {students.isError && (
          <Alert variant='error'>
            <AlertCircle />
            <AlertDescription>
              {getErrorMessage(students.error)}
            </AlertDescription>
          </Alert>
        )}

        {students.isPending ? (
          <Skeleton className='h-64' />
        ) : rows.length === 0 ? (
          <EmptyState
            icon={Users}
            title={
              search.q || search.status
                ? 'Tidak ada mahasiswa yang cocok'
                : 'Belum ada akun mahasiswa'
            }
            description={
              search.q || search.status
                ? 'Ubah kata kunci atau filter.'
                : 'Tambahkan satu akun, atau impor banyak akun dari CSV.'
            }
          />
        ) : (
          <>
            <ResponsiveDataList
              caption='Daftar mahasiswa'
              rows={rows}
              columns={columns}
              rowKey={(u) => u.id}
              onRowClick={(u) => setDialog({ mode: 'edit', user: u })}
            />
            <ListFooter
              page={students.data?.meta.page ?? 1}
              perPage={PER_PAGE}
              total={students.data?.meta.total ?? 0}
              onPageChange={(page) =>
                navigate({ search: (prev) => ({ ...prev, page }) })
              }
            />
          </>
        )}
      </section>

      <UserDialog state={dialog} onClose={() => setDialog(null)} studentsOnly />
    </Page>
  )
}
