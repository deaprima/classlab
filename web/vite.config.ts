/// <reference types="vitest/config" />
import path from 'path'
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import { playwright } from '@vitest/browser-playwright'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // All variables, not only VITE_*: these two configure the dev server and must not reach the browser.
  const env = loadEnv(mode, process.cwd(), '')
  // Host names the dev server accepts besides localhost, e.g. a tunnel to try the camera on a phone over
  // HTTPS (DEV_ALLOWED_HOSTS=.trycloudflare.com). Unset: Vite's default, which refuses unknown hosts.
  const allowedHosts = env.DEV_ALLOWED_HOSTS?.split(',')
    .map((h) => h.trim())
    .filter(Boolean)

  return {
    plugins: [
      tanstackRouter({
        target: 'react',
        autoCodeSplitting: true,
      }),
      react(),
      tailwindcss(),
    ],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    server: {
      ...(allowedHosts?.length ? { allowedHosts } : {}),
      // Same origin in dev as behind Caddy in prod: the SameSite=Strict refresh cookie scoped to
      // /api/v1/auth then works without CORS.
      proxy: {
        '/api': {
          target: env.API_PROXY_TARGET || 'http://localhost:8080',
        },
      },
    },
    test: {
      silent: 'passed-only',
      unstubEnvs: true,
      browser: {
        enabled: true,
        provider: playwright(),
        instances: [{ browser: 'chromium' }],
      },
      coverage: {
        // include: ['src/**/*.{js,jsx,ts,tsx}'], // Uncomment to expand the report to all src/**/* so untested modules appear as 0% coverage.
        exclude: [
          'src/components/ui/**',
          'src/assets/**',
          'src/tanstack-table.d.ts',
          'src/routeTree.gen.ts',
          'src/test-utils/**',
          'src/routes/**',
        ],
      },
    },
  }
})
