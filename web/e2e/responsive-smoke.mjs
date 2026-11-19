// Responsive smoke test (docs/tasks.md 6.5.1): every student page, every staff page (as ADMIN, who
// can open them all) and the home of each staff role, at four viewports, must not scroll sideways
// (`scrollWidth <= innerWidth`) and must not crash. The API is mocked in the browser, so no backend
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

// Staff accounts; ADMIN opens every staff page, the others only get their own home checked.
const staff = (id, identifier, name, role) => ({
  ...USER,
  id,
  identifier,
  name,
  email: `${role.toLowerCase()}@classlab.test`,
  role,
  active_points: 0,
  org_unit_name: 'Rumpun TI',
})
const ADMIN = staff('u2', '197501012000011001', 'Dr. Budi Santoso', 'ADMIN')
const TU = staff('u3', '198203152008012002', 'Siti Rahma', 'TU')
const KALAB = staff('u4', '197805202005011003', 'Dr. Hendra Wijaya', 'KALAB')
const OB = staff('u5', '199001102015011003', 'Agus Prasetyo', 'OB')

const policy = (type, label, approvers, requesters, open, close) => ({
  type,
  label,
  approver_roles: approvers,
  requester_roles: requesters,
  approval_policy: type === 'KELAS' ? 'AUTO_IF_ELIGIBLE' : 'ALWAYS_MANUAL',
  letter_required: type === 'AUDITORIUM',
  open_time: open,
  close_time: close,
  min_duration_min: 30,
  max_duration_min: 480,
  max_advance_days: 30,
  updated_at: '2026-10-20T08:00:00+07:00',
})
const POLICIES = [
  policy('KELAS', 'Ruang Kelas', ['TU'], ['MAHASISWA', 'ORGANISASI'], '07:00', '21:00'),
  policy('LAB', 'Laboratorium', ['KALAB'], ['MAHASISWA', 'ORGANISASI'], '07:00', '21:00'),
  policy('AUDITORIUM', 'Auditorium', ['TU', 'ADMIN'], ['ORGANISASI'], '07:00', '22:00'),
  policy('RUANG_UJIAN', 'Ruang Ujian', ['TU', 'ADMIN'], ['MAHASISWA', 'ORGANISASI'], '07:00', '18:00'),
  policy('RUANG_SIDANG', 'Ruang Sidang', ['TU', 'ADMIN'], ['MAHASISWA'], '07:00', '17:00'),
]

const AUDIT = {
  id: 'a1',
  actor_id: ADMIN.id,
  actor_role: 'ADMIN',
  actor_identifier: ADMIN.identifier,
  actor_name: ADMIN.name,
  action: 'ROOM_TYPE_POLICY_UPDATED',
  entity_type: 'ROOM_TYPE_POLICY',
  entity_id: null,
  before: { max_advance_days: 30 },
  after: { max_advance_days: 60 },
  ip: '10.0.0.12',
  user_agent: 'Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/130 Mobile Safari/537.36',
  request_id: 'req_01JBX7Q2W9F3ZK8M4N6P0R2T5V',
  created_at: '2026-10-20T09:15:00+07:00',
}

