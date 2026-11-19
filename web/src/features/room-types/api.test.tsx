import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter } from '@/test-utils/mock-adapter'
import { afterEach, describe, expect, it } from 'vitest'
import { render } from 'vitest-browser-react'
import { api } from '@/lib/api-client'
import { useRoomTypes } from './api'

const original = api.defaults.adapter

function Probe() {
  const { data, isPending } = useRoomTypes()
  if (isPending) return <p>memuat</p>
  return <p>{data?.map((p) => p.label).join(', ')}</p>
}

afterEach(() => {
  api.defaults.adapter = original
})

describe('useRoomTypes', () => {
  it('loads the policy list from GET /room-types', async () => {
    api.defaults.adapter = mockAdapter((req) => {
      expect(req.url).toBe('/room-types')
      return {
        status: 200,
        data: {
          data: [
            { type: 'KELAS', label: 'Ruang Kelas' },
            { type: 'LAB', label: 'Laboratorium' },
          ],
        },
      }
    })
    const screen = await render(
      <QueryClientProvider client={new QueryClient()}>
        <Probe />
      </QueryClientProvider>
    )
    await expect
      .element(screen.getByText('Ruang Kelas, Laboratorium'))
      .toBeVisible()
  })
})
