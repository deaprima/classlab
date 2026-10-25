// End-to-end smoke test (docs/tasks.md 7.5): one booking through the real web app, API and database, from
// request to history. A student requests a lab, the lab head approves it, the student checks in with the
// room's QR link, checks out with a photo from Chromium's fake camera, and finds it in the history.
//
// Check-in only opens 15 minutes before the start, so the API runs with DEV_CLOCK_START set to tomorrow
// 08:50 WIB and the browser clock is moved to the same instant; the test then runs at any hour of the day.
// It uses its own database (classlab_e2e) in the dev Postgres container and never touches the dev data.
//
// Needs: `docker compose up -d` (the classlab-postgres container), Go, and the Playwright Chromium.
// Run with `pnpm smoke:e2e`. Screenshots of a failing step go to SMOKE_SHOTS (default: the OS temp dir).
import { spawn, spawnSync } from 'node:child_process'
import { mkdirSync, mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { chromium } from 'playwright'
import { createServer } from 'vite'

const ROOT = resolve(import.meta.dirname, '../..')
const API_PORT = 8091
const WEB_PORT = 5199
const API = `http://localhost:${API_PORT}`
const BASE = `http://localhost:${WEB_PORT}`
const SHOTS = process.env.SMOKE_SHOTS || join(tmpdir(), 'classlab-e2e')

// The dev compose credentials (docker-compose.yml); the test database is created next to the dev one.
const PG_CONTAINER = 'classlab-postgres'
const DB_NAME = 'classlab_e2e'
const DATABASE_URL = `postgres://classlab:classlab_secret@localhost:5432/${DB_NAME}?sslmode=disable`

// Seed accounts (api/internal/seed), all with the published dev password.
const PASSWORD = 'classlab-dev-2026'
const ADMIN = '197501012000011001'
const KALAB = '197805202005011003'
const STUDENT = '2021101001'

// Tomorrow in WIB, so the slot is always in the future and inside lab hours (07:00 to 21:00).
const DAY = new Date(Date.now() + 7 * 3600_000 + 24 * 3600_000).toISOString().slice(0, 10)
const START = `${DAY}T08:50:00+07:00`
const PURPOSE = `Praktikum E2E ${Date.now().toString(36)}`

function run(cmd, args, opts = {}) {
  const res = spawnSync(cmd, args, { encoding: 'utf8', ...opts })
  if (res.status !== 0) {
    throw new Error(`${cmd} ${args.join(' ')} failed:\n${res.stderr || res.stdout || res.error}`)
  }
  return res.stdout
}

function psql(sql) {
  return run('docker', ['exec', PG_CONTAINER, 'psql', '-U', 'classlab', '-d', DB_NAME, '-tAc', sql]).trim()
}

function prepareDatabase() {
  run('docker', ['exec', PG_CONTAINER, 'psql', '-U', 'classlab', '-c', `DROP DATABASE IF EXISTS ${DB_NAME} WITH (FORCE)`])
  run('docker', ['exec', PG_CONTAINER, 'psql', '-U', 'classlab', '-c', `CREATE DATABASE ${DB_NAME}`])
  run('docker', [
    'run', '--rm', '--network', `container:${PG_CONTAINER}`,
    '-v', `${join(ROOT, 'api/db/migrations')}:/migrations`,
    'migrate/migrate:v4.18.3', '-path=/migrations', '-database', DATABASE_URL, 'up',
  ])
  run('go', ['run', './cmd/seed'], { cwd: join(ROOT, 'api'), env: { ...process.env, DATABASE_URL } })
}

async function startApi() {
  const bin = join(mkdtempSync(join(tmpdir(), 'classlab-e2e-')), process.platform === 'win32' ? 'api.exe' : 'api')
  run('go', ['build', '-o', bin, './cmd/server'], { cwd: join(ROOT, 'api') })
  const api = spawn(bin, [], {
    cwd: join(ROOT, 'api'),
    stdio: ['ignore', 'ignore', 'inherit'],
    env: {
      ...process.env,
      APP_ENV: 'development',
      PORT: String(API_PORT),
      DATABASE_URL,
      JWT_SECRET: 'e2e-only-secret-that-is-at-least-32-chars',
      // The QR link is built from the first origin, so it points at this test's web server.
      CORS_ALLOWED_ORIGINS: BASE,
      UPLOAD_DIR: mkdtempSync(join(tmpdir(), 'classlab-e2e-uploads-')),
      DEV_CLOCK_START: START,
    },
  })
  for (let i = 0; i < 60; i++) {
    if (api.exitCode !== null) throw new Error(`API exited with code ${api.exitCode}`)
    const ok = await fetch(`${API}/health`).then((r) => r.ok, () => false)
    if (ok) return api
    await new Promise((r) => setTimeout(r, 250))
  }
  api.kill()
  throw new Error('API did not become healthy')
}

async function apiCall(method, path, token, body) {
  const res = await fetch(`${API}/api/v1${path}`, {
    method,
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  })
  const json = await res.json().catch(() => null)
  if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${JSON.stringify(json)}`)
  return json
}

let failed = false

async function step(name, page, fn) {
  process.stdout.write(`- ${name} ... `)
  try {
    await fn()
    console.log('ok')
  } catch (err) {
    console.log('FAIL')
    if (page) {
      mkdirSync(SHOTS, { recursive: true })
      const file = join(SHOTS, `${name.replace(/[^a-z0-9]+/gi, '_')}.png`)
      await page.screenshot({ path: file, fullPage: true }).catch(() => {})
      console.error(`  screenshot: ${file}`)
    }
    failed = true
    throw err
  }
}

function expectEqual(actual, expected, what) {
  if (actual !== expected) throw new Error(`${what} = ${JSON.stringify(actual)}; want ${JSON.stringify(expected)}`)
}

console.log(`Server and browser clock start at ${START}; slot ${DAY} 09:00 – 10:00.\n`)
prepareDatabase()
const api = await startApi()
process.env.API_PROXY_TARGET = API
const server = await createServer({ server: { port: WEB_PORT, strictPort: true }, logLevel: 'error' })
await server.listen()
const browser = await chromium.launch({
  args: ['--use-fake-ui-for-media-stream', '--use-fake-device-for-media-stream'],
})

async function signIn(identifier) {
  const context = await browser.newContext({ viewport: { width: 1280, height: 800 } })
  await context.grantPermissions(['camera'], { origin: BASE })
  const page = await context.newPage()
  await page.clock.install({ time: new Date(START) })
  await page.goto(`${BASE}/sign-in`)
  await page.getByLabel('NIM / NIP').fill(identifier)
  await page.getByLabel('Password', { exact: true }).fill(PASSWORD)
  await page.getByRole('button', { name: 'Masuk' }).click()
  await page.waitForURL((url) => !url.pathname.startsWith('/sign-in'))
  return page
}

try {
  let room, adminToken, booking
  await step('admin prepares a lab room', null, async () => {
    const login = await apiCall('POST', '/auth/login', null, { identifier: ADMIN, password: PASSWORD })
    adminToken = login.access_token
    room = await apiCall('POST', '/rooms', adminToken, {
      org_unit_id: login.user.org_unit_id, code: 'E2E-LAB', name: 'Lab Uji E2E', type: 'LAB', capacity: 30, facilities: [],
    })
  })

  const student = await signIn(STUDENT)
  await step('student requests the lab from the schedule', student, async () => {
    await student.goto(`${BASE}/schedule?group=LAB&room=${room.id}&date=${DAY}`)
    await student.getByRole('button', { name: `Pilih ${DAY} 09:00` }).click()
    await student.getByRole('button', { name: `Pilih ${DAY} 09:30` }).click()
    await student.getByRole('button', { name: 'Ajukan Ruang' }).click()
    const dialog = student.getByRole('dialog')
    await dialog.getByLabel('Nama kegiatan').fill(PURPOSE)
    const created = student.waitForResponse((r) => r.url().endsWith('/api/v1/bookings') && r.request().method() === 'POST')
    await dialog.getByRole('button', { name: 'Kirim Pengajuan' }).click()
    const res = await created
    expectEqual(res.status(), 201, 'create status')
    booking = await res.json()
    expectEqual(booking.status, 'MENUNGGU', 'new lab booking status')
    await dialog.waitFor({ state: 'detached' })
  })

  const kalab = await signIn(KALAB)
  await step('lab head approves the request', kalab, async () => {
    await kalab.goto(`${BASE}/approvals`)
    await kalab.getByText(PURPOSE).first().waitFor()
    await kalab.getByRole('complementary', { name: 'Detail pengajuan' }).getByRole('button', { name: 'Setujui' }).click()
    const approved = kalab.waitForResponse((r) => r.url().endsWith(`/bookings/${booking.id}/approve`))
    await kalab.getByRole('alertdialog').getByRole('button', { name: 'Setujui' }).click()
    expectEqual((await approved).status(), 200, 'approve status')
  })

  await step('student checks in with the room QR link', student, async () => {
    const qr = await apiCall('GET', `/rooms/${room.id}/qr`, adminToken)
    if (!qr.url.startsWith(`${BASE}/check-in?`)) throw new Error(`QR url ${qr.url} does not point at ${BASE}`)
    await student.goto(qr.url)
    await student.getByText('Check-in berhasil').waitFor()
  })

  await step('student checks out with a camera photo', student, async () => {
    await student.getByRole('link', { name: 'Check-out' }).click()
    await student.getByRole('radio', { name: 'Bersih dan kondisi baik' }).click()
    await student.getByRole('button', { name: 'Buka kamera' }).click()
    // The shutter is enabled once the stream opens; a frame to capture arrives a moment later.
    await student.waitForFunction(() => document.querySelector('video')?.videoWidth > 0, null, { polling: 100 })
    await student.getByRole('button', { name: 'Jepret' }).click()
    await student.getByRole('button', { name: 'Gunakan foto' }).click()
    const done = student.waitForResponse((r) => r.url().endsWith(`/bookings/${booking.id}/check-out`))
    await student.getByRole('button', { name: 'Check-out' }).click()
    expectEqual((await done).status(), 200, 'check-out status')
    await student.getByText('Check-out berhasil').waitFor()
  })

  await step('booking shows in the history', student, async () => {
    await student.goto(`${BASE}/history`)
    await student.getByText(PURPOSE).first().waitFor()
  })

  await step('database holds the finished booking with its photo', null, async () => {
    const row = psql(`SELECT status || ',' || (checked_in_at IS NOT NULL) || ',' || (checkout_attachment_id IS NOT NULL)
      FROM bookings WHERE id = '${booking.id}'`)
    expectEqual(row, 'SELESAI,true,true', 'status, checked in, photo')
    const events = psql(`SELECT string_agg(to_status::text, '>' ORDER BY created_at) FROM booking_events WHERE booking_id = '${booking.id}'`)
    expectEqual(events, 'MENUNGGU>DISETUJUI>BERLANGSUNG>SELESAI', 'status history')
  })
} catch (err) {
  failed = true
  console.error(`\n${err.stack || err}`)
} finally {
  await browser.close()
  await server.close()
  api.kill()
}

if (failed) process.exit(1)
console.log('\nThe whole flow works end to end.')
