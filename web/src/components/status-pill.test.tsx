import { describe, expect, it } from 'vitest'
import { render } from 'vitest-browser-react'
import { type BookingStatus } from '@/lib/api-types'
import { BOOKING_STATUS_LABELS } from '@/lib/status-labels'
import {
  AVAILABILITY_PILL,
  BOOKING_PILL,
  REPORT_PILL,
} from '@/lib/status-pill-map'
import { BookingStatusBadge } from './status-badge'
import { StatusPill } from './status-pill'

describe('status mapping (docs/tasks.md C5)', () => {
  it('maps every booking status to one of the ten Figma pills', () => {
    expect(BOOKING_PILL).toEqual({
      MENUNGGU: 'Menunggu',
      DISETUJUI: 'Disetujui',
      DITOLAK: 'Ditolak',
      DIBATALKAN: 'Terbatas',
      BERLANGSUNG: 'Digunakan',
      SELESAI: 'Selesai',
      TIDAK_HADIR: 'Ditolak',
      TANPA_CHECKOUT: 'Ditolak',
      KEDALUWARSA: 'Terbatas',
    })
  })

  it('maps availability and report statuses', () => {
    expect(AVAILABILITY_PILL.DIAJUKAN).toBe('Diajukan')
    expect(REPORT_PILL.BARU).toBe('Diajukan')
    expect(REPORT_PILL.DITERIMA).toBe('Diterima')
  })
})

describe('StatusPill', () => {
  it('shows the status as text by default', async () => {
    const screen = await render(<StatusPill status='Tersedia' />)
    await expect.element(screen.getByText('Tersedia')).toBeVisible()
  })

  it('keeps the API label while using the mapped style', async () => {
    const status: BookingStatus = 'TANPA_CHECKOUT'
    const screen = await render(<BookingStatusBadge status={status} />)
    const label = screen.getByText(BOOKING_STATUS_LABELS[status])
    await expect.element(label).toBeVisible()
    await expect.element(label).toHaveAttribute('data-status', 'Ditolak')
  })
})
