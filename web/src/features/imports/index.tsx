import { getRouteApi } from '@tanstack/react-router'
import { type ImportKind } from '@/lib/api-types'
import { Page } from '@/components/layout/page'
import { SegmentedControl } from '@/components/segmented-control'
import { ImportHistory } from './import-history'
import { ImportWizard } from './import-wizard'

const route = getRouteApi('/_authenticated/manage/imports')

/** "Impor CSV" (Figma): the class schedule of a semester or the student accounts, in three steps. */
export function Imports() {
  const { tab } = route.useSearch()
  const navigate = route.useNavigate()
  const kind: ImportKind = tab ?? 'SCHEDULES'

  return (
    <Page
      title='Impor CSV'
      description='Unggah jadwal akademik atau akun mahasiswa. Berkas diperiksa dulu sebelum diterapkan.'
    >
      <div className='flex flex-col gap-5'>
        <SegmentedControl<ImportKind>
          aria-label='Jenis impor'
          value={kind}
          onValueChange={(k) =>
            navigate({
              search: { tab: k === 'SCHEDULES' ? undefined : k },
              replace: true,
            })
          }
          options={[
            { value: 'SCHEDULES', label: 'Jadwal Akademik' },
            { value: 'USERS', label: 'Akun Mahasiswa' },
          ]}
        />
        {/* Keyed by kind: switching the tab starts that import afresh and drops any shown passwords. */}
        <ImportWizard key={kind} kind={kind} />
        <ImportHistory />
      </div>
    </Page>
  )
}
