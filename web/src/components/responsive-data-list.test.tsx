import { describe, expect, it } from 'vitest'
import { render } from 'vitest-browser-react'
import { ResponsiveDataList, type DataColumn } from './responsive-data-list'

type Row = { id: string; room: string; time: string }

const rows: Row[] = [
  { id: '1', room: 'R-301', time: '08:00 – 10:00' },
  { id: '2', room: 'LAB-RPL-02', time: '10:00 – 12:00' },
]

const columns: DataColumn<Row>[] = [
  { key: 'room', header: 'Ruang', cell: (r) => r.room, primary: true },
  { key: 'time', header: 'Waktu', cell: (r) => r.time },
  {
    key: 'act',
    header: 'Aksi',
    cell: (r) => <button>Buka {r.room}</button>,
    actions: true,
  },
]

describe('ResponsiveDataList', () => {
  it('renders a table with column headers on desktop', async () => {
    const screen = await render(
      <ResponsiveDataList
        layout='table'
        caption='Pengajuan'
        rows={rows}
        columns={columns}
        rowKey={(r) => r.id}
      />
    )
    await expect
      .element(screen.getByRole('columnheader', { name: 'Waktu' }))
      .toBeVisible()
    await expect.element(screen.getByRole('row')).toHaveLength(3)
    await expect
      .element(screen.getByText('LAB-RPL-02', { exact: true }))
      .toBeVisible()
  })

  it('renders one card per row on mobile with labelled details and actions', async () => {
    const screen = await render(
      <ResponsiveDataList
        layout='cards'
        caption='Pengajuan'
        rows={rows}
        columns={columns}
        rowKey={(r) => r.id}
      />
    )
    expect(screen.container.querySelector('table')).toBeNull()
    await expect.element(screen.getByRole('listitem')).toHaveLength(2)
    await expect.element(screen.getByText('Waktu').first()).toBeVisible()
    await expect
      .element(screen.getByRole('button', { name: 'Buka R-301' }))
      .toBeVisible()
  })

  it('does not scroll sideways in a 360 px wide container', async () => {
    const screen = await render(
      <div style={{ width: 360 }}>
        <ResponsiveDataList
          layout='cards'
          caption='Pengajuan'
          rows={rows}
          columns={columns}
          rowKey={(r) => r.id}
        />
      </div>
    )
    const wrapper = screen.container.firstElementChild as HTMLElement
    expect(wrapper.scrollWidth).toBeLessThanOrEqual(wrapper.clientWidth)
  })
})
