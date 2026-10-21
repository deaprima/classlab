// Responsive smoke test (docs/tasks.md 6.5.1): every student page at four viewports must not
// scroll sideways (`scrollWidth <= innerWidth`). The API is mocked in the browser, so no backend
// is needed. Run with `pnpm smoke:responsive`; set SMOKE_SHOTS=<dir> to keep a screenshot of each
// page and viewport. This does not replace the real-device checklist (docs/responsive-checklist.md).
import { mkdirSync } from 'node:fs'
import { chromium } from 'playwright'
import { createServer } from 'vite'

const PORT = 5198
const BASE = `http://localhost:${PORT}`
const SHOTS = process.env.SMOKE_SHOTS

const VIEWPORTS = [
  { name: '360x640', width: 360, height: 640 },
  { name: '390x844', width: 390, height: 844 },
  { name: '768x1024', width: 768, height: 1024 },
  { name: '1280x800', width: 1280, height: 800 },
]

const USER = {
  id: 'u1',
  org_unit_id: 'ou1',
  identifier: '2021101002',
  name: 'Bunga Lestari',
  email: 'bunga@classlab.test',
  role: 'MAHASISWA',
  is_active: true,
  booking_blocked_until: null,
  pic_name: null,
  pic_phone: null,
  must_change_password: false,
  active_points: 2,
  org_unit_name: 'Informatika',
  created_at: '2026-10-01T08:00:00+07:00',
}

const ROOM = {
  id: 'r1',
  org_unit_id: 'ou1',
  code: 'LAB-RPL-02',
  name: 'Laboratorium Rekayasa Perangkat Lunak',
  type: 'LAB',
  capacity: 32,
  facilities: ['Proyektor', 'AC'],
  is_active: true,
  open_time: null,
  close_time: null,
}

const day = (n) => {
  const d = new Date(Date.now() + n * 86_400_000)
  return new Date(d.getTime() + 7 * 3_600_000).toISOString().slice(0, 10)
}
const slot = (n, a, b) => ({
  start_at: `${day(n)}T${String(a).padStart(2, '0')}:00:00+07:00`,
  end_at: `${day(n)}T${String(b).padStart(2, '0')}:00:00+07:00`,
})

const booking = (id, status, n, purpose) => ({
  id,
  room_id: ROOM.id,
  room: ROOM,
  requester_id: USER.id,
  requester: { id: USER.id, identifier: USER.identifier, name: USER.name },
  ...slot(n, 10, 12),
  purpose,
  participants: 30,
  notes: '',
  details: {},
  status,
  approval_mode: 'MANUAL',
  occurrence_no: 1,
  occurrence_count: 1,
  created_at: '2026-10-20T08:00:00+07:00',
  updated_at: '2026-10-20T08:00:00+07:00',
})

const BOOKINGS = [
  booking('b1', 'DISETUJUI', 2, 'Praktikum Basis Data dan Pemrograman Web Lanjut'),
  booking('b2', 'MENUNGGU', 3, 'Rapat panitia dies natalis fakultas'),
  booking('b3', 'SELESAI', -5, 'Kuliah pengganti'),
  booking('b4', 'DITOLAK', -3, 'Seminar proposal'),
]

const REPORT = {
  id: 'rp1',
  room_id: ROOM.id,
  room: ROOM,
  reporter_id: USER.id,
  reporter: { id: USER.id, identifier: USER.identifier, name: USER.name },
  description: 'Proyektor tidak menyala dan kabel HDMI putus di meja dosen',
  status: 'BARU',
  created_at: '2026-10-14T09:00:00+07:00',
  events: [],
}

const NOTE = {
  id: 'n1',
  type: 'BOOKING_APPROVED',
  title: 'Booking disetujui',
  body: 'Pengajuan LAB-RPL-02 pada 25/10/2026 10:00-12:00 WIB telah disetujui.',
  ref_type: 'BOOKING',
  ref_id: 'b1',
  created_at: '2026-10-14T09:00:00+07:00',
}

const page1 = (data) => ({ data, meta: { page: 1, per_page: 20, total: data.length } })

