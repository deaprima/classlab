import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  apiErrorBody,
  mockAdapter,
  sampleUser,
} from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { Imports } from '.'

const mocks = vi.hoisted(() => ({
  search: {} as Record<string, unknown>,
  navigate: vi.fn(),
}))

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    getRouteApi: () => ({
      useSearch: () => mocks.search,
      useNavigate: () => mocks.navigate,
    }),
  }
})
vi.mock('@/components/layout/page', () => ({
  Page: ({ title, children }: { title: string; children: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}))
const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
vi.mock('sonner', () => ({ toast }))

const originalAdapter = api.defaults.adapter

type Call = { method: string; url: string; params?: unknown; form: FormData }

const record = (over: Record<string, unknown> = {}) => ({
  id: 'imp-1',
  kind: 'SCHEDULES',
  uploaded_by: { id: 'u', identifier: 'tu', name: 'Pak Hendra' },
  file_name: 'jadwal-ganjil-2026.csv',
  semester: null,
  status: 'COMMITTED',
  summary: { occurrences: 1680, bookings_cancelled: 3 },
  created_at: '2026-10-20T03:00:00Z',
  rolled_back_at: null,
  ...over,
})

const SCHEDULE_PREVIEW = {
  rows: 120,
  occurrences: 1680,
  errors: [],
  bookings_to_cancel: [
    {
      booking_id: 'b1',
      room_code: 'R-301',
      start_at: '2026-11-03T03:00:00Z',
      end_at: '2026-11-03T05:00:00Z',
      conflicts_with: 'Kuliah Kalkulus',
    },
  ],
  plan_hash: 'hash-1',
}
const USERS_PREVIEW = {
  rows: 120,
  to_create: 112,
  to_update: 8,
  errors: [],
  plan_hash: 'hash-2',
}

function serve(handlers: { preview?: () => object; apply?: () => object }) {
  const calls: Call[] = []
  api.defaults.adapter = mockAdapter((config) => {
    const url = String(config.url)
    if (url === '/imports') {
      return {
        status: 200,
        data: {
          data: [record()],
          meta: { page: 1, per_page: 10, total: 1 },
        },
      }
    }
    if (url.endsWith('/rollback')) {
      calls.push({ method: 'post', url, form: new FormData() })
      return {
        status: 200,
        data: { id: 'imp-1', status: 'ROLLED_BACK', deleted_occurrences: 40 },
      }
    }
    if (url.startsWith('/imports/')) {
      calls.push({
        method: String(config.method),
        url,
        params: config.params,
        form: config.data as FormData,
      })
      const dry = (config.params as { dry_run?: boolean } | undefined)?.dry_run
      const make = dry ? handlers.preview : handlers.apply
      return make ? (make() as { status: number }) : { status: 404 }
    }
    return { status: 404 }
  })
  return calls
}

const csv = () =>
  new File(['identifier,name\n1,A'], 'akun.csv', { type: 'text/csv' })

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Imports />
    </QueryClientProvider>
  )
}

