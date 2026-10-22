import { Link } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { type ImportKind } from '@/lib/api-types'
import { Button } from '@/components/ui/button'
import { Page } from '@/components/layout/page'
import { ImportHistory } from './import-history'
import { ImportWizard } from './import-wizard'

const COPY: Record<
  ImportKind,
  { title: string; description: string; historyTitle: string }
> = {
  SCHEDULES: {
    title: 'Impor Jadwal Akademik',
    description:
      'Unggah jadwal kuliah satu semester dari CSV. Berkas diperiksa dulu sebelum diterapkan.',
    historyTitle: 'Riwayat Impor Jadwal',
  },
  USERS: {
    title: 'Impor Akun Mahasiswa',
    description:
      'Buat atau perbarui banyak akun mahasiswa dari CSV. Berkas diperiksa dulu sebelum diterapkan.',
    historyTitle: 'Riwayat Impor Akun',
  },
}

/** "Impor CSV" (Figma), one page per kind: the class schedule of a semester, or the student accounts. */
export function ImportPage({ kind }: { kind: ImportKind }) {
  const copy = COPY[kind]
  return (
    <Page
      title={copy.title}
      description={copy.description}
      actions={
        <Button variant='secondary' asChild>
          {kind === 'USERS' ? (
            <Link to='/manage/students'>
              <ArrowLeft />
              Kembali ke Mahasiswa
            </Link>
          ) : (
            <Link to='/admin/schedules'>
              <ArrowLeft />
              Kembali ke Jadwal
            </Link>
          )}
        </Button>
      }
    >
      <div className='flex flex-col gap-5'>
        <ImportWizard kind={kind} />
        <ImportHistory kind={kind} title={copy.historyTitle} />
      </div>
    </Page>
  )
}

export function ScheduleImportPage() {
  return <ImportPage kind='SCHEDULES' />
}

export function StudentImportPage() {
  return <ImportPage kind='USERS' />
}
