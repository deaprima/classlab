import { type InternalAxiosRequestConfig } from 'axios'
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
import { type Room } from '@/lib/api-types'
import { addDays, todayWib } from '@/lib/datetime'
import { RequestForm } from './request-form'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    Link: ({ children }: { children: React.ReactNode }) => (
      <a href='/'>{children}</a>
    ),
  }
})

const base = {
  org_unit_id: sampleUser.org_unit_id,
  facilities: [],
  is_active: true,
  open_time: null,
  close_time: null,
}
const KELAS_ROOM = {
  ...base,
  id: 'room-kelas',
  code: 'R-302',
  name: 'Ruang 302',
  type: 'KELAS',
  capacity: 50,
}
const LAB_ROOM = {
  ...base,
  id: 'room-lab',
  code: 'LAB-1',
  name: 'Lab Jaringan',
  type: 'LAB',
  capacity: 30,
}
const AUD_ROOM = {
  ...base,
  id: 'room-aud',
  code: 'Auditorium',
  name: 'Auditorium',
  type: 'AUDITORIUM',
  capacity: 200,
}

const policy = (over: Record<string, unknown>) => ({
  label: 'x',
  approver_roles: ['TU'],
  requester_roles: ['MAHASISWA', 'ORGANISASI'],
  approval_policy: 'AUTO_IF_ELIGIBLE',
  letter_required: false,
  open_time: '07:00',
  close_time: '21:00',
  min_duration_min: 30,
  max_duration_min: 480,
  max_advance_days: 30,
  updated_at: '2026-10-01T00:00:00Z',
  ...over,
})
const POLICIES = [
  policy({ type: 'KELAS', label: 'Ruang Kelas' }),
  policy({
    type: 'LAB',
    label: 'Laboratorium',
    approver_roles: ['KALAB'],
    approval_policy: 'ALWAYS_MANUAL',
  }),
  policy({
    type: 'AUDITORIUM',
    label: 'Auditorium',
    approver_roles: ['TU', 'ADMIN'],
    requester_roles: ['ORGANISASI'],
    approval_policy: 'ALWAYS_MANUAL',
    letter_required: true,
    min_duration_min: 60,
  }),
]

const tomorrow = addDays(todayWib(), 1)
const dayAfter = addDays(todayWib(), 2)
const originalAdapter = api.defaults.adapter

type Reply = { status: number; data?: unknown }
type Handlers = {
  booking?: (config: InternalAxiosRequestConfig) => Reply
  availability?: 'TERSEDIA' | 'DIGUNAKAN'
  me?: Record<string, unknown>
  upload?: (config: InternalAxiosRequestConfig) => Reply
}

function serve(h: Handlers = {}) {
  const rooms = [KELAS_ROOM, LAB_ROOM, AUD_ROOM]
  api.defaults.adapter = mockAdapter((config) => {
    if (config.url === '/room-types')
      return { status: 200, data: { data: POLICIES } }
    if (config.url === '/rooms') {
      return {
        status: 200,
        data: { data: rooms, meta: { page: 1, per_page: 100, total: 3 } },
      }
    }
    if (config.url === '/auth/me') {
      return {
        status: 200,
        data: { ...useAuthStore.getState().auth.user, ...h.me },
      }
    }
    if (config.url === '/availability') {
      const status = h.availability ?? 'TERSEDIA'
      return {
        status: 200,
        data: {
          data: rooms.map((room) => ({
            room,
            operating_hours: {
              open_time: '07:00',
              close_time: '21:00',
              custom: false,
            },
            status,
            conflicts:
              status === 'TERSEDIA'
                ? []
                : [
                    {
                      kind: 'SCHEDULE',
                      title: 'Kuliah Kalkulus',
                      start_at: `${tomorrow}T08:30:00+07:00`,
                      end_at: `${tomorrow}T10:00:00+07:00`,
                    },
                  ],
          })),
        },
      }
    }
    if (config.url === '/bookings' && config.method === 'post') {
      return h.booking ? h.booking(config) : { status: 500 }
    }
    if (config.url === '/attachments' && config.method === 'post') {
      return h.upload ? h.upload(config) : { status: 500 }
    }
    if (config.url === '/attachments') {
      return {
        status: 200,
        data: { data: [], meta: { page: 1, per_page: 20, total: 0 } },
      }
    }
    return { status: 404 }
  })
}