const IMPORT = {
  id: 'i1',
  kind: 'SCHEDULES',
  uploaded_by: { id: TU.id, identifier: TU.identifier, name: TU.name },
  file_name: 'jadwal-semester-ganjil-2026-rumpun-teknologi-informasi.csv',
  semester: '2026-GANJIL',
  status: 'COMMITTED',
  summary: { rows: 120, occurrences: 1680, bookings_cancelled: 2 },
  created_at: '2026-10-01T08:00:00+07:00',
  rolled_back_at: null,
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
  manual_reason: 'ROOM_POLICY',
  expires_at: status === 'MENUNGGU' ? `${day(1)}T10:00:00+07:00` : null,
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

const CHECKOUT = {
  booking_id: 'b3',
  room_id: ROOM.id,
  room_code: ROOM.code,
  room_name: ROOM.name,
  room_type: ROOM.type,
  checked_out_at: `${day(0)}T12:05:00+07:00`,
  condition: 'ADA_MASALAH',
  note: 'Dua kursi patah di baris belakang dan stopkontak meja 4 longgar',
  attachment_id: 'att1',
}

const HOURS = { open_time: '07:00', close_time: '21:00', custom: false }
const TIMELINE = [
  { id: 's1', kind: 'SCHEDULE', title: 'Pemrograman Berorientasi Objek Kelas A', ...slot(0, 8, 10) },
  { id: 'b1', kind: 'BOOKING', title: 'Praktikum Basis Data', ...slot(0, 13, 15) },
]

const page1 = (data) => ({ data, meta: { page: 1, per_page: 20, total: data.length } })

function reply(user, path, search) {
  if (path.endsWith('/auth/refresh')) return { access_token: 'smoke', expires_in: 900 }
  if (path.endsWith('/auth/me')) return user
  if (path.endsWith('/room-types')) return { data: POLICIES }
  if (path.endsWith('/users')) return page1([USER, TU, KALAB, OB, ADMIN])
  if (path.endsWith('/audit-logs')) return page1([AUDIT])
  if (path.endsWith('/imports')) return page1([IMPORT, { ...IMPORT, id: 'i2', kind: 'USERS', status: 'ROLLED_BACK', rolled_back_at: '2026-10-02T08:00:00+07:00' }])
  if (path.endsWith('/checkouts')) return page1([CHECKOUT])
  if (/\/rooms\/[^/]+\/qr$/.test(path)) {
    return { url: 'http://localhost:5198/check-in?room=LAB-RPL-02&t=0123456789abcdef0123456789abcdef', qr_token: '0123456789abcdef', rotated_at: '2026-10-20T08:00:00+07:00' }
  }
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
  if (path.endsWith('/availability')) {
    return { data: [{ room: ROOM, operating_hours: HOURS, status: 'DIGUNAKAN', conflicts: [{ kind: 'SCHEDULE', ...TIMELINE[0] }] }] }
  }
  if (path.endsWith('/schedule')) return { data: TIMELINE, operating_hours: HOURS }
  if (path.endsWith('/rooms')) return page1([ROOM])
  if (/\/rooms\/[^/]+$/.test(path)) return ROOM
  if (/\/bookings\/[^/]+\/events$/.test(path)) return { data: [] }
  if (/\/bookings\/[^/]+$/.test(path)) return BOOKINGS[0]
  if (path.endsWith('/bookings')) {
    const status = search.get('status')
    const list = status ? BOOKINGS.filter((b) => b.status === status) : BOOKINGS
    return page1(list)
  }
  return page1([])
}

async function mockApi(page, user) {
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
      body: JSON.stringify(reply(user, url.pathname, url.searchParams)),
    })
  })
}

const STUDENT_PAGES = [
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

const STAFF_PAGES = [
  '/',
  '/approvals',
  '/bookings',
  '/admin/rooms',
  '/admin/schedules',
  '/today',
  '/verify',
  '/reports',
  '/manage/students',
  '/manage/students/import',
  '/manage/imports',
  '/admin/users',
  '/admin/room-types',
  '/admin/audit',
  '/notifications',
  '/account',
]

// The home page differs per staff role (queue, room status, check-outs), so each one gets its own run.
const RUNS = [
  ...STUDENT_PAGES.map((path) => ({ user: USER, path })),
  ...STAFF_PAGES.map((path) => ({ user: ADMIN, path })),
  ...[TU, KALAB, OB].map((user) => ({ user, path: '/' })),
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
    for (const { user, path } of RUNS) {
      const context = await browser.newContext({
        viewport: { width: viewport.width, height: viewport.height },
      })
      const page = await context.newPage()
      // A page that crashes would pass the width check while showing nothing, so errors fail it too.
      const errors = []
      page.on('pageerror', (err) => errors.push(err.message))
      // Signed out only on the sign-in page; everything else restores the mocked session.
      if (path === '/sign-in') {
        await page.route('**/api/v1/auth/refresh', (r) =>
          r.fulfill({ status: 401, contentType: 'application/json', body: '{}' })
        )
      } else {
        await mockApi(page, user)
      }
      await page.goto(BASE + path, { waitUntil: 'networkidle' }).catch(() => {})
      await page.waitForTimeout(500)
      const { scrollWidth, innerWidth, crashed } = await page.evaluate(() => ({
        scrollWidth: document.documentElement.scrollWidth,
        innerWidth: window.innerWidth,
        crashed: document.body.innerText.includes('Terjadi gangguan pada aplikasi'),
      }))
      const fits = scrollWidth <= innerWidth
      const ok = fits && !crashed && errors.length === 0
      const label = `${viewport.name.padEnd(9)} ${user.role.padEnd(9)} ${path}`
      const problem = !fits
        ? `scrollWidth ${scrollWidth}, innerWidth ${innerWidth}`
        : crashed || errors.length > 0
          ? `crashed: ${errors[0] ?? 'error page shown'}`
          : `scrollWidth ${scrollWidth}`
      console.log(`${ok ? 'ok  ' : 'FAIL'} ${label} (${problem})`)
      if (!ok) failures.push(`${label}: ${problem}`)
      if (SHOTS) {
        const file = `${viewport.name}_${user.role}${path.replace(/[^a-z0-9]+/gi, '_')}.png`
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
  console.error(`\n${failures.length} page(s) scroll sideways or crash:\n- ${failures.join('\n- ')}`)
  process.exit(1)
}
console.log('\nNo horizontal scroll and no crash on any page, role or viewport.')