function reply(method, path, search) {
  if (path.endsWith('/auth/refresh')) return { access_token: 'smoke', expires_in: 900 }
  if (path.endsWith('/auth/me')) return USER
  if (path.endsWith('/room-types')) return { data: [] }
  if (path.endsWith('/notifications/unread-count')) return { count: 1 }
  if (path.endsWith('/notifications')) return page1([NOTE])
  if (path.endsWith('/reports')) return page1([REPORT])
  if (path.endsWith('/violations')) {
    return {
      ...page1([
        {
          id: 'v1',
          booking_id: 'b3',
          kind: 'BATAL_TERLAMBAT',
          points: 1,
          active: true,
          annulled_at: null,
          annulled_by: null,
          annul_reason: null,
          created_at: '2026-10-12T08:00:00+07:00',
          room_code: 'R-302',
          booking_start_at: '2026-10-12T10:00:00+07:00',
        },
      ]),
      active_points: 2,
      booking_blocked_until: null,
    }
  }
  if (path.endsWith('/availability')) return { data: [] }
  if (path.endsWith('/schedule')) return { data: [] }
  if (path.endsWith('/rooms')) return page1([ROOM])
  if (/\/bookings\/[^/]+\/events$/.test(path)) return { data: [] }
  if (/\/bookings\/[^/]+$/.test(path)) return BOOKINGS[0]
  if (path.endsWith('/bookings')) {
    const status = search.get('status')
    const list = status ? BOOKINGS.filter((b) => b.status === status) : BOOKINGS
    return page1(list)
  }
  return page1([])
}

async function mockApi(page) {
  await page.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url())
    if (url.pathname.endsWith('/notifications/stream')) return route.abort()
    if (url.pathname.endsWith('/check-in')) {
      return route.fulfill({
        status: 409,
        contentType: 'application/json',
        body: JSON.stringify({
          error: { code: 'NO_ELIGIBLE_BOOKING', message: 'x', details: [], request_id: 'r' },
        }),
      })
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(reply(route.request().method(), url.pathname, url.searchParams)),
    })
  })
}

const PAGES = [
  '/sign-in',
  '/',
  '/schedule',
  '/bookings',
  '/history',
  '/reports',
  '/notifications',
  '/account',
  '/guide',
  '/check-in?room=LAB-RPL-02&t=token',
  '/check-out/b1',
  '/change-password',
]

const server = await createServer({
  server: { port: PORT, strictPort: true },
  logLevel: 'error',
})
await server.listen()
const browser = await chromium.launch()
const failures = []

try {
  if (SHOTS) mkdirSync(SHOTS, { recursive: true })
  for (const viewport of VIEWPORTS) {
    for (const path of PAGES) {
      const context = await browser.newContext({
        viewport: { width: viewport.width, height: viewport.height },
      })
      const page = await context.newPage()
      // Signed out only on the sign-in page; everything else restores the mocked session.
      if (path === '/sign-in') {
        await page.route('**/api/v1/auth/refresh', (r) =>
          r.fulfill({ status: 401, contentType: 'application/json', body: '{}' })
        )
      } else {
        await mockApi(page)
      }
      await page.goto(BASE + path, { waitUntil: 'networkidle' }).catch(() => {})
      await page.waitForTimeout(500)
      const { scrollWidth, innerWidth } = await page.evaluate(() => ({
        scrollWidth: document.documentElement.scrollWidth,
        innerWidth: window.innerWidth,
      }))
      const ok = scrollWidth <= innerWidth
      console.log(
        `${ok ? 'ok  ' : 'FAIL'} ${viewport.name.padEnd(9)} ${path} (scrollWidth ${scrollWidth}, innerWidth ${innerWidth})`
      )
      if (!ok) failures.push(`${viewport.name} ${path}`)
      if (SHOTS) {
        const file = `${viewport.name}${path.replace(/[^a-z0-9]+/gi, '_')}.png`
        await page.screenshot({ path: `${SHOTS}/${file}`, fullPage: !!process.env.SMOKE_FULL })
      }
      await context.close()
    }
  }
} finally {
  await browser.close()
  await server.close()
}

if (failures.length > 0) {
  console.error(`\n${failures.length} page(s) scroll sideways:\n- ${failures.join('\n- ')}`)
  process.exit(1)
}
console.log('\nNo horizontal scroll on any page or viewport.')
