import { useState } from 'react'
import {
  AlertCircle,
  AlertTriangle,
  CheckCircle2,
  Download,
  Loader2,
  XCircle,
} from 'lucide-react'
import { getApiError, getErrorMessage } from '@/lib/api-error'
import {
  type ImportKind,
  type ImportRowError,
  type ImportUsersPreview,
  type ImportUsersResult,
  type ScheduleApplyResult,
  type SchedulePreview,
} from '@/lib/api-types'
import { formatDateShort, formatTimeRange } from '@/lib/datetime'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Dropzone } from '@/components/dropzone'
import { StatusPill } from '@/components/status-pill'
import {
  type ImportPreview,
  type ImportResult,
  useApplyImport,
  usePreviewImport,
} from './api'
import {
  type Term,
  checkCsvFile,
  credentialsCsv,
  defaultSemester,
  downloadText,
  rowErrorsFromDetails,
  semesterValue,
} from './rules'

const CREDENTIALS_SHOWN = 10

type Stage =
  | { step: 1 }
  | {
      step: 2
      file: File
      semester?: string
      preview: ImportPreview | null
      errors: ImportRowError[]
    }
  | { step: 3; result: ImportResult; semester?: string }

const isUsersPreview = (p: ImportPreview): p is ImportUsersPreview =>
  'to_create' in p
const isUsersResult = (r: ImportResult): r is ImportUsersResult =>
  'credentials' in r

/** The three steps of one import: upload the CSV, read the preview, apply and read the result. */
export function ImportWizard({ kind }: { kind: ImportKind }) {
  const [stage, setStage] = useState<Stage>({ step: 1 })
  const [fileError, setFileError] = useState<string | null>(null)
  const [problem, setProblem] = useState<string | null>(null)
  const initial = defaultSemester(new Date())
  const [year, setYear] = useState(String(initial.year))
  const [term, setTerm] = useState<Term>(initial.term)
  const preview = usePreviewImport()
  const apply = useApplyImport()

  const yearOk = /^\d{4}$/.test(year)
  const semester =
    kind === 'USERS' && yearOk ? semesterValue(Number(year), term) : undefined

  const runPreview = (file: File, forSemester: string | undefined) => {
    setProblem(null)
    preview.mutate(
      { kind, file, semester: forSemester },
      {
        onSuccess: (p) =>
          setStage({
            step: 2,
            file,
            semester: forSemester,
            preview: p,
            errors: p.errors,
          }),
        onError: (error) => {
          const api = getApiError(error)
          // Content problems come as 422 IMPORT_INVALID with a line and column per detail.
          if (api?.code === 'IMPORT_INVALID') {
            setStage({
              step: 2,
              file,
              semester: forSemester,
              preview: null,
              errors: rowErrorsFromDetails(api.details),
            })
            return
          }
          setProblem(getErrorMessage(error))
        },
      }
    )
  }

  const pick = (files: File[]) => {
    const file = files[0]
    if (!file) return
    const error = checkCsvFile(file)
    setFileError(error)
    if (error) return
    if (kind === 'USERS' && !semester) {
      setFileError('Isi tahun semester dengan 4 angka, mis. 2026.')
      return
    }
    runPreview(file, semester)
  }

  const reset = () => {
    setStage({ step: 1 })
    setProblem(null)
    setFileError(null)
  }

  if (stage.step === 3) {
    return (
      <Result
        kind={kind}
        result={stage.result}
        semester={stage.semester}
        onAgain={reset}
      />
    )
  }

  if (stage.step === 2) {
    const { file, preview: p, errors } = stage
    const invalid = errors.length > 0 || !p
    const confirm = () => {
      if (!p) return
      setProblem(null)
      apply.mutate(
        { kind, file, semester: stage.semester, planHash: p.plan_hash },
        {
          onSuccess: (result) =>
            setStage({ step: 3, result, semester: stage.semester }),
          onError: (error) => {
            const code = getApiError(error)?.code
            setProblem(
              code === 'IMPORT_PLAN_CHANGED'
                ? 'Data berubah sejak pratinjau. Periksa ulang berkas sebelum menerapkan.'
                : getErrorMessage(error)
            )
          },
        }
      )
    }

    return (
      <div className='grid grid-cols-1 items-start gap-5 lg:grid-cols-[minmax(0,1fr)_340px]'>
        <section
          aria-label='Pratinjau impor'
          className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
        >
          <h2 className='type-card-title text-fg'>
            Pratinjau Impor · Langkah 2 dari 3
          </h2>
          <div className='flex flex-wrap items-center justify-between gap-3 rounded-2xl border p-4'>
            <div className='min-w-0'>
              <p className='truncate type-card-title text-fg'>{file.name}</p>
              <p className='type-body-sm text-fg-secondary'>
                {summaryLine(p, stage.semester)}
              </p>
            </div>
            <Button variant='secondary' size='sm' onClick={reset}>
              Ganti File
            </Button>
          </div>

          {invalid ? (
            <Alert variant='error'>
              <XCircle />
              <AlertTitle>{errors.length} baris tidak valid</AlertTitle>
              <AlertDescription>
                Semua-atau-tidak-sama-sekali: perbaiki file lalu unggah ulang.
                Tidak ada yang tersimpan.
              </AlertDescription>
            </Alert>
          ) : (
            <Alert>
              <CheckCircle2 />
              <AlertTitle>Berkas valid</AlertTitle>
              <AlertDescription>
                Periksa ringkasannya, lalu terapkan impor.
              </AlertDescription>
            </Alert>
          )}
          {problem && (
            <Alert variant='error'>
              <AlertCircle />
              <AlertDescription className='flex flex-wrap items-center gap-3'>
                {problem}
                <Button
                  size='sm'
                  variant='secondary'
                  disabled={preview.isPending}
                  onClick={() => runPreview(file, stage.semester)}
                >
                  Periksa ulang
                </Button>
              </AlertDescription>
            </Alert>
          )}

          {errors.length > 0 && <ErrorTable errors={errors} />}
        </section>

        <aside
          aria-label='Ringkasan penerapan'
          className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
        >
          {p && <Consequences preview={p} />}
          <Button
            disabled={invalid || apply.isPending}
            onClick={confirm}
            className='h-10'
          >
            {apply.isPending && <Loader2 className='animate-spin' />}
            Terapkan Impor
          </Button>
          <Button
            variant='secondary'
            className='h-10'
            disabled={apply.isPending}
            onClick={reset}
          >
            Unggah Ulang
          </Button>
        </aside>
      </div>
    )
  }

  return (
    <section
      aria-label='Unggah berkas'
      className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
    >
      <h2 className='type-card-title text-fg'>Unggah CSV · Langkah 1 dari 3</h2>
      {kind === 'USERS' && (
        <div className='grid max-w-sm grid-cols-2 gap-3'>
          <div className='grid gap-2'>
            <Label htmlFor='import-year'>Tahun</Label>
            <Input
              id='import-year'
              inputMode='numeric'
              maxLength={4}
              value={year}
              onChange={(e) => setYear(e.target.value)}
            />
          </div>
          <div className='grid gap-2'>
            <Label htmlFor='import-term'>Semester</Label>
            <Select value={term} onValueChange={(v) => setTerm(v as Term)}>
              <SelectTrigger id='import-term' className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='GANJIL'>Ganjil</SelectItem>
                <SelectItem value='GENAP'>Genap</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      )}
      <Dropzone
        accept='.csv,text/csv'
        onFiles={pick}
        disabled={preview.isPending}
        title={
          preview.isPending
            ? 'Memeriksa berkas…'
            : 'Tarik berkas CSV ke sini atau pilih dari perangkat'
        }
        hint={columnsHint(kind)}
      />
      {fileError && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertDescription>{fileError}</AlertDescription>
        </Alert>
      )}
      {problem && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertDescription>{problem}</AlertDescription>
        </Alert>
      )}
      <p className='type-body-sm text-fg-secondary'>
        Berkas hanya diperiksa dulu; belum ada yang tersimpan sampai kamu
        menerapkannya. Pemisah koma atau titik koma diterima, maksimal 2 MB.
      </p>
    </section>
  )
}

