import { describe, expect, it } from 'vitest'
import { type Booking, type BookingEvent } from '@/lib/api-types'
import { buildTimeline } from './timeline'

function booking(over: Partial<Booking> = {}): Booking {
  return {
    status: 'MENUNGGU',
    approval_mode: 'MANUAL',
    created_at: '2026-10-19T09:12:00+07:00',
    decided_at: null,
    decision_reason: '',
    checked_in_at: null,
    checked_out_at: null,
    checkout_condition: null,
    ...over,
  } as Booking
}

function event(over: Partial<BookingEvent>): BookingEvent {
  return {
    id: 'e1',
    booking_id: 'b1',
    actor_id: null,
    kind: 'STATUS',
    to_status: 'MENUNGGU',
    created_at: '2026-10-19T10:00:00+07:00',
    ...over,
  } as BookingEvent
}

const titles = (steps: ReturnType<typeof buildTimeline>) =>
  steps.map((s) => `${s.state}:${s.title}`)

describe('buildTimeline', () => {
  it('shows a waiting request as sent, under review, and what comes next', () => {
    expect(titles(buildTimeline(booking(), [], 'TU'))).toEqual([
      'done:Pengajuan dikirim',
      'current:Ditinjau TU',
      'upcoming:Disetujui',
      'upcoming:Check-in & pemakaian',
    ])
  })

  it('shows an approved booking waiting for its start', () => {
    const steps = buildTimeline(
      booking({ status: 'DISETUJUI', approval_mode: 'AUTO' }),
      [],
      'TU'
    )
    expect(titles(steps)).toEqual([
      'done:Pengajuan dikirim',
      'done:Disetujui otomatis',
      'current:Check-in & pemakaian',
    ])
  })

  it('shows a rejection with its reason', () => {
    const steps = buildTimeline(
      booking({ status: 'DITOLAK', decision_reason: 'Ruang dipakai ujian.' }),
      [],
      'TU'
    )
    expect(steps[steps.length - 1]).toMatchObject({
      title: 'Ditolak',
      description: 'Ruang dipakai ujian.',
      state: 'done',
    })
  })

  it('follows a booking through use to the check-out', () => {
    const running = buildTimeline(
      booking({
        status: 'BERLANGSUNG',
        checked_in_at: '2026-10-21T13:50:00+07:00',
      }),
      [],
      'TU'
    )
    expect(titles(running).slice(-2)).toEqual([
      'done:Check-in',
      'current:Pemakaian',
    ])
    const finished = buildTimeline(
      booking({
        status: 'SELESAI',
        checked_in_at: '2026-10-21T13:50:00+07:00',
        checked_out_at: '2026-10-21T15:30:00+07:00',
        checkout_condition: 'ADA_MASALAH',
      }),
      [],
      'TU'
    )
    expect(finished[finished.length - 1]).toMatchObject({
      title: 'Selesai',
      description: 'Ada kerusakan yang dilaporkan',
    })
  })

  it('lists edits with the fields that changed, each once', () => {
    const steps = buildTimeline(
      booking(),
      [
        event({
          id: 'e2',
          kind: 'EDITED',
          changes: {
            start_at: { from: 'a', to: 'b' },
            end_at: { from: 'a', to: 'b' },
            participants: { from: 10, to: 20 },
          },
        }),
      ],
      'TU'
    )
    expect(steps[1]).toMatchObject({
      title: 'Pengajuan diubah',
      description: 'Diubah: waktu, peserta',
    })
  })

  it('takes the cancellation reason and time from the last cancel event', () => {
    const steps = buildTimeline(
      booking({ status: 'DIBATALKAN' }),
      [
        event({
          id: 'e3',
          to_status: 'DIBATALKAN',
          reason: 'Jadwal berubah',
          created_at: '2026-10-20T08:00:00+07:00',
        }),
      ],
      'TU'
    )
    expect(steps[steps.length - 1]).toMatchObject({
      title: 'Dibatalkan',
      description: 'Jadwal berubah',
    })
  })

  it('covers the remaining final statuses', () => {
    for (const status of [
      'TIDAK_HADIR',
      'TANPA_CHECKOUT',
      'KEDALUWARSA',
    ] as const) {
      const steps = buildTimeline(booking({ status }), [], 'TU')
      expect(steps[steps.length - 1]?.state).toBe('done')
    }
  })
})
