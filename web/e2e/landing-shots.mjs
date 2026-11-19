// Takes screenshots of the landing page (light, dark, mobile) and checks it never scrolls sideways.
// The API is mocked (no session), so no backend is needed. Run: node e2e/landing-shots.mjs <dir>
import { mkdirSync } from 'node:fs'
import { chromium } from 'playwright'
import { createServer } from 'vite'

const PORT = 5197
const BASE = `http://localhost:${PORT}`
const OUT = process.argv[2] ?? 'landing-shots'
mkdirSync(OUT, { recursive: true })

const server = await createServer({
  server: { port: PORT, strictPort: true },
  logLevel: 'error',
})
await server.listen()
const browser = await chromium.launch()

const RUNS = [
  { name: 'light-1440', width: 1440, height: 900, scheme: 'light' },
  { name: 'dark-1440', width: 1440, height: 900, scheme: 'dark' },
  { name: 'light-390', width: 390, height: 844, scheme: 'light' },
]

let failed = false
for (const run of RUNS) {
  const context = await browser.newContext({
    viewport: { width: run.width, height: run.height },
    colorScheme: run.scheme,
  })
  const page = await context.newPage()
  // No session: the refresh cookie is refused, so "/" shows the landing page.
  await page.route('**/api/v1/auth/refresh', (r) =>
    r.fulfill({
      status: 401,
      contentType: 'application/json',
      body: JSON.stringify({
        error: { code: 'UNAUTHENTICATED', message: 'x', details: [], request_id: 'r' },
      }),
    })
  )
  await page.goto(BASE + '/', { waitUntil: 'networkidle' })
  await page.waitForSelector('h1')
  // Scroll through the page so every reveal fires, then back to the top.
  const height = await page.evaluate(() => document.documentElement.scrollHeight)
  for (let y = 0; y < height; y += 500) {
    await page.evaluate(
      (top) => window.scrollTo({ top, behavior: 'instant' }),
      y
    )
    await page.waitForTimeout(120)
  }
  await page.waitForTimeout(1800)
  await page.evaluate(() =>
    window.scrollTo({ top: 0, behavior: 'instant' })
  )
  await page.waitForTimeout(900)
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - window.innerWidth
  )
  if (overflow > 0) {
    failed = true
    console.log(`${run.name}: scrolls sideways by ${overflow}px`)
    const wide = await page.evaluate(() =>
      [...document.querySelectorAll('body *')]
        .filter((e) => e.getBoundingClientRect().right > window.innerWidth + 1)
        .slice(0, 8)
        .map((e) => `${e.tagName} ${String(e.className).slice(0, 70)} right=${Math.round(e.getBoundingClientRect().right)}`)
    )
    console.log(wide.join('\n'))
  }
  await page.screenshot({ path: `${OUT}/${run.name}-top.png` })
  await page.screenshot({ path: `${OUT}/${run.name}.png`, fullPage: true })
  console.log(`${run.name}: ok (height ${height}px)`)
  await context.close()
}

await browser.close()
await server.close()
process.exit(failed ? 1 : 0)
