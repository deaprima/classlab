import { useState } from 'react'
import { getRouteApi } from '@tanstack/react-router'
import { AlertCircle, KeyRound, Pencil, UserPlus } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getErrorMessage } from '@/lib/api-error'
import { type UserRole } from '@/lib/api-types'
import { ROLE_LABELS } from '@/lib/rbac'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Page } from '@/components/layout/page'
import { Pager } from '@/components/pager'
import { SearchInput } from '@/components/search-input'
import { useUsers } from './api'
import { UserDialog, type UserDialogState } from './user-dialogs'

const route = getRouteApi('/_authenticated/admin/users')
const ALL = 'ALL'

export function AdminUsers() {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const me = useAuthStore((s) => s.auth.user?.id)
  const [dialog, setDialog] = useState<UserDialogState>(null)
  const users = useUsers({
    page: search.page ?? 1,
    per_page: 20,
    q: search.q,
    role: search.role,
    is_active:
      search.status === undefined ? undefined : search.status === 'active',
  })

  const filter = (patch: Partial<typeof search>) =>
    navigate({ search: (prev) => ({ ...prev, ...patch, page: undefined }) })
  const rows = users.data?.data ?? []

  return (
    <Page
      title='Kelola Pengguna'
      description='Akun mahasiswa, organisasi, TU, Kepala Lab, OB, dan Admin.'
      actions={
        <Button onClick={() => setDialog({ mode: 'create' })}>
          <UserPlus />
          Tambah Pengguna
        </Button>
      }
    >
      <Card className='mb-6'>
        <CardContent className='grid gap-4 sm:grid-cols-3'>
          <SearchInput
            label='Cari pengguna'
            placeholder='Cari nama, NIM/NIP, atau email'
            value={search.q}
            onCommit={(q) => filter({ q })}
          />
          <Select
            value={search.role ?? ALL}
            onValueChange={(v) =>
              filter({ role: v === ALL ? undefined : (v as UserRole) })
            }
          >
            <SelectTrigger className='w-full' aria-label='Peran'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Semua peran</SelectItem>
              {Object.entries(ROLE_LABELS).map(([value, label]) => (
                <SelectItem key={value} value={value}>
                  {label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select
            value={search.status ?? ALL}
            onValueChange={(v) =>
              filter({
                status: v === ALL ? undefined : (v as 'active' | 'inactive'),
              })
            }
          >
            <SelectTrigger className='w-full' aria-label='Status akun'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>Semua status</SelectItem>
              <SelectItem value='active'>Aktif</SelectItem>
              <SelectItem value='inactive'>Nonaktif</SelectItem>
            </SelectContent>
          </Select>
        </CardContent>
      </Card>

      {users.isError && (
        <Alert variant='destructive' className='mb-4'>
          <AlertCircle />
          <AlertDescription>{getErrorMessage(users.error)}</AlertDescription>
        </Alert>
      )}

      <div className='overflow-x-auto rounded-xl border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Nama</TableHead>
              <TableHead>NIM / NIP</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Peran</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>
                <span className='sr-only'>Aksi</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {users.isPending &&
              Array.from({ length: 5 }, (_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={6}>
                    <Skeleton className='h-6' />
                  </TableCell>
                </TableRow>
              ))}
            {users.isSuccess && rows.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={6}
                  className='py-10 text-center text-muted-foreground'
                >
                  Tidak ada pengguna yang cocok.
                </TableCell>
              </TableRow>
            )}
            {rows.map((u) => (
              <TableRow key={u.id}>
                <TableCell className='font-medium'>
                  {u.name}
                  {u.id === me && (
                    <span className='ms-2 text-xs text-muted-foreground'>
                      (Anda)
                    </span>
                  )}
                </TableCell>
                <TableCell className='tabular-nums'>{u.identifier}</TableCell>
                <TableCell>{u.email ?? '-'}</TableCell>
                <TableCell>{ROLE_LABELS[u.role]}</TableCell>
                <TableCell>
                  {u.is_active ? (
                    <Badge variant='secondary'>Aktif</Badge>
                  ) : (
                    <Badge variant='outline'>Nonaktif</Badge>
                  )}
                </TableCell>
                <TableCell>
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
                      aria-label={`Reset kata sandi ${u.name}`}
                      onClick={() => setDialog({ mode: 'password', user: u })}
                    >
                      <KeyRound />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <Pager
        meta={users.data?.meta}
        onPageChange={(page) =>
          navigate({ search: (prev) => ({ ...prev, page }) })
        }
      />

      <UserDialog state={dialog} onClose={() => setDialog(null)} />
    </Page>
  )
}