function signIn(role: 'MAHASISWA' | 'ORGANISASI' = 'MAHASISWA') {
  useAuthStore.getState().auth.setSession('token', { ...sampleUser, role })
}

const SLOT = { date: tomorrow, start_time: '08:00', end_time: '10:00' }

async function renderForm(
  props: Partial<React.ComponentProps<typeof RequestForm>> = {}
) {
  const onCreated = vi.fn()
  const onCancel = vi.fn()
  const screen = await render(
    <QueryClientProvider client={new QueryClient()}>
      <RequestForm
        room={KELAS_ROOM as Room}
        initialOccurrences={[SLOT]}
        onCancel={onCancel}
        {...props}
        onCreated={onCreated}
      />
    </QueryClientProvider>
  )
  return { screen, onCreated, onCancel }
}

const submitButton = (
  screen: Awaited<ReturnType<typeof renderForm>>['screen']
) => screen.getByRole('button', { name: 'Kirim Pengajuan' })

describe('RequestForm', () => {
  beforeEach(() => {
    signIn()
  })

  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  describe('policy preview', () => {
    it('promises automatic approval for a free classroom', async () => {
      serve()
      const { screen } = await renderForm()
      await expect
        .element(screen.getByText('Akan disetujui otomatis'))
        .toBeVisible()
      await expect.element(screen.getByText('2 jam')).toBeVisible()
    })

    it('names the approver for a lab', async () => {
      serve()
      const { screen } = await renderForm({ room: LAB_ROOM as Room })
      await expect
        .element(
          screen.getByText('Menunggu validasi Kepala Lab', { exact: true })
        )
        .toBeVisible()
    })

    it('warns about a taken slot and disables sending', async () => {
      serve({ availability: 'DIGUNAKAN' })
      const { screen } = await renderForm()
      await expect.element(screen.getByText('Slot tidak kosong')).toBeVisible()
      await expect.element(screen.getByText(/Kuliah Kalkulus/)).toBeVisible()
      await expect.element(submitButton(screen)).toBeDisabled()
    })
  })

  describe('blocked and quota', () => {
    it('shows the block banner and disables sending', async () => {
      serve({ me: { booking_blocked_until: '2099-11-02T08:00:00+07:00' } })
      const { screen } = await renderForm()
      await expect
        .element(screen.getByText(/Booking baru diblokir sampai/))
        .toBeVisible()
      await expect.element(submitButton(screen)).toBeDisabled()
    })

    it('shows the block banner when the server refuses with BOOKING_BLOCKED', async () => {
      serve({
        booking: () => ({
          status: 403,
          data: apiErrorBody('BOOKING_BLOCKED', 'Akun diblokir.', [
            { field: 'blocked_until', message: '2099-11-02T08:00:00+07:00' },
          ]),
        }),
      })
      const { screen } = await renderForm()
      await userEvent.fill(
        screen.getByLabelText('Nama kegiatan'),
        'Kuliah pengganti'
      )
      await userEvent.click(submitButton(screen))
      await expect
        .element(screen.getByText(/Booking baru diblokir sampai/))
        .toBeVisible()
    })

    it('explains a full quota', async () => {
      serve({
        booking: () => ({
          status: 409,
          data: apiErrorBody(
            'QUOTA_EXCEEDED',
            'Kuota booking aktif sudah penuh (2).'
          ),
        }),
      })
      const { screen } = await renderForm()
      await userEvent.fill(
        screen.getByLabelText('Nama kegiatan'),
        'Kuliah pengganti'
      )
      await userEvent.click(submitButton(screen))
      await expect
        .element(screen.getByText('Kuota booking aktif penuh'))
        .toBeVisible()
      await expect
        .element(screen.getByText('Kuota booking aktif sudah penuh (2).'))
        .toBeVisible()
    })
  })

  describe('cancelling', () => {
    it('closes without calling the API', async () => {
      const post = vi.fn(() => ({ status: 201 }))
      serve({ booking: post })
      const { screen, onCancel } = await renderForm()
      await userEvent.click(screen.getByRole('button', { name: 'Batal' }))
      expect(onCancel).toHaveBeenCalledOnce()
      expect(post).not.toHaveBeenCalled()
    })
  })

  describe('sending', () => {
    it('validates locally before calling the API', async () => {
      const post = vi.fn(() => ({ status: 201 }))
      serve({ booking: post })
      const { screen } = await renderForm()
      await userEvent.click(submitButton(screen))
      await expect
        .element(screen.getByText('Nama kegiatan wajib diisi.'))
        .toBeVisible()
      expect(post).not.toHaveBeenCalled()
    })

    it('submits WIB timestamps with an Idempotency-Key and reuses it on retry', async () => {
      const keys: string[] = []
      const bodies: unknown[] = []
      serve({
        booking: (config) => {
          keys.push(String(config.headers.get('Idempotency-Key')))
          bodies.push(JSON.parse(String(config.data)))
          return {
            status: 503,
            data: apiErrorBody('SERVICE_BUSY', 'Server sibuk, coba lagi.'),
          }
        },
      })
      const { screen } = await renderForm()
      await userEvent.fill(
        screen.getByLabelText('Nama kegiatan'),
        'Kuliah pengganti'
      )
      await userEvent.fill(screen.getByLabelText('Jumlah peserta'), '35')
      await userEvent.click(submitButton(screen))
      await expect
        .element(screen.getByText('Server sibuk, coba lagi.'))
        .toBeVisible()
      await userEvent.click(submitButton(screen))
      await vi.waitFor(() => expect(keys).toHaveLength(2))

      expect(bodies[0]).toEqual({
        room_id: KELAS_ROOM.id,
        start_at: `${tomorrow}T08:00:00+07:00`,
        end_at: `${tomorrow}T10:00:00+07:00`,
        purpose: 'Kuliah pengganti',
        participants: 35,
      })
      expect(keys[0]).toMatch(/^[0-9a-f-]{36}$/)
      expect(keys[1]).toBe(keys[0])
    })

    it('sends several dates as occurrences', async () => {
      const bodies: Record<string, unknown>[] = []
      serve({
        booking: (config) => {
          bodies.push(JSON.parse(String(config.data)))
          return {
            status: 201,
            data: { group_id: 'g1', status: 'MENUNGGU', bookings: [{}, {}] },
          }
        },
      })
      const { screen, onCreated } = await renderForm()
      await userEvent.click(
        screen.getByRole('button', { name: 'Tambah Tanggal' })
      )
      await userEvent.fill(
        screen.getByLabelText('Tanggal 2', { exact: true }),
        dayAfter
      )
      await userEvent.click(
        screen.getByRole('combobox', { name: 'Mulai' }).nth(1)
      )
      await userEvent.click(screen.getByRole('option', { name: '09:00' }))
      await userEvent.click(
        screen.getByRole('combobox', { name: 'Selesai' }).nth(1)
      )
      await userEvent.click(screen.getByRole('option', { name: '11:00' }))
      await userEvent.fill(
        screen.getByLabelText('Nama kegiatan'),
        'Kuliah pengganti'
      )
      await expect.element(screen.getByText('2 tanggal')).toBeVisible()

      await userEvent.click(submitButton(screen))

      await vi.waitFor(() => expect(onCreated).toHaveBeenCalledOnce())
      expect(bodies[0]).toMatchObject({
        room_id: KELAS_ROOM.id,
        occurrences: [
          {
            start_at: `${tomorrow}T08:00:00+07:00`,
            end_at: `${tomorrow}T10:00:00+07:00`,
          },
          {
            start_at: `${dayAfter}T09:00:00+07:00`,
            end_at: `${dayAfter}T11:00:00+07:00`,
          },
        ],
      })
      expect(bodies[0]).not.toHaveProperty('start_at')
    })

    it('explains a 409 conflict with the room and the clashing time', async () => {
      serve({
        booking: () => ({
          status: 409,
          data: apiErrorBody(
            'BOOKING_CONFLICT',
            'Bentrok: R-302 sudah terisi.',
            [
              {
                kind: 'SCHEDULE',
                title: 'Kuliah Kalkulus',
                start_at: `${tomorrow}T08:30:00+07:00`,
                end_at: `${tomorrow}T10:00:00+07:00`,
              },
            ]
          ),
        }),
      })
      const { screen, onCreated } = await renderForm()
      await userEvent.fill(
        screen.getByLabelText('Nama kegiatan'),
        'Kuliah pengganti'
      )
      await userEvent.click(submitButton(screen))

      await expect
        .element(screen.getByText('Bentrok: R-302 sudah terisi.'))
        .toBeVisible()
      await expect.element(screen.getByText(/08:30 – 10:00/)).toBeVisible()
      await expect.element(screen.getByText(/Kuliah Kalkulus/)).toBeVisible()
      await expect.element(submitButton(screen)).toBeDisabled()
      expect(onCreated).not.toHaveBeenCalled()
    })

    it('puts server field errors on the matching fields', async () => {
      serve({
        booking: () => ({
          status: 422,
          data: apiErrorBody('VALIDATION_ERROR', 'Data tidak valid.', [
            { field: 'purpose', message: 'Nama kegiatan tidak boleh kosong.' },
          ]),
        }),
      })
      const { screen } = await renderForm()
      await userEvent.fill(
        screen.getByLabelText('Nama kegiatan'),
        'Kuliah pengganti'
      )
      await userEvent.click(submitButton(screen))
      await expect
        .element(screen.getByText('Nama kegiatan tidak boleh kosong.'))
        .toBeVisible()
    })

    it('reports the created booking', async () => {
      serve({
        booking: (config) => ({
          status: 201,
          data: {
            id: 'b1',
            status: 'DISETUJUI',
            room: KELAS_ROOM,
            ...JSON.parse(String(config.data)),
          },
        }),
      })
      const { screen, onCreated } = await renderForm()
      await userEvent.fill(
        screen.getByLabelText('Nama kegiatan'),
        'Kuliah pengganti'
      )
      await userEvent.click(submitButton(screen))
      await vi.waitFor(() => expect(onCreated).toHaveBeenCalledOnce())
      expect(onCreated.mock.calls[0][0]).toMatchObject({
        id: 'b1',
        status: 'DISETUJUI',
      })
    })
  })

  describe('an organisation booking the auditorium', () => {
    beforeEach(() => {
      useAuthStore.getState().auth.reset()
      signIn('ORGANISASI')
    })

    const audProps = {
      room: AUD_ROOM as Room,
      initialOccurrences: [
        { date: tomorrow, start_time: '13:00', end_time: '16:00' },
      ],
    }

    it('promises manual validation and asks for the letter and headcount', async () => {
      serve()
      const { screen } = await renderForm(audProps)
      await expect
        .element(
          screen.getByText('Menunggu validasi TU / Admin', { exact: true })
        )
        .toBeVisible()
      await expect
        .element(screen.getByText('Surat tugas / kegiatan (wajib)'))
        .toBeVisible()
      await expect
        .element(screen.getByText('Jumlah peserta (wajib)'))
        .toBeVisible()
    })

    it('does not send without the letter and the headcount', async () => {
      const post = vi.fn(() => ({ status: 201 }))
      serve({ booking: post })
      const { screen } = await renderForm(audProps)
      await userEvent.fill(
        screen.getByLabelText('Nama kegiatan'),
        'Seminar Nasional'
      )
      await userEvent.click(submitButton(screen))
      await expect
        .element(screen.getByText('Surat wajib dilampirkan.'))
        .toBeVisible()
      await expect
        .element(screen.getByText('Jumlah peserta wajib diisi.'))
        .toBeVisible()
      expect(post).not.toHaveBeenCalled()
    })

    it('uploads the letter and sends its id with the letter number', async () => {
      const bodies: Record<string, unknown>[] = []
      let uploaded: FormDataEntryValue | null = null
      serve({
        upload: (config) => {
          uploaded = (config.data as FormData).get('kind')
          return {
            status: 201,
            data: {
              id: 'att-1',
              kind: 'LETTER',
              content_type: 'application/pdf',
              size_bytes: 1_200_000,
              original_name: 'Surat-Kegiatan.pdf',
              created_at: '2026-10-21T08:00:00+07:00',
            },
          }
        },
        booking: (config) => {
          bodies.push(JSON.parse(String(config.data)))
          return {
            status: 201,
            data: { id: 'b1', status: 'MENUNGGU', room: AUD_ROOM },
          }
        },
      })
      const { screen, onCreated } = await renderForm(audProps)

      await expect
        .element(screen.getByText('Surat tugas / kegiatan (wajib)'))
        .toBeVisible()
      const input = screen.container.querySelector(
        'input[type=file]'
      ) as HTMLInputElement
      await userEvent.upload(
        input,
        new File(['%PDF-1.4'], 'Surat-Kegiatan.pdf', {
          type: 'application/pdf',
        })
      )
      await expect.element(screen.getByText('Surat-Kegiatan.pdf')).toBeVisible()
      await expect.element(screen.getByText(/PDF · 1,1 MB/)).toBeVisible()
      expect(uploaded).toBe('LETTER')

      await userEvent.fill(
        screen.getByLabelText('Nomor surat (opsional)'),
        '012/HMIF/X/2026'
      )
      await userEvent.fill(
        screen.getByLabelText('Nama kegiatan'),
        'Seminar Nasional'
      )
      await userEvent.fill(
        screen.getByLabelText('Jumlah peserta (wajib)'),
        '150'
      )
      await userEvent.click(submitButton(screen))

      await vi.waitFor(() => expect(onCreated).toHaveBeenCalledOnce())
      expect(bodies[0]).toMatchObject({
        room_id: AUD_ROOM.id,
        letter_attachment_id: 'att-1',
        letter_number: '012/HMIF/X/2026',
        participants: 150,
      })
    })

    it('rejects a letter over 5 MB without uploading it', async () => {
      const upload = vi.fn(() => ({ status: 201 }))
      serve({ upload })
      const { screen } = await renderForm(audProps)
      await expect
        .element(screen.getByText('Surat tugas / kegiatan (wajib)'))
        .toBeVisible()
      const input = screen.container.querySelector(
        'input[type=file]'
      ) as HTMLInputElement
      const big = new File([new Uint8Array(5 * 1024 * 1024 + 1)], 'besar.pdf', {
        type: 'application/pdf',
      })
      await userEvent.upload(input, big)
      await expect
        .element(screen.getByText('Berkas terlalu besar. Maksimal 5 MB.'))
        .toBeVisible()
      expect(upload).not.toHaveBeenCalled()
    })

    it('shows the server message when the letter is refused', async () => {
      serve({
        upload: () => ({
          status: 422,
          data: apiErrorBody('VALIDATION_ERROR', 'Berkas tidak valid.', [
            { field: 'file', message: 'Tipe berkas tidak didukung.' },
          ]),
        }),
      })
      const { screen } = await renderForm(audProps)
      await expect
        .element(screen.getByText('Surat tugas / kegiatan (wajib)'))
        .toBeVisible()
      const input = screen.container.querySelector(
        'input[type=file]'
      ) as HTMLInputElement
      await userEvent.upload(
        input,
        new File(['x'], 'surat.pdf', { type: 'application/pdf' })
      )
      await expect
        .element(screen.getByText('Tipe berkas tidak didukung.'))
        .toBeVisible()
    })
  })
})
