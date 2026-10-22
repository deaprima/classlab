import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { mockAdapter, sampleUser } from '@/test-utils/mock-adapter'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { useAuthStore } from '@/stores/auth-store'
import { api } from '@/lib/api-client'
import { Audit } from '.'
import { actionLabel, actorLabel, rangeStart, stateLines } from './rules'

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
vi.mock('@/hooks/use-media-query', () => ({ useMediaQuery: () => true }))
vi.mock('@/hooks/use-mobile', () => ({ useIsMobile: () => false }))
vi.mock('@/components/layout/page', () => ({
  Page: ({ title, children }: { title: string; children: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}))

const originalAdapter = api.defaults.adapter

const log = (id: string, over: Record<string, unknown> = {}) => ({
  id,
  actor_id: 'a1',
  actor_role: 'ADMIN',
  actor_identifier: 'admin',
  actor_name: 'Admin Sistem',
  action: 'USER_ROLE_CHANGED',
  entity_type: 'USER',
  entity_id: 'e1',
  before: { role: 'MAHASISWA' },
  after: { role: 'OB' },
  ip: '10.12.4.21',
  user_agent: 'x',
  request_id: 'a41f-92c0',
  created_at: '2026-10-19T07:02:00Z',
  ...over,
})
const LOGS = [
  log('l1'),
  log('l2', {
    actor_name: 'Pak Hendra',
    actor_role: 'TU',
    action: 'SCHEDULES_IMPORTED',
    entity_type: 'IMPORT',
    before: null,
    after: { occurrences: 1680 },
  }),
]

function serve(requests: Record<string, unknown>[] = []) {
  api.defaults.adapter = mockAdapter((config) => {
    if (String(config.url) === '/audit-logs') {
      requests.push((config.params ?? {}) as Record<string, unknown>)
      return {
        status: 200,
        data: {
          data: LOGS,
          meta: { page: 1, per_page: 10, total: 342 },
        },
      }
    }
    return { status: 404 }
  })
  return requests
}

function renderPage() {
  useAuthStore.getState().auth.setSession('t', { ...sampleUser, role: 'ADMIN' })
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <Audit />
    </QueryClientProvider>
  )
}

describe('Audit Log', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.search = {}
  })
  afterEach(() => {
    api.defaults.adapter = originalAdapter
    useAuthStore.getState().auth.reset()
  })

  it('lists the entries and shows before and after of the first one', async () => {
    const requests = serve()
    const screen = await renderPage()

    await expect
      .element(screen.getByText('Impor jadwal akademik'))
      .toBeVisible()
    const detail = screen.getByRole('complementary', {
      name: 'Detail aktivitas',
    })
    await expect.element(detail.getByText('Ubah peran pengguna')).toBeVisible()
    await expect.element(detail.getByText('role: MAHASISWA')).toBeVisible()
    await expect.element(detail.getByText('role: OB')).toBeVisible()
    await expect.element(detail.getByText('10.12.4.21')).toBeVisible()
    // The default range is the last 7 days.
    expect(requests[0].from).toBeTruthy()
    expect(requests[0].per_page).toBe(10)
  })

  it('selects another entry through the URL and filters by entity', async () => {
    serve()
    const screen = await renderPage()

    await userEvent.click(screen.getByText('Impor jadwal akademik'))
    const pick = mocks.navigate.mock.calls[0][0] as {
      search: (prev: object) => object
    }
    expect(pick.search({})).toEqual({ log: 'l2' })

    await userEvent.click(screen.getByRole('button', { name: 'Impor' }))
    const filter = mocks.navigate.mock.calls[1][0] as {
      search: (prev: object) => object
    }
    expect(filter.search({})).toEqual({
      page: undefined,
      log: undefined,
      entity: 'IMPORT',
    })
  })

  it('sends the entity and the range the URL asks for', async () => {
    mocks.search = { entity: 'ROOM_TYPE', range: 'all' }
    const requests = serve()
    await renderPage()

    await vi.waitFor(() => expect(requests.length).toBeGreaterThan(0))
    expect(requests[0].entity_type).toBe('ROOM_TYPE')
    expect(requests[0].from).toBeUndefined()
  })
})

describe('audit rules', () => {
  it('names known actions and keeps unknown ones as they are', () => {
    expect(actionLabel('USER_BLOCK_CLEARED')).toBe('Buka blokir')
    expect(actionLabel('SOMETHING_NEW')).toBe('SOMETHING_NEW')
  })

  it('says Sistem when nobody did it', () => {
    expect(actorLabel({ actor_name: null, actor_identifier: null })).toBe(
      'Sistem'
    )
    expect(actorLabel({ actor_name: null, actor_identifier: 'tu01' })).toBe(
      'tu01'
    )
  })

  it('turns a state into lines', () => {
    expect(stateLines(null)).toEqual([])
    expect(stateLines({ role: 'OB', active: true, note: null })).toEqual([
      'role: OB',
      'active: true',
      'note: –',
    ])
    expect(stateLines('x')).toEqual(['x'])
  })

  it('starts the range at the right moment, none for "all"', () => {
    const now = new Date('2026-10-22T05:00:00Z')
    expect(rangeStart('all', now)).toBeUndefined()
    expect(Date.parse(rangeStart('day', now)!)).toBe(now.getTime() - 86_400_000)
    expect(Date.parse(rangeStart('week', now)!)).toBe(
      Date.parse('2026-10-16T00:00:00+07:00')
    )
  })
})
