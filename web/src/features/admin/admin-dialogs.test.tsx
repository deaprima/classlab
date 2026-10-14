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
import { RoomFormDialog } from './rooms/room-form-dialog'
import { ScheduleFormDialog } from './schedules/schedule-form-dialog'
import { UserDialog } from './users/user-dialogs'

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }))
vi.mock('sonner', () => ({ toast }))

const originalAdapter = api.defaults.adapter
const KAPRODI = {
  ...sampleUser,
  id: 'kaprodi-id',
  role: 'KAPRODI' as const,
  name: 'Budi',
}
const ROOM: Room = {
  id: '0b6a3a8e-1d2c-4c7e-9f0a-1234567890ab',
  org_unit_id: sampleUser.org_unit_id,
  code: 'LAB-1',
  name: 'Lab Jaringan',
  type: 'LAB',
  capacity: 30,
  facilities: ['Proyektor', 'AC'],
  is_active: true,
}

function wrap(ui: React.ReactNode) {
  return render(
    <QueryClientProvider client={new QueryClient()}>{ui}</QueryClientProvider>
  )
}

describe('admin dialogs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore.getState().auth.setSession('t', KAPRODI)
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('creates a room in the admin’s unit with parsed facilities', async () => {
    const bodies: unknown[] = []
    api.defaults.adapter = mockAdapter((config) => {
      bodies.push(JSON.parse(String(config.data)))
      return { status: 201, data: { ...ROOM, code: 'LAB-4' } }
    })
    const onOpenChange = vi.fn()
    const screen = await wrap(
      <RoomFormDialog open onOpenChange={onOpenChange} />
    )

    await userEvent.fill(screen.getByLabelText('Kode'), 'lab-4')
    await userEvent.fill(screen.getByLabelText('Nama'), 'Lab AI')
    await userEvent.fill(screen.getByLabelText('Kapasitas'), '25')
    await userEvent.fill(
      screen.getByLabelText('Fasilitas'),
      'Proyektor, AC\nGPU'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))

    await vi.waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
    expect(bodies[0]).toEqual({
      org_unit_id: KAPRODI.org_unit_id,
      code: 'lab-4',
      name: 'Lab AI',
      type: 'KELAS',
      capacity: 25,
      facilities: ['Proyektor', 'AC', 'GPU'],
    })
  })

  it('shows a duplicate room code on the code field', async () => {
    api.defaults.adapter = mockAdapter(() => ({
      status: 422,
      data: apiErrorBody('VALIDATION_ERROR', 'Data tidak valid.', [
        { field: 'code', message: 'Kode ruang sudah digunakan.' },
      ]),
    }))
    const screen = await wrap(<RoomFormDialog open onOpenChange={vi.fn()} />)

    await userEvent.fill(screen.getByLabelText('Kode'), 'LAB-1')
    await userEvent.fill(screen.getByLabelText('Nama'), 'Duplikat')
    await userEvent.fill(screen.getByLabelText('Kapasitas'), '10')
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))

    await expect
      .element(screen.getByText('Kode ruang sudah digunakan.'))
      .toBeInTheDocument()
  })

  it('lists every clashing session when a weekly schedule conflicts', async () => {
    const bodies: unknown[] = []
    api.defaults.adapter = mockAdapter((config) => {
      bodies.push(JSON.parse(String(config.data)))
      return {
        status: 409,
        data: apiErrorBody('BOOKING_CONFLICT', 'Jadwal bentrok.', [
          {
            kind: 'SCHEDULE',
            start_at: '2026-10-26T08:00:00+07:00',
            end_at: '2026-10-26T10:00:00+07:00',
          },
          {
            kind: 'BOOKING',
            status: 'DISETUJUI',
            start_at: '2026-11-02T09:00:00+07:00',
            end_at: '2026-11-02T11:00:00+07:00',
          },
        ]),
      }
    })
    const screen = await wrap(
      <ScheduleFormDialog
        state={{ mode: 'create', roomId: ROOM.id, date: '2026-10-19' }}
        rooms={[ROOM]}
        onClose={vi.fn()}
      />
    )

    await userEvent.fill(screen.getByLabelText('Judul'), 'Basis Data')
    await userEvent.click(screen.getByLabelText('Ulangi setiap minggu'))
    await expect
      .element(screen.getByText(/16 sesi akan dibuat/))
      .toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Buat 16 sesi' }))

    await expect
      .element(screen.getByText('Jadwal bentrok.'))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText(/Sen, 26 Okt 2026, 08:00–10:00/))
      .toBeInTheDocument()
    await expect
      .element(screen.getByText(/Sen, 2 Nov 2026, 09:00–11:00/))
      .toBeInTheDocument()
    expect(bodies[0]).toMatchObject({
      room_id: ROOM.id,
      title: 'Basis Data',
      source: 'AKADEMIK',
      start_at: '2026-10-19T07:30:00+07:00',
      end_at: '2026-10-19T10:00:00+07:00',
      repeat: { freq: 'WEEKLY', until: '2027-02-01' },
    })
  })

  it('does not send role or status when Kaprodi edits their own account', async () => {
    const bodies: unknown[] = []
    api.defaults.adapter = mockAdapter((config) => {
      bodies.push(JSON.parse(String(config.data)))
      return { status: 200, data: KAPRODI }
    })
    const screen = await wrap(
      <UserDialog state={{ mode: 'edit', user: KAPRODI }} onClose={vi.fn()} />
    )

    await expect.element(screen.getByRole('switch')).toBeDisabled()
    await userEvent.fill(screen.getByLabelText('Nama'), 'Budi Santoso')
    await userEvent.click(screen.getByRole('button', { name: 'Simpan' }))

    await vi.waitFor(() =>
      expect(bodies).toEqual([{ name: 'Budi Santoso', email: KAPRODI.email }])
    )
  })

  it('resets a password only when both entries match', async () => {
    const bodies: unknown[] = []
    api.defaults.adapter = mockAdapter((config) => {
      bodies.push(JSON.parse(String(config.data)))
      return { status: 200, data: sampleUser }
    })
    const screen = await wrap(
      <UserDialog
        state={{ mode: 'password', user: { ...sampleUser } }}
        onClose={vi.fn()}
      />
    )

    await userEvent.fill(
      screen.getByLabelText('Kata sandi baru', { exact: true }),
      'baru-12345'
    )
    await userEvent.fill(
      screen.getByLabelText('Ulangi kata sandi baru'),
      'beda-12345'
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Reset kata sandi' })
    )
    await expect
      .element(screen.getByText('Konfirmasi kata sandi tidak sama.'))
      .toBeInTheDocument()
    expect(bodies).toHaveLength(0)

    await userEvent.fill(
      screen.getByLabelText('Ulangi kata sandi baru'),
      'baru-12345'
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Reset kata sandi' })
    )
    await vi.waitFor(() => expect(bodies).toEqual([{ password: 'baru-12345' }]))
  })
})