describe('Impor CSV', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.search = {}
    useAuthStore.getState().auth.setSession('t', { ...sampleUser, role: 'TU' })
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('previews a schedule file, lists the bookings it cancels, then applies it', async () => {
    const calls = serve({
      preview: () => ({ status: 200, data: SCHEDULE_PREVIEW }),
      apply: () => ({
        status: 200,
        data: {
          import_id: 'imp-2',
          rows: 120,
          occurrences: 1680,
          bookings_cancelled: 1,
        },
      }),
    })
    const screen = await renderPage()

    await userEvent.upload(screen.getByLabelText(/Tarik berkas CSV/), csv())

    await expect.element(screen.getByText('Langkah 2 dari 3')).toBeVisible()
    await expect
      .element(screen.getByText('R-301 · Kuliah Kalkulus'))
      .toBeVisible()
    // The first call only checks the file.
    expect(calls[0].url).toBe('/imports/schedules')
    expect(calls[0].params).toEqual({ dry_run: true })

    await userEvent.click(
      screen.getByRole('button', { name: 'Terapkan Impor' })
    )

    await expect.element(screen.getByText('Impor selesai')).toBeVisible()
    await expect
      .element(
        screen.getByText(/120 baris menjadi 1\.680 pertemuan; 1 booking/)
      )
      .toBeVisible()
    // The plan of the preview travels back with the file.
    expect(calls[1].params).toBeUndefined()
    expect(calls[1].form.get('plan_hash')).toBe('hash-1')
  })

  it('shows row errors and does not let an invalid file be applied', async () => {
    serve({
      preview: () => ({
        status: 200,
        data: {
          ...SCHEDULE_PREVIEW,
          bookings_to_cancel: [],
          errors: [
            {
              line: 14,
              column: 'room_code',
              message: 'Ruang "R-999" tidak dikenal',
            },
            {
              line: 87,
              column: 'start_time',
              message: 'Bentrok jadwal manual R-402',
            },
          ],
        },
      }),
    })
    const screen = await renderPage()

    await userEvent.upload(screen.getByLabelText(/Tarik berkas CSV/), csv())

    await expect.element(screen.getByText('2 baris tidak valid')).toBeVisible()
    await expect
      .element(screen.getByText('Bentrok jadwal manual R-402'))
      .toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Terapkan Impor' }))
      .toBeDisabled()
  })

  it('shows the lines of a 422 IMPORT_INVALID answer as row errors', async () => {
    serve({
      preview: () => ({
        status: 422,
        data: apiErrorBody('IMPORT_INVALID', 'Berkas tidak valid.', [
          { line: '3', column: 'day_of_week', message: 'Harus 1 sampai 7' },
        ]),
      }),
    })
    const screen = await renderPage()

    await userEvent.upload(screen.getByLabelText(/Tarik berkas CSV/), csv())

    await expect.element(screen.getByText('Harus 1 sampai 7')).toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Terapkan Impor' }))
      .toBeDisabled()
  })

  it('sends the semester with an account import and shows the one-time passwords', async () => {
    mocks.search = { tab: 'USERS' }
    const calls = serve({
      preview: () => ({ status: 200, data: USERS_PREVIEW }),
      apply: () => ({
        status: 200,
        data: {
          import_id: 'imp-3',
          created: 112,
          updated: 8,
          credentials: [
            { identifier: '2023010090', temporary_password: 'k7Tn4pQ2xW' },
          ],
        },
      }),
    })
    const screen = await renderPage()

    await userEvent.upload(screen.getByLabelText(/Tarik berkas CSV/), csv())
    await expect
      .element(screen.getByText('112 akun baru', { exact: false }).first())
      .toBeVisible()
    await userEvent.click(
      screen.getByRole('button', { name: 'Terapkan Impor' })
    )

    await expect
      .element(screen.getByText('Password sementara hanya tampil sekali'))
      .toBeVisible()
    await expect.element(screen.getByText('k7Tn4pQ2xW')).toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Unduh Kredensial (CSV)' }))
      .toBeVisible()
    expect(calls[0].form.get('semester')).toMatch(/^\d{4}-(GANJIL|GENAP)$/)
    expect(calls[1].form.get('plan_hash')).toBe('hash-2')
  })

  it('tells the user to check again when the data changed since the preview', async () => {
    serve({
      preview: () => ({ status: 200, data: SCHEDULE_PREVIEW }),
      apply: () => ({
        status: 409,
        data: apiErrorBody('IMPORT_PLAN_CHANGED', 'Rencana berubah.'),
      }),
    })
    const screen = await renderPage()

    await userEvent.upload(screen.getByLabelText(/Tarik berkas CSV/), csv())
    await userEvent.click(
      screen.getByRole('button', { name: 'Terapkan Impor' })
    )

    await expect
      .element(screen.getByText(/Data berubah sejak pratinjau/))
      .toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Periksa ulang' }))
      .toBeVisible()
  })

  it('refuses a file that is not a CSV before sending anything', async () => {
    const calls = serve({})
    const screen = await renderPage()

    await userEvent.upload(
      screen.getByLabelText(/Tarik berkas CSV/),
      new File(['x'], 'jadwal.xlsx', { type: 'application/vnd.ms-excel.x' })
    )

    await expect
      .element(screen.getByText('Berkas harus berformat CSV (.csv).'))
      .toBeVisible()
    expect(calls).toEqual([])
  })

  it('lists the history and rolls a schedule import back after confirming', async () => {
    const calls = serve({})
    const screen = await renderPage()

    await expect
      .element(screen.getByText('jadwal-ganjil-2026.csv').first())
      .toBeVisible()
    await userEvent.click(
      screen.getByRole('button', { name: /Rollback/ }).first()
    )
    await userEvent.click(
      screen.getByRole('alertdialog').getByRole('button', { name: 'Rollback' })
    )

    await vi.waitFor(() =>
      expect(calls.some((c) => c.url === '/imports/imp-1/rollback')).toBe(true)
    )
    await vi.waitFor(() => expect(toast.success).toHaveBeenCalled())
  })
})
