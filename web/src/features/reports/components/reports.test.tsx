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
import { ReportDetailSheet } from './report-detail-sheet'
import { ReportFormDialog } from './report-form-dialog'

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
vi.mock('sonner', () => ({ toast }))

const originalAdapter = api.defaults.adapter
const ROOM = {
  id: '0b6a3a8e-1d2c-4c7e-9f0a-1234567890ab',
  org_unit_id: sampleUser.org_unit_id,
  code: 'LAB-1',
  name: 'Lab Jaringan',
  type: 'LAB',
  capacity: 30,
  facilities: [],
  is_active: true,
}
const REPORT = {
  id: '22222222-2222-4222-8222-222222222222',
  room_id: ROOM.id,
  room: ROOM,
  reporter_id: sampleUser.id,
  reporter: { id: sampleUser.id, identifier: '2021101001', name: 'Andi' },
  description: 'Proyektor mati',
  status: 'BARU',
  created_at: '2026-10-14T09:00:00+07:00',
  events: [
    {
      id: 'e1',
      report_id: '22222222-2222-4222-8222-222222222222',
      actor_id: sampleUser.id,
      to_status: 'BARU',
      created_at: '2026-10-14T09:00:00+07:00',
    },
  ],
}

function wrap(ui: React.ReactNode) {
  return render(
    <QueryClientProvider client={new QueryClient()}>{ui}</QueryClientProvider>
  )
}

const roomsReply = {
  status: 200,
  data: { data: [ROOM], meta: { page: 1, per_page: 100, total: 1 } },
}

describe('ReportFormDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore.getState().auth.setSession('t', { ...sampleUser })
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  async function fill(screen: Awaited<ReturnType<typeof wrap>>) {
    await userEvent.click(screen.getByRole('combobox'))
    await userEvent.click(screen.getByRole('option', { name: /LAB-1/ }))
    await userEvent.fill(
      screen.getByLabelText('Kerusakan', { exact: true }),
      'Proyektor mati'
    )
    await userEvent.upload(
      screen.getByLabelText('Foto (opsional)'),
      new File([new Uint8Array([0xff, 0xd8, 0xff])], 'foto.jpg', {
        type: 'image/jpeg',
      })
    )
  }

  it('uploads the report as multipart form data', async () => {
    let sent: FormData | undefined
    api.defaults.adapter = mockAdapter((config) => {
      if (config.url === '/rooms') return roomsReply
      sent = config.data as FormData
      return { status: 201, data: { ...REPORT, photo_path: '/x' } }
    })
    const onCreated = vi.fn()
    const screen = await wrap(
      <ReportFormDialog open onOpenChange={vi.fn()} onCreated={onCreated} />
    )

    await fill(screen)
    await expect
      .element(screen.getByAltText('Pratinjau foto kerusakan'))
      .toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Kirim Laporan' }))

    await vi.waitFor(() => expect(onCreated).toHaveBeenCalled())
    expect(sent).toBeInstanceOf(FormData)
    expect(sent?.get('room_id')).toBe(ROOM.id)
    expect(sent?.get('description')).toBe('Proyektor mati')
    expect((sent?.get('photo') as File).name).toBe('foto.jpg')
  })

  it('shows the server photo error on the photo field', async () => {
    api.defaults.adapter = mockAdapter((config) =>
      config.url === '/rooms'
        ? roomsReply
        : {
            status: 422,
            data: apiErrorBody('VALIDATION_ERROR', 'Data tidak valid.', [
              {
                field: 'photo',
                message: 'Foto rusak atau tidak dapat dibaca.',
              },
            ]),
          }
    )
    const screen = await wrap(
      <ReportFormDialog open onOpenChange={vi.fn()} onCreated={vi.fn()} />
    )

    await fill(screen)
    await userEvent.click(screen.getByRole('button', { name: 'Kirim Laporan' }))

    await expect
      .element(screen.getByText('Foto rusak atau tidak dapat dibaca.'))
      .toBeInTheDocument()
  })
})

describe('ReportDetailSheet status action', () => {
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('lets TU/Kalab move the report one step with a note', async () => {
    useAuthStore
      .getState()
      .auth.setSession('t', { ...sampleUser, id: 'tu', role: 'TU' })
    const patches: unknown[] = []
    api.defaults.adapter = mockAdapter((config) => {
      if (config.method === 'patch') {
        patches.push({ url: config.url, body: JSON.parse(String(config.data)) })
        return { status: 200, data: { ...REPORT, status: 'DITERIMA' } }
      }
      return { status: 200, data: REPORT }
    })
    const screen = await wrap(
      <ReportDetailSheet reportId={REPORT.id} onClose={vi.fn()} />
    )

    await userEvent.click(
      screen.getByRole('button', { name: 'Tandai Diterima' })
    )
    await userEvent.fill(
      screen.getByLabelText('Catatan untuk pelapor (opsional)'),
      'Teknisi dijadwalkan besok.'
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Tandai Diterima' }).last()
    )

    await vi.waitFor(() =>
      expect(patches).toEqual([
        {
          url: `/reports/${REPORT.id}/status`,
          body: { status: 'DITERIMA', note: 'Teknisi dijadwalkan besok.' },
        },
      ])
    )
  })

  it('offers no status change to the reporter', async () => {
    useAuthStore.getState().auth.setSession('t', { ...sampleUser })
    api.defaults.adapter = mockAdapter(() => ({ status: 200, data: REPORT }))
    const screen = await wrap(
      <ReportDetailSheet reportId={REPORT.id} onClose={vi.fn()} />
    )

    await expect.element(screen.getByText('Riwayat status')).toBeInTheDocument()
    await expect
      .element(screen.getByRole('button', { name: /Tandai/ }))
      .not.toBeInTheDocument()
  })
})