function columnsHint(kind: ImportKind): string {
  return kind === 'USERS'
    ? 'Kolom wajib: identifier, name. Opsional: email, course_code, course_name, class_name.'
    : 'Kolom wajib: room_code, title, day_of_week, start_time, end_time, semester_start, semester_end. Opsional: skip_dates.'
}

function summaryLine(p: ImportPreview | null, semester?: string): string {
  if (!p) return 'Berkas tidak dapat diimpor.'
  const sem = semester ? ` · semester ${semester}` : ''
  return isUsersPreview(p)
    ? `${p.rows} baris · ${p.to_create} akun baru · ${p.to_update} diperbarui${sem}`
    : `${p.rows} baris · ${p.occurrences.toLocaleString('id-ID')} kejadian${sem}`
}

function ErrorTable({ errors }: { errors: ImportRowError[] }) {
  return (
    <div className='overflow-x-auto rounded-xl border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Baris</TableHead>
            <TableHead>Kolom</TableHead>
            <TableHead>Pesan</TableHead>
            <TableHead>Jenis</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {errors.map((e, i) => (
            <TableRow key={`${e.line}-${e.column ?? ''}-${i}`}>
              <TableCell className='font-medium tabular-nums'>
                {e.line || '–'}
              </TableCell>
              <TableCell className='text-fg-secondary'>
                {e.column ?? '–'}
              </TableCell>
              <TableCell className='whitespace-normal'>{e.message}</TableCell>
              <TableCell>
                <StatusPill status='Ditolak' label='Error' />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

/** What applying will do: the bookings a schedule import cancels, or the accounts it creates and updates. */
function Consequences({ preview }: { preview: ImportPreview }) {
  if (isUsersPreview(preview)) {
    return (
      <>
        <h2 className='type-card-title text-fg'>Yang Akan Terjadi</h2>
        <dl className='flex flex-col gap-2 type-body'>
          <div className='flex justify-between gap-4'>
            <dt className='text-fg-secondary'>Akun baru</dt>
            <dd className='text-fg tabular-nums'>{preview.to_create}</dd>
          </div>
          <div className='flex justify-between gap-4'>
            <dt className='text-fg-secondary'>Akun diperbarui</dt>
            <dd className='text-fg tabular-nums'>{preview.to_update}</dd>
          </div>
        </dl>
        <p className='type-body-sm text-fg-secondary'>
          Password sementara akun baru tampil sekali setelah impor diterapkan.
        </p>
      </>
    )
  }
  return <ScheduleConsequences preview={preview} />
}

function ScheduleConsequences({ preview }: { preview: SchedulePreview }) {
  const list = preview.bookings_to_cancel
  return (
    <>
      <h2 className='type-card-title text-fg'>Booking yang Dibatalkan</h2>
      <p className='type-body-sm text-fg-secondary'>
        Jadwal akademik selalu menang. Booking yang bentrok dibatalkan otomatis
        dan pemohon diberi notifikasi.
      </p>
      {list.length === 0 ? (
        <p className='type-body text-fg'>Tidak ada booking yang bentrok.</p>
      ) : (
        <ul className='flex max-h-80 flex-col gap-3 overflow-y-auto'>
          {list.map((b) => (
            <li key={b.booking_id}>
              <p className='type-card-title text-fg'>
                {b.room_code} · {b.conflicts_with}
              </p>
              <p className='type-body-sm text-fg-secondary'>
                {formatDateShort(b.start_at)} ·{' '}
                {formatTimeRange(b.start_at, b.end_at)}
              </p>
            </li>
          ))}
        </ul>
      )}
    </>
  )
}

function Result({
  kind,
  result,
  semester,
  onAgain,
}: {
  kind: ImportKind
  result: ImportResult
  semester?: string
  onAgain: () => void
}) {
  return (
    <section
      aria-label='Hasil impor'
      className='flex min-w-0 flex-col gap-4 rounded-2xl border bg-card p-5'
    >
      <h2 className='type-card-title text-fg'>
        Hasil Impor · Langkah 3 dari 3
      </h2>
      <Alert>
        <CheckCircle2 />
        <AlertTitle>Impor selesai</AlertTitle>
        <AlertDescription>
          {isUsersResult(result)
            ? `${result.created} akun dibuat, ${result.updated} akun diperbarui.${semester ? ` Semester ${semester}.` : ''}`
            : scheduleResultLine(result as ScheduleApplyResult)}
        </AlertDescription>
      </Alert>
      {isUsersResult(result) && <Credentials result={result} />}
      {!isUsersResult(result) && (
        <p className='type-body-sm text-fg-secondary'>
          Salah impor? Buka Riwayat Impor di bawah dan pilih Rollback untuk
          menghapus pertemuan yang belum dimulai.
        </p>
      )}
      <div>
        <Button variant='secondary' onClick={onAgain}>
          Impor {kind === 'USERS' ? 'Akun' : 'Jadwal'} Lagi
        </Button>
      </div>
    </section>
  )
}

function scheduleResultLine(r: ScheduleApplyResult): string {
  return `${r.rows} baris menjadi ${r.occurrences.toLocaleString('id-ID')} pertemuan; ${r.bookings_cancelled} booking dibatalkan.`
}

/** The one-time temporary passwords: shown here and saved as a CSV built in the browser, never stored. */
function Credentials({ result }: { result: ImportUsersResult }) {
  const rows = result.credentials
  if (rows.length === 0) {
    return (
      <p className='type-body text-fg-secondary'>
        Tidak ada akun baru, jadi tidak ada password sementara.
      </p>
    )
  }
  return (
    <>
      <Alert variant='warning'>
        <AlertTriangle />
        <AlertTitle>Password sementara hanya tampil sekali</AlertTitle>
        <AlertDescription>
          Unduh sekarang. Password tidak disimpan; bila hilang, gunakan Reset
          Password pada akun mahasiswa.
        </AlertDescription>
      </Alert>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <h3 className='type-card-title text-fg'>
          Kredensial awal ({Math.min(rows.length, CREDENTIALS_SHOWN)} dari{' '}
          {rows.length})
        </h3>
        <Button
          variant='accent'
          onClick={() =>
            downloadText('kredensial-akun.csv', credentialsCsv(rows))
          }
        >
          <Download />
          Unduh Kredensial (CSV)
        </Button>
      </div>
      <div className='overflow-x-auto rounded-xl border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>NIM</TableHead>
              <TableHead>Password Sementara</TableHead>
              <TableHead>Hasil</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.slice(0, CREDENTIALS_SHOWN).map((c) => (
              <TableRow key={c.identifier}>
                <TableCell className='font-medium'>{c.identifier}</TableCell>
                <TableCell className='font-mono'>
                  {c.temporary_password}
                </TableCell>
                <TableCell>
                  <StatusPill status='Tersedia' label='Baru' />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </>
  )
}
