import { describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { BentoCard } from './bento-card'
import { DateBadge } from './date-badge'
import { Dropzone } from './dropzone'
import { EmptyState } from './empty-state'
import { ReportCard } from './report-card'
import { RoomTile } from './room-tile'
import { SegmentedControl } from './segmented-control'
import { SlotBlock } from './slot-block'
import { StatCard } from './stat-card'
import { Step } from './step'

describe('SegmentedControl', () => {
  it('marks the active option and reports a change', async () => {
    const onValueChange = vi.fn()
    const screen = await render(
      <SegmentedControl
        aria-label='Tampilan'
        value='pengajuan'
        onValueChange={onValueChange}
        options={[
          { value: 'pengajuan', label: 'Pengajuan', count: 3 },
          { value: 'riwayat', label: 'Riwayat' },
        ]}
      />
    )
    await expect
      .element(screen.getByRole('radio', { name: /Pengajuan/ }))
      .toBeChecked()
    await userEvent.click(screen.getByRole('radio', { name: 'Riwayat' }))
    expect(onValueChange).toHaveBeenCalledWith('riwayat')
  })
})

describe('Dropzone', () => {
  it('passes the chosen files, one or many', async () => {
    const a = new File(['a'], 'a.pdf', { type: 'application/pdf' })
    const b = new File(['b'], 'b.pdf', { type: 'application/pdf' })

    const single = vi.fn()
    const one = await render(<Dropzone onFiles={single} accept='.pdf' />)
    await userEvent.upload(
      one.container.querySelector('input[type=file]') as HTMLInputElement,
      a
    )
    expect(single).toHaveBeenCalledWith([
      expect.objectContaining({ name: 'a.pdf' }),
    ])
    await one.unmount()

    const many = vi.fn()
    const two = await render(<Dropzone onFiles={many} multiple />)
    await userEvent.upload(
      two.container.querySelector('input[type=file]') as HTMLInputElement,
      [a, b]
    )
    expect(many.mock.calls[0][0]).toHaveLength(2)
  })

  it('does not accept files while disabled', async () => {
    const screen = await render(<Dropzone onFiles={vi.fn()} disabled />)
    await expect
      .element(screen.getByRole('button', { name: 'Pilih berkas' }))
      .toBeDisabled()
  })
})

describe('display components', () => {
  it('DateBadge shows the WIB date parts', async () => {
    const screen = await render(<DateBadge date='2026-10-19' />)
    await expect.element(screen.getByText('19')).toBeVisible()
    await expect.element(screen.getByText('Sen')).toBeVisible()
    await expect.element(screen.getByText('Okt')).toBeVisible()
  })

  it('SlotBlock states its status in text for screen readers', async () => {
    const screen = await render(<SlotBlock status='Digunakan' time='08:00' />)
    await expect.element(screen.getByText('Digunakan')).toBeInTheDocument()
  })

  it('StatCard labels the attention dot', async () => {
    const screen = await render(
      <StatCard title='Menunggu' value={4} showDot caption='perlu keputusan' />
    )
    await expect
      .element(screen.getByRole('img', { name: 'Perlu perhatian' }))
      .toBeInTheDocument()
  })

  it('EmptyState renders title, description and action', async () => {
    const screen = await render(
      <EmptyState
        title='Belum ada pengajuan'
        description='Ajukan ruang pertamamu.'
        action={<button>Ajukan Ruang</button>}
      />
    )
    await expect
      .element(screen.getByRole('heading', { name: 'Belum ada pengajuan' }))
      .toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Ajukan Ruang' }))
      .toBeVisible()
  })

  it('Step marks the current step', async () => {
    const screen = await render(
      <ol>
        <Step state='done' title='Diajukan' hasNext />
        <Step state='current' title='Menunggu persetujuan' />
      </ol>
    )
    await expect
      .element(screen.getByRole('listitem').nth(1))
      .toHaveAttribute('aria-current', 'step')
  })

  it('BentoCard shows its title and content slot', async () => {
    const screen = await render(
      <BentoCard title='Hari ini' action={<button>Lihat</button>}>
        <p>Isi kartu</p>
      </BentoCard>
    )
    await expect
      .element(screen.getByRole('heading', { name: 'Hari ini' }))
      .toBeVisible()
    await expect.element(screen.getByText('Isi kartu')).toBeVisible()
  })

  it('RoomTile shows code, capacity and availability', async () => {
    const screen = await render(
      <RoomTile code='R-301' capacity={40} status='Tersedia' />
    )
    await expect.element(screen.getByText('R-301')).toBeVisible()
    await expect.element(screen.getByText('40 kursi')).toBeVisible()
    await expect.element(screen.getByText('Tersedia')).toBeVisible()
  })

  it('ReportCard shows room, damage and status', async () => {
    const screen = await render(
      <ReportCard room='LAB-RPL-02' damage='Proyektor mati' status='Diproses' />
    )
    await expect.element(screen.getByText('Proyektor mati')).toBeVisible()
    await expect.element(screen.getByText('Diproses')).toBeVisible()
  })
})
