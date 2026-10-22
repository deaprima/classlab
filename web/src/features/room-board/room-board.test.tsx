import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { addDays, todayWib } from '@/lib/datetime'
import { POLICIES } from '@/features/my-bookings/fixtures'
import { RoomBoard } from '.'

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

const originalAdapter = api.defaults.adapter

const room = (code: string, type: string) => ({
  id: code,
  org_unit_id: 'u',
  code,
  name: `Ruang ${code}`,
  type,
  capacity: 30,
  facilities: [],
  is_active: true,
  open_time: null,
  close_time: null,
})
const ROOMS = [room('R-1', 'KELAS'), room('R-2', 'KELAS'), room('LAB-1', 'LAB')]

function serve(requests: Record<string, string>[] = []) {
  api.defaults.adapter = mockAdapter((config) => {
    const url = String(config.url)
    if (url === '/room-types') return { status: 200, data: { data: POLICIES } }
    if (url === '/rooms') {
      return {
        status: 200,
        data: {
          data: ROOMS,
          meta: { page: 1, per_page: 100, total: ROOMS.length },
        },
      }
    }
    const schedule = /^\/rooms\/([^/]+)\/schedule$/.exec(url)
    if (schedule) {
      requests.push({ room: schedule[1], ...(config.params as object) })
      const day = (config.params as { from: string }).from.slice(0, 10)
      return {
        status: 200,
        data: {
          data:
            schedule[1] === 'R-1'
              ? [
                  {
                    id: 'i1',
                    kind: 'SCHEDULE',
                    title: 'Kuliah Basis Data',
                    start_at: `${day}T09:00:00+07:00`,
                    end_at: `${day}T11:00:00+07:00`,
                  },
                ]
              : [],
        },
      }
    }
    return { status: 404 }
  })
  return requests
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <RoomBoard />
    </QueryClientProvider>
  )
}

describe('RoomBoard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.search = {}
    useAuthStore.getState().auth.setSession('t', { ...sampleUser, role: 'TU' })
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('shows every room as a column with its lessons, for today', async () => {
    const requests = serve()
    const screen = await renderPage()

    await expect
      .element(screen.getByRole('columnheader', { name: 'R-1' }))
      .toBeVisible()
    await expect
      .element(screen.getByRole('columnheader', { name: 'LAB-1' }))
      .toBeVisible()
    await expect.element(screen.getByText('Kuliah Basis Data')).toBeVisible()
    // One timeline per room, all for the same day: today.
    expect(requests).toHaveLength(3)
    expect(new Set(requests.map((r) => r.from.slice(0, 10)))).toEqual(
      new Set([todayWib()])
    )
  })

  it('reads the day from the URL', async () => {
    const day = addDays(todayWib(), 2)
    mocks.search = { date: day }
    const requests = serve()
    const screen = await renderPage()

    await expect.element(screen.getByText('Kuliah Basis Data')).toBeVisible()
    expect(requests.every((r) => r.from.startsWith(day))).toBe(true)
  })

  it('narrows to a group of room types', async () => {
    mocks.search = { group: 'LAB' }
    const requests = serve()
    const screen = await renderPage()

    await expect
      .element(screen.getByRole('columnheader', { name: 'LAB-1' }))
      .toBeVisible()
    expect(
      screen.getByRole('columnheader', { name: 'R-1' }).elements()
    ).toEqual([])
    expect(requests.map((r) => r.room)).toEqual(['LAB-1'])
  })

  it('moves a day with the arrows and returns with "Hari ini"', async () => {
    mocks.search = { date: addDays(todayWib(), 1) }
    serve()
    const screen = await renderPage()

    await userEvent.click(
      screen.getByRole('button', { name: 'Hari berikutnya' })
    )
    const next = mocks.navigate.mock.calls[0][0] as {
      search: (prev: object) => object
    }
    expect(next.search({ date: mocks.search.date })).toEqual({
      date: addDays(todayWib(), 2),
    })

    await userEvent.click(screen.getByRole('button', { name: 'Hari ini' }))
    const back = mocks.navigate.mock.calls[1][0] as {
      search: (prev: object) => object
    }
    expect(back.search({})).toEqual({ date: undefined })
  })
})
